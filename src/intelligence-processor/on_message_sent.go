package intelligenceprocessor

// on_message_sent.go — handler de l'evenement AMQP "message.sent" (routing
// key "message.sent").
//
// AJOUT AU PERIMETRE PAR DECISION PM (Decision A, M-021) : le plan initial ne
// listait que message.delivered/message.failed. Sans ce handler,
// analytics.message_daily.sent resterait a 0 pour tout message, et la
// protection division-par-zero de la vue materialisee analytics.kpi_daily
// (migration 0017 : "CASE WHEN sent > 0 ... ELSE 0") renverrait alors
// delivery_rate=0 et avg_cost_per_msg=0 pour TOUS les workspaces — l'ecran
// Analytics produit afficherait des KPIs a zero en permanence, meme avec des
// messages effectivement livres.
//
// Contrat producteur (verifie dans src/api/messages_send.go,
// publishMessageSent, avant d'ecrire ce fichier) :
//
//	{"message_id":"...", "workspace_id":"...", "recipient":"...", "provider_id":"...", "status":"..."}
//
// Autonomie (voir consumer.go) : seul message_id est utilise pour les
// ecritures — workspace_id/recipient/provider_id/status du payload sont
// ignores pour l'agregation (workspace_id/recipient/channel/cost/created_at
// sont TOUJOURS relus depuis messaging.messages par message_id).
//
// Compteurs incrementes : sent, cost UNIQUEMENT (repartition stricte imposee
// par le PM — jamais delivered/failed/delivery_latency_ms_sum ici, voir
// analytics_aggregate.go).
//
// Pas de transaction : ce handler ne fait qu'une seule ecriture (l'UPSERT
// analytics), contrairement a on_message_delivered.go/on_message_failed.go
// (contact intelligence + analytics + campagne, exigence explicite de
// transaction unique du plan M-021 — qui ne s'applique pas ici).
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// handleMessageSent traite l'evenement "message.sent".
func handleMessageSent(ctx context.Context, s *Service, body []byte) error {
	evt, err := parseMessageEvent(body)
	if err != nil {
		return newPermanentError(err)
	}
	if err := validateMessageID(evt.MessageID); err != nil {
		return newPermanentError(err)
	}

	row, err := loadMessage(ctx, s.DB, evt.MessageID)
	if errors.Is(err, sql.ErrNoRows) {
		s.Logger.Warn("intelligence-processor: message.sent ignore, message introuvable",
			"message_id", evt.MessageID)
		return newPermanentError(fmt.Errorf("message.sent: message %s introuvable", evt.MessageID))
	}
	if err != nil {
		return err // transitoire -> requeue
	}

	cost := int64(0)
	if row.Cost.Valid {
		cost = row.Cost.Int64
	}
	country := CountryFromRecipient(row.Recipient)

	if err := upsertMessageDaily(ctx, s.DB, row.CreatedAt, row.WorkspaceID, country, row.channelOrUnknown(),
		1, 0, 0, cost, 0,
	); err != nil {
		return err // transitoire -> requeue
	}

	s.Logger.Info("intelligence-processor: message.sent traite",
		"message_id", evt.MessageID, "workspace_id", row.WorkspaceID, "country", country, "cost", cost)
	return nil
}
