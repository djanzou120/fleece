package intelligenceprocessor

// delivery_outcome.go — logique COMMUNE a on_message_delivered.go et
// on_message_failed.go : les deux handlers publics ne different que par le
// booleen `delivered` et le statut cible ecrit dans
// campaign.campaign_recipients ; toute la mecanique (transaction unique,
// lecture autonome, scoring contact, agregat analytics, correlation
// campagne) est factorisee ici pour eviter ~90 lignes de duplication. Regle
// transverse n°1 ("1 fichier par handler d'evenement") respectee au sens ou
// handleMessageDelivered/handleMessageFailed — les handlers effectivement
// enregistres dans handlersByRoutingKey (consumer.go) — vivent chacun dans
// leur propre fichier dedie ; ce fichier est une brique interne partagee,
// jamais enregistree directement comme handler.
//
// ═══════════════════════════════════════════════════════════════════════════
// IDEMPOTENCE — LE POINT DUR DE CE WORKER (a lire avant de modifier ce fichier)
// ═══════════════════════════════════════════════════════════════════════════
//
// Le plan de migration (M-021) exige que ce worker soit protege contre le
// double-traitement d'un message.delivered/message.failed rejoue par AMQP
// (redelivery apres un ack perdu, ou consumer.go/processDelivery redemarre
// avant d'avoir acquitte). Contrairement a src/core-processor (dont l'UPDATE
// de statut "WHERE status NOT IN (...)" est NATURELLEMENT idempotent car
// c'est une TRANSITION d'etat), ce worker n'effectue QUE des incrementations
// ADDITIVES ("+1" partout) : sample_size, success_count/failure_count,
// sent/delivered/failed/cost/delivery_latency_ms_sum. Une transition d'etat
// se rejoue sans effet (2e UPDATE = 0 ligne) ; un "+1" rejoue produit "+2".
//
// PISTE ENVISAGEE PUIS ECARTEE : utiliser messaging.messages.status comme
// garde ("UPDATE ... WHERE status NOT IN (...) RETURNING ... ", meme motif
// que src/core-processor). ECARTEE car un VRAI CONFLIT DE CONCEPTION : les
// deux workers (core-processor ET intelligence-processor) recoivent CHACUN
// leur propre copie du MEME evenement (deux queues distinctes bindees au
// meme exchange, voir consumer.go) et cibleraient LA MEME ligne
// messaging.messages. Si ce worker faisait AUSSI "UPDATE ... WHERE status
// NOT IN ('delivered','failed','rejected')", le PREMIER des deux workers a
// executer son UPDATE (ordre non garanti entre deux consumers independants)
// gagnerait la transition, et le SECOND verrait TOUJOURS 0 ligne affectee
// (statut deja terminal) — meme au tout premier traitement, jamais seulement
// en cas de doublon. Ce worker ne toucherait donc JAMAIS ses propres
// compteurs si core-processor le devance (frequent, l'UPDATE de
// core-processor est plus simple/rapide). Utiliser messaging.messages.status
// ici serait donc une garde qui "a l'air" idempotente mais qui, en realite,
// CASSE le fonctionnement nominal (PM : "ne bricole pas une garde qui a l'air
// idempotente sans l'etre").
//
// RECHERCHE D'UNE TABLE DE DEDUPLICATION EXISTANTE (candidate proposee par le
// PM : contact_intel.delivery_history / la table d'historique de la
// migration 0014) — BLOCAGE DE SCHEMA CONSTATE : migrations/0008_contact_intel.sql
// definit contact_intel.contact_channel_history(id bigserial PK, phone,
// channel, success, created_at) ; migrations/0014_contact_intel_scoring.sql
// (inspectee integralement) N'AJOUTE AUCUNE colonne a cette table et ne cree
// AUCUNE autre table d'historique. Il n'existe donc NULLE PART dans le schema
// une colonne message_id assortie d'une contrainte UNIQUE (ou d'une cle
// primaire) permettant un "INSERT ... ON CONFLICT (message_id, event_type) DO
// NOTHING" fiable. Conformement a l'instruction PM ("si non, ne cree pas de
// migration : implemente la variante la plus sure possible, documente
// precisement la limite d'idempotence restante et remonte-la au PM"), AUCUNE
// migration n'a ete ajoutee.
//
// VARIANTE RETENUE ("la plus sure possible" sans migration) :
//  1. La lecture de messaging.messages est un SELECT PUR (loadMessage, dans
//     LA MEME transaction que les ecritures ci-dessous, mais sans clause
//     WHERE status — donc SANS AUCUNE INTERFERENCE avec core-processor, qui
//     reste le seul et unique proprietaire d'ecriture de cette colonne,
//     Decision C).
//  2. Les compteurs contact_intel/analytics restent des "+1" purs, SANS garde
//     de deduplication au niveau de cette agregation precise.
//  3. La correlation campagne (campaign_correlation.go), elle, EST
//     idempotente (garde reelle sur campaign_recipients.status, table
//     possedee exclusivement par ce worker + on_campaign_run.go/
//     campaigns_import.go — aucun conflit avec core-processor).
//
// LIMITE RESIDUELLE ASSUMEE (a tracer par le PM comme nouvelle dette, ex.
// D-M25) : en cas de redelivery AMQP d'un message.delivered/message.failed
// DEJA traite avec succes par ce worker (commit effectue, ack perdu en
// reseau, ou crash du worker entre le commit et l'envoi de l'ack), le
// contact_channel_scores.sample_size, contacts.success_count/failure_count et
// analytics.message_daily.sent/delivered/failed/cost/delivery_latency_ms_sum
// seront DOUBLE-COMPTES pour ce message precis. Facteurs attenuants qui
// rendent cette limite acceptable en l'etat (et distinguent ce risque du
// remboursement wallet de core-processor, qui LUI est rigoureusement
// idempotent) :
//   - Impact BORNE et NON MONETAIRE : au pire un score de joignabilite ou un
//     KPI produit legerement biaise pour un seul message — jamais une perte
//     financiere, jamais un solde negatif, jamais une duplication de
//     paiement (ce worker n'ecrit JAMAIS wallet.*, Decision C).
//   - Fenetre de declenchement etroite : une redelivery APRES commit
//     n'arrive que si l'ack lui-meme est perdu (panne reseau entre ce
//     processus et le broker, ou crash exactement entre le Commit() SQL et
//     l'appel delivery.Ack()) — pas a chaque redemarrage normal du worker
//     (qui, lui, ne redeliverait que des messages JAMAIS acquittes, donc
//     jamais commit).
//   - Recommandation pour fermer definitivement ce point (a la charge d'une
//     tache future, migration additive) : une table dediee
//     contact_intel.processed_events(message_id uuid, event_type text,
//     PRIMARY KEY (message_id, event_type)) remplie par
//     "INSERT ... ON CONFLICT DO NOTHING" au DEBUT de la transaction ;
//     ne compter les agregats que si l'insert a REELLEMENT insere une ligne.
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// processDeliveryOutcome applique, en UNE SEULE transaction (exigence
// explicite du plan M-021), les effets communs a "message.delivered"
// (delivered=true) et "message.failed" (delivered=false) : scoring contact,
// agregat analytics, correlation campagne.
//
// Retourne une permanentError (voir consumer.go) si message_id est inconnu de
// messaging.messages (evenement force/rejoue sur un message jamais existe —
// rejet definitif, log WARN, Ack sans requeue, cf. §Autonomie du plan).
// Retourne une erreur "nue" pour toute panne technique transitoire (requeue).
func processDeliveryOutcome(ctx context.Context, s *Service, evt messageEvent, delivered bool) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err // transitoire -> requeue
	}
	// No-op si Commit() a deja ete appele (meme convention que
	// src/core-processor/on_message_failed.go / src/api/wallet_deduct.go).
	defer func() { _ = tx.Rollback() }()

	// 1. Lecture AUTONOME du message (voir §Autonomie, consumer.go). SELECT
	// PUR (pas de clause WHERE status) : ce worker ne modifie JAMAIS
	// messaging.messages, propriete exclusive de core-processor (Decision C).
	row, err := loadMessage(ctx, tx, evt.MessageID)
	if errors.Is(err, sql.ErrNoRows) {
		s.Logger.Warn("intelligence-processor: message introuvable, rejet permanent",
			"message_id", evt.MessageID, "delivered", delivered)
		if cErr := tx.Commit(); cErr != nil {
			return cErr
		}
		return newPermanentError(fmt.Errorf("message %s introuvable dans messaging.messages", evt.MessageID))
	}
	if err != nil {
		return err // transitoire -> requeue
	}

	now := time.Now().UTC()

	// 2. Contact Intelligence (contact_score.go). Voir §Idempotence ci-dessus :
	// pas de garde de deduplication ici (limite residuelle assumee et
	// documentee).
	channel := row.channelOrUnknown()
	if err := upsertChannelScore(ctx, tx, row.Recipient, channel, delivered, now); err != nil {
		return err
	}
	if err := upsertContactCounters(ctx, tx, row.Recipient, channel, delivered, now); err != nil {
		return err
	}

	// 3. Analytics (analytics_aggregate.go). Repartition stricte des
	// compteurs (jamais sent/cost ici — voir on_message_sent.go) :
	//   delivered=true  -> delivered=1, delivery_latency_ms_sum=latence
	//   delivered=false -> failed=1
	// day = jour d'ENVOI (row.CreatedAt), jamais le jour de traitement du DLR.
	country := CountryFromRecipient(row.Recipient)
	var deliveredDelta, failedDelta, latencyDelta int64
	if delivered {
		deliveredDelta = 1
		latencyDelta = deliveryLatencyMs(row.CreatedAt, now)
	} else {
		failedDelta = 1
	}
	if err := upsertMessageDaily(ctx, tx, row.CreatedAt, row.WorkspaceID, country, channel,
		0, deliveredDelta, failedDelta, 0, latencyDelta,
	); err != nil {
		return err
	}

	// 4. Correlation campagne (campaign_correlation.go) — idempotence REELLE
	// ici (voir doc de tete de fichier campaign_correlation.go), contrairement
	// aux etapes 2/3 ci-dessus.
	targetStatus := "failed"
	if delivered {
		targetStatus = "delivered"
	}
	touched, campaignID, err := correlateCampaignRecipient(ctx, tx, evt.MessageID, targetStatus)
	if err != nil {
		return err
	}
	if touched {
		s.Logger.Info("intelligence-processor: destinataire de campagne correle",
			"message_id", evt.MessageID, "campaign_id", campaignID, "status", targetStatus)
	}

	return tx.Commit()
}
