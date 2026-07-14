// Package output contient les ports pilotes (driven) du service Campaign.
// Ce sont des interfaces que les adapters de couche 3 implementent.
// Les use cases dependent uniquement de ces interfaces, jamais des implementations concretes.
package output

import (
	"context"

	"fleece/src/campaign/internal/domain"
)

// CampaignRepository persiste et recupere les campagnes.
// Implemente en couche 3 (Postgres, schema "campaign").
type CampaignRepository interface {
	// Save cree ou met a jour une campagne.
	Save(ctx context.Context, c *domain.Campaign) error

	// Get recupere une campagne par son ID.
	// Retourne domain.ErrCampaignNotFound si absente.
	Get(ctx context.Context, id string) (*domain.Campaign, error)

	// ListScheduledDue retourne les campagnes dont le status='scheduled'
	// et scheduled_at <= now(). Utilisee par le scheduler.
	ListScheduledDue(ctx context.Context) ([]*domain.Campaign, error)

	// UpdateStatusAtomic met a jour le statut d'une campagne de facon atomique.
	// Retourne le nombre de lignes affectees (0 si la campagne avait deja change de statut —
	// anti-double-run). Le use case ne doit continuer que si rowsAffected == 1.
	// Utilise : UPDATE ... WHERE id=$1 AND status=$2 (from) SET status=$3 (to).
	UpdateStatusAtomic(ctx context.Context, id string, from, to domain.CampaignStatus) (rowsAffected int64, err error)
}

// RecipientRepository persiste les destinataires d'une campagne.
// Implemente en couche 3 (Postgres, schema "campaign").
type RecipientRepository interface {
	// BulkInsertDedup insere les destinataires en deduplication.
	// Utilise INSERT ... ON CONFLICT (campaign_id, recipient) DO NOTHING.
	// Retourne le nombre de lignes reellement inserees (les doublons sont silencieusement ignores).
	BulkInsertDedup(ctx context.Context, campaignID string, recipients []string) (inserted int64, err error)

	// ListPendingByCampaign retourne les destinataires en etat 'pending' pour une campagne.
	// Ne retourne jamais les destinataires deja sent/delivered/failed (reprise idempotente).
	ListPendingByCampaign(ctx context.Context, campaignID string) ([]*domain.CampaignRecipient, error)

	// UpdateStatusByMessageID met a jour le statut d'un destinataire identifie par message_id.
	// Utilise pour la correlation DLR (message.delivered/message.failed).
	// Si messageID est NULL ou ne correspond a aucun destinataire, retourne (0, nil) sans erreur.
	UpdateStatusByMessageID(ctx context.Context, messageID string, status domain.RecipientStatus) (rowsAffected int64, err error)

	// MarkSent marque un destinataire comme envoye et enregistre son message_id.
	MarkSent(ctx context.Context, id int64, messageID string) error

	// MarkFailed marque un destinataire comme echoue (sans message_id).
	MarkFailed(ctx context.Context, id int64) error
}

// RunRepository persiste les executions de campagnes.
// Implemente en couche 3 (Postgres, schema "campaign").
type RunRepository interface {
	// Create cree un nouveau run et retourne son ID (bigserial).
	Create(ctx context.Context, run *domain.CampaignRun) (int64, error)

	// IncrementSent incremente sent_count d'une unite pour le run donne.
	IncrementSent(ctx context.Context, runID int64) error

	// IncrementDelivered incremente delivered_count d'une unite pour le run donne.
	IncrementDelivered(ctx context.Context, runID int64) error

	// IncrementFailed incremente failed_count d'une unite pour le run donne.
	IncrementFailed(ctx context.Context, runID int64) error

	// Finish enregistre finished_at = now() pour le run donne.
	Finish(ctx context.Context, runID int64) error
}

// RoutingClient obtient la decision de routage (provider + cout unitaire) pour un message.
// Implemente en couche 3 (client REST vers le service routing).
//
// IMPORTANT (isolation inter-services D11/D19) : cette interface utilise uniquement
// des types primitifs Go. L'implementation concrete parle HTTP et n'importe jamais
// fleece/src/routing/... .
type RoutingClient interface {
	// GetRoutingDecision retourne le provider selectionne et le cout unitaire
	// pour l'envoi d'un message vers le recipient donne sur le canal de la strategie.
	// recipient est passe pour beneficier de l'enrichissement contact-intelligence (T-016/D28).
	// workspaceID, channel, country : parametres de routage.
	// Retourne (providerID, channel, costAmount, currency, error).
	GetRoutingDecision(ctx context.Context, workspaceID, channel, country, recipient string) (providerID, selectedChannel string, costAmount int64, currency string, err error)
}

// MessagingClient envoie un message individuel via le service messaging.
// Implemente en couche 3 (client REST vers le service messaging).
//
// IMPORTANT (isolation inter-services D11/D19) : cette interface utilise uniquement
// des types primitifs Go. L'implementation concrete parle HTTP et n'importe jamais
// fleece/src/messaging/... .
type MessagingClient interface {
	// SendMessage envoie un message et retourne l'ID du message cree dans le service messaging.
	// recipient est le numero de telephone (format E.164).
	// body est le corps du message.
	// channel est le canal d'envoi (ex. "sms", "whatsapp").
	// workspaceID est l'identifiant du workspace.
	// Retourne (messageID, error).
	SendMessage(ctx context.Context, workspaceID, recipient, body, channel string) (messageID string, err error)
}
