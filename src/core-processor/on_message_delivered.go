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
// "sent". La garde precedente ("status NOT IN ('delivered', 'failed',
// 'rejected')") etait TROP LARGE : elle laissait passer 'draft' → 'delivered'
// et 'pending' → 'delivered', deux transitions que la machine a etats
// interdit explicitement — un bug reel independant du desarmement B1 (webhook
// qui posait le statut avant ce worker). La garde ci-dessous ("status = 'sent'")
// est le miroir exact de la seule arete entrante de "delivered" dans
// messageTransitions : SI Message.Transition() evolue un jour (nouvelle arete
// vers delivered, ou suppression de l'arete sent→delivered), CETTE GARDE SQL
// DOIT ETRE MISE A JOUR EN MEME TEMPS (aucune verification automatique ne les
// lie aujourd'hui — ce worker n'importe pas fleece/src/api, voir note
// ci-dessous).
//
// IDEMPOTENCE : la garde SQL "AND status = 'sent'" fait qu'un double-traitement
// (redelivery AMQP apres un Ack perdu en reseau, ou un replay operateur) ne
// change rien a la deuxieme execution — le statut est deja 'delivered' (ou
// tout autre etat non-'sent'), donc 0 ligne affectee, traite comme un succes
// no-op (log INFO/WARN, jamais d'erreur).
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
import (
	"context"
)

// handleMessageDelivered traite l'evenement "message.delivered".
func handleMessageDelivered(ctx context.Context, s *Service, evt messageEvent) error {
	rows, err := s.DB.ExecRows(ctx,
		`UPDATE messaging.messages SET status = 'delivered'
		   WHERE id = $1 AND status = 'sent'`,
		evt.MessageID,
	)
	if err != nil {
		// Erreur technique (ex. DB indisponible) : transitoire -> requeue
		// (voir consumer.go, processDelivery).
		return err
	}

	if rows == 0 {
		// Idempotence : soit le statut n'est plus 'sent' (deja 'delivered' —
		// double-traitement — ou un autre etat non-'sent', ex. 'failed'/'rejected'/
		// 'draft'/'pending'), soit id inexistant. Dans tous les cas ce n'est PAS
		// une erreur — le message AMQP est acquitte normalement (Ack), seul le
		// log distingue le cas pour observabilite.
		s.Logger.Info("core-processor: message.delivered ignore (statut deja 'delivered' ou pas 'sent', ou introuvable)",
			"message_id", evt.MessageID, "external_id", evt.ExternalID, "provider_id", evt.ProviderID)
		return nil
	}

	s.Logger.Info("core-processor: message marque delivered",
		"message_id", evt.MessageID, "external_id", evt.ExternalID, "provider_id", evt.ProviderID)
	return nil
}
