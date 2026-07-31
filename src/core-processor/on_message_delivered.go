package coreprocessor

// on_message_delivered.go — handler de l'evenement AMQP "message.delivered"
// (routing key "message.delivered", voir consumer.go pour le contrat exact de
// l'evenement, D-M21).
//
// Effet : transition messaging.messages.status -> 'delivered'. Aucun
// mouvement d'argent sur ce chemin (un message delivre n'est jamais
// rembourse — seul "message.failed" declenche un remboursement, voir
// on_message_failed.go).
//
// GARDE SQL = MIROIR DE Message.Transition() (correctif B1/Q3, Phase 3) :
// src/api/message.go definit la machine a etats reelle du Message (messageTransitions) :
//
//	draft     → pending, failed
//	pending   → sent, failed, rejected
//	sent      → delivered, failed
//	delivered → (terminal)
//	failed    → (terminal)
//	rejected  → (terminal)
//
// Le SEUL etat depuis lequel "delivered" est une transition autorisee est
// "sent". La garde ci-dessous ("status = 'sent'") est le miroir exact de la
// seule arete entrante de "delivered" dans messageTransitions : SI
// Message.Transition() evolue un jour (nouvelle arete vers delivered, ou
// suppression de l'arete sent→delivered), CETTE GARDE SQL DOIT ETRE MISE A
// JOUR EN MEME TEMPS (aucune verification automatique ne les lie aujourd'hui
// — ce worker n'importe pas fleece/src/api, voir note ci-dessous).
//
// IDEMPOTENCE : la garde SQL "AND status = 'sent'" fait qu'un double-traitement
// (redelivery AMQP apres un Ack perdu en reseau, ou un replay operateur) ne
// change rien a la deuxieme execution — le statut est deja 'delivered' (ou
// tout autre etat non-'sent'), donc RETURNING ne renvoie aucune ligne
// (sql.ErrNoRows), traite comme un succes no-op (log INFO/WARN, jamais
// d'erreur).
//
// POURQUOI PAS api.Message.Transition() : la machine a etats Message vit dans
// fleece/src/api (structure plate, package api). L'utiliser depuis ce worker
// forcerait core-processor a importer tout le binaire du service HTTP unifie
// (cycle de build/deploiement indesirable, et un import qui n'apporte aucune
// valeur ici : il faudrait de toute facon relire l'etat courant depuis la DB
// avant d'appeler Transition(), ce qui revient a faire l'UPDATE conditionnel
// SQL directement). La garde SQL "status = 'sent'" exprime exactement la
// meme invariante (seule "sent" autorise la transition vers "delivered") de
// facon atomique et sans dependance croisee entre les deux binaires
// deployables.
//
// E2 (revue architecture Phase 3) — UNE SEULE REQUETE (UPDATE ... RETURNING
// workspace_id), PLUS DE SECONDE LECTURE : AVANT ce correctif, ce handler
// utilisait ExecRows (rows affectees seulement) puis, si rows == 1,
// fanOutMessageDelivered relisait workspace_id par une SECONDE requete
// (lookupMessageWorkspace, SELECT ... FROM messaging.messages) — une requete
// superflue pour CHAQUE message delivered, meme en l'absence de tout endpoint
// abonne. on_message_failed.go faisait deja bien (RETURNING workspace_id,
// cost en une seule requete) : ce handler est desormais aligne sur le meme
// motif — RETURNING workspace_id directement dans l'UPDATE de transition,
// via s.DB.Get (sql.ErrNoRows ≡ 0 ligne ≡ garde "status = 'sent'" non
// satisfaite). La garde "status = 'sent'" et la semantique d'idempotence
// SONT INCHANGEES — seul le mode de recuperation de workspace_id change.
//
// FAN-OUT WEBHOOKS SORTANTS (D-M43, voir webhook_dispatch.go) : lorsque la
// transition a REELLEMENT eu lieu (RETURNING a renvoye une ligne,
// workspace_id connu), ce handler appelle s.fanOutWebhookEvent APRES
// l'UPDATE ci-dessus (donc apres son commit implicite — une instruction SQL
// hors transaction explicite est committee des son execution) et JAMAIS sur
// le chemin sql.ErrNoRows (idempotence B1/D-M26 : un rejeu Telegram ne doit
// jamais declencher un second webhook client). workspace_id est REUTILISE
// depuis le RETURNING ci-dessus, jamais une seconde requete (E2). Cette
// fonction est void (elle ne retourne aucune erreur) : un echec de fan-out
// webhook (endpoint client indisponible, panne DB de persistance de la
// delivery) ne doit JAMAIS faire echouer ce handler ni provoquer un requeue
// du message AMQP — le statut est deja committe.
import (
	"context"
	"database/sql"
	"errors"
)

// deliveredMessageRow scanne le RETURNING de l'UPDATE messaging.messages :
// workspace_id, seul necessaire au fan-out webhook (E2 — plus de seconde
// requete, voir doc de tete de fichier).
type deliveredMessageRow struct {
	WorkspaceID string `db:"workspace_id"`
}

// handleMessageDelivered traite l'evenement "message.delivered".
func handleMessageDelivered(ctx context.Context, s *Service, evt messageEvent) error {
	var row deliveredMessageRow
	err := s.DB.Get(ctx, &row,
		`UPDATE messaging.messages SET status = 'delivered'
		   WHERE id = $1 AND status = 'sent'
		 RETURNING workspace_id`,
		evt.MessageID,
	)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// Idempotence : soit le statut n'est plus 'sent' (deja 'delivered' —
		// double-traitement — ou un autre etat non-'sent', ex. 'failed'/'rejected'/
		// 'draft'/'pending'), soit id inexistant. Dans tous les cas ce n'est PAS
		// une erreur — le message AMQP est acquitte normalement (Ack), seul le
		// log distingue le cas pour observabilite. AUCUN fan-out webhook (B1/D-M26).
		//
		// CORRECTIF D-M37 : le statut REEL est relu et journalise. Sans lui, ce
		// log confondait deux situations radicalement differentes sous un meme
		// niveau INFO — une idempotence benigne (deja 'delivered', rejeu
		// Telegram) et une PERTE REELLE de DLR. Le second cas devient inevitable
		// des le remplacement des stubs providers (D-M08) : mapProviderStatus
		// retourne 'pending' pour tout statut provider inconnu, et le vrai Twilio
		// repond 'queued'/'accepted' — un message partira donc en
		// status='pending' avec un external_id, et son DLR 'delivered' sera
		// rejete par la garde "status = 'sent'" sans que rien ne le signale.
		s.logRejectedDelivery(ctx, evt)
		return nil
	case err != nil:
		// Erreur technique (ex. DB indisponible) : transitoire -> requeue
		// (voir consumer.go, processDelivery).
		return err
	}

	s.Logger.Info("core-processor: message marque delivered",
		"message_id", evt.MessageID, "external_id", evt.ExternalID, "provider_id", evt.ProviderID)

	// Fan-out webhooks sortants (D-M43) : APRES commit, UNIQUEMENT car la
	// transition a reellement eu lieu ci-dessus (garde anti-doublon-client,
	// voir doc de tete de fichier). workspace_id reutilise depuis le
	// RETURNING ci-dessus (E2, jamais une seconde requete).
	s.fanOutWebhookEvent(ctx, "message.delivered", evt, row.WorkspaceID)

	return nil
}
