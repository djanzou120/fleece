package intelligenceprocessor

// campaign_correlation.go — correlation d'un message.delivered/message.failed
// avec un destinataire de campagne (campaign.campaign_recipients), DANS la
// transaction unique de delivery_outcome.go.
//
// Colonnes exactes (verifiees dans migrations/0007_campaign.sql +
// migrations/0016_campaign_execution.sql avant d'ecrire ce fichier) :
//
//	campaign.campaign_recipients(id bigserial PK, campaign_id uuid NOT NULL,
//	  recipient text NOT NULL, status text DEFAULT 'pending', message_id uuid NULL)
//	campaign.campaign_runs(id bigserial PK, campaign_id uuid NOT NULL,
//	  started_at, finished_at, total_count, sent_count, delivered_count, failed_count)
//
// IDEMPOTENCE (naturelle, PAS besoin de garde ad-hoc) : campaign_recipients.status
// n'est ecrit QUE par ce worker (ici, et via markRecipientSent dans
// on_campaign_run.go) et par src/api/campaigns_import.go (valeur initiale
// 'pending' seulement) — AUCUN AUTRE ECRIVAIN, en particulier PAS
// core-processor. La garde SQL "AND status NOT IN ('delivered','failed')"
// ci-dessous est donc un VRAI mecanisme d'idempotence (contrairement a
// messaging.messages.status, dispute avec core-processor — voir
// delivery_outcome.go pour la discussion complete du point dur d'idempotence
// de ce worker).
//
// BLOCAGE DE SCHEMA RENCONTRE (a documenter au PM, voir rapport de tache
// M-021) : campaign.campaign_recipients N'A PAS de colonne run_id — le seul
// lien disponible entre un destinataire et "sa" campagne est campaign_id
// (migrations 0007/0016 inspectees : campaign_recipients porte campaign_id et
// message_id, mais aucune colonne reliant une ligne a un campaign_runs.id
// precis). Une campagne peut avoir PLUSIEURS runs (campaign_runs n'a pas de
// contrainte d'unicite sur campaign_id). Il est donc IMPOSSIBLE de determiner
// de maniere fiable a quel campaign_runs specifique imputer
// delivered_count/failed_count pour un destinataire donne, sans deviner
// (heuristique "dernier run de cette campagne" non garantie correcte si
// plusieurs runs coexistent). Conformement a l'instruction PM ("si le lien
// n'existe pas, ne cree pas de migration — implemente le reste et remonte le
// blocage"), cette fonction met a jour campaign_recipients.status (le
// "reste"), MAIS N'INCREMENTE PAS campaign_runs.delivered_count/failed_count.
// Dette a tracer par le PM (nouvelle reference, ex. D-M24) : ajouter une
// colonne run_id (ou uuid similaire) a campaign_recipients serait la
// migration additive naturelle pour fermer ce blocage.
import (
	"context"
	"database/sql"
	"errors"

	gosql "fleece/src/go/sql"
)

// correlateCampaignRecipient transitionne (si applicable) UN destinataire de
// campagne vers targetStatus ("delivered" ou "failed"), en le correlant par
// message_id. Retourne (touched=true, campaignID, nil) si une ligne a
// effectivement ete transitionnee — auquel cas l'appelant sait que le message
// provenait bien d'une campagne (utile pour l'observabilite/les logs), sans
// que campaign_runs.*_count soit incremente (voir blocage de schema ci-dessus).
//
// touched=false, campaignID="" (sans erreur) signifie soit : le message
// n'est pas issu d'une campagne (cas normal, PAS une erreur), soit le
// destinataire a deja atteint un statut terminal (idempotence — voir doc de
// tete de fichier).
func correlateCampaignRecipient(ctx context.Context, tx *gosql.Tx, messageID, targetStatus string) (touched bool, campaignID string, err error) {
	var row struct {
		CampaignID string `db:"campaign_id"`
	}
	getErr := tx.Get(ctx, &row,
		`UPDATE campaign.campaign_recipients SET status = $1
		   WHERE message_id = $2 AND status NOT IN ('delivered', 'failed')
		 RETURNING campaign_id`,
		targetStatus, messageID,
	)
	if errors.Is(getErr, sql.ErrNoRows) {
		return false, "", nil
	}
	if getErr != nil {
		return false, "", getErr
	}
	return true, row.CampaignID, nil
}
