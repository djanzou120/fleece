package coreprocessor

// rejected_dlr.go — journalisation d'un DLR rejeté par la garde de transition
// (D-M37).
//
// LE PROBLÈME : la garde `WHERE status = 'sent'` protège l'idempotence, mais
// son échec (0 ligne) recouvre DEUX situations qui n'ont rien à voir :
//
//   - **Bénigne** — le message est déjà dans un état TERMINAL ('delivered',
//     'failed', 'rejected'). C'est un rejeu (Telegram republie ses updates
//     jusqu'à 24 h, et `mapTelegramStatusToRoutingKey` mappe `delivered` ET
//     `read` sur la même routing key) : le DLR est légitimement ignoré.
//   - **GRAVE** — le message est dans un état NON TERMINAL ('pending',
//     'draft', 'sending'). Le DLR est alors DÉFINITIVEMENT PERDU : le message
//     restera dans cet état, aucun autre chemin ne le fera progresser, et le
//     client ne saura jamais que son message a été délivré.
//
// Avant ce correctif, les deux produisaient le MÊME log INFO, sans le statut
// réel. Il était donc impossible, en production, de distinguer une idempotence
// normale d'une perte — et le second cas devient inévitable dès le
// remplacement des stubs providers (D-M08) : `mapProviderStatus` retourne
// `'pending'` pour tout statut provider inconnu, or le vrai Twilio répond
// `queued`/`accepted`.
//
// COÛT ASSUMÉ : une requête SQL supplémentaire, mais UNIQUEMENT sur le chemin
// de rejet — jamais sur le chemin nominal. Un rejet est déjà l'exception ; y
// ajouter une lecture pour savoir ce qui s'est réellement passé est un échange
// évidemment favorable.

import (
	"context"
	"database/sql"
	"errors"
)

// terminalMessageStatuses énumère les états depuis lesquels plus aucune
// transition n'est attendue. Un DLR rejeté parce que le message est dans l'un
// d'eux est bénin ; dans tout autre état, c'est une perte.
var terminalMessageStatuses = map[string]bool{
	"delivered": true,
	"failed":    true,
	"rejected":  true,
}

// logRejectedDelivery relit le statut courant du message et journalise le rejet
// au niveau qui correspond à ce qui s'est réellement passé : INFO pour une
// idempotence bénigne, WARN pour une perte réelle.
//
// Ne retourne aucune erreur : c'est de l'observabilité. Si la relecture échoue
// (base indisponible), on journalise malgré tout le rejet — perdre le log en
// plus du DLR serait le pire des deux mondes.
func (s *Service) logRejectedDelivery(ctx context.Context, evt messageEvent) {
	var row struct {
		Status string `db:"status"`
	}
	err := s.DB.Get(ctx, &row,
		`SELECT status FROM messaging.messages WHERE id = $1`,
		evt.MessageID,
	)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		// Aucun message pour cet identifiant : événement orphelin (forgé,
		// rejoué après purge, ou corrélation erronée). Rien à récupérer.
		s.Logger.Warn("core-processor: message.delivered ignore (message introuvable)",
			"message_id", evt.MessageID, "external_id", evt.ExternalID, "provider_id", evt.ProviderID)

	case err != nil:
		s.Logger.Warn("core-processor: message.delivered ignore (statut courant illisible)",
			"message_id", evt.MessageID, "external_id", evt.ExternalID,
			"provider_id", evt.ProviderID, "err", err)

	case terminalMessageStatuses[row.Status]:
		s.Logger.Info("core-processor: message.delivered ignore (idempotence, statut deja terminal)",
			"message_id", evt.MessageID, "external_id", evt.ExternalID,
			"provider_id", evt.ProviderID, "current_status", row.Status)

	default:
		// LE cas que ce correctif rend visible : statut non terminal, DLR perdu.
		s.Logger.Warn("core-processor: DLR PERDU — message.delivered rejete alors que le statut n'est pas terminal",
			"dlr_lost", true,
			"message_id", evt.MessageID, "external_id", evt.ExternalID,
			"provider_id", evt.ProviderID, "current_status", row.Status)
	}
}
