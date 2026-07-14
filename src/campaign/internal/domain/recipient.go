package domain

// RecipientStatus represente l'etat d'un destinataire au sein d'une campagne.
type RecipientStatus string

const (
	// RecipientPending : le message n'a pas encore ete envoye.
	RecipientPending RecipientStatus = "pending"
	// RecipientSent : le message a ete soumis au provider.
	RecipientSent RecipientStatus = "sent"
	// RecipientDelivered : le message a ete livre au destinataire (DLR confirme).
	RecipientDelivered RecipientStatus = "delivered"
	// RecipientFailed : l'envoi ou la livraison a echoue.
	RecipientFailed RecipientStatus = "failed"
)

// CampaignRecipient represente un destinataire d'une campagne.
// La deduplication est enforced en base par l'index UNIQUE (campaign_id, recipient).
// Le domaine expose la notion de recipient unique sans porter la logique de dedup SQL.
type CampaignRecipient struct {
	// ID est la cle primaire bigserial.
	ID int64
	// CampaignID est la cle etrangere vers la campagne.
	CampaignID string
	// Recipient est le numero de telephone (format E.164) ou l'identifiant canal.
	Recipient string
	// Status est l'etat de l'envoi individuel.
	Status RecipientStatus
	// MessageID est l'identifiant du message dans le service messaging.
	// NULL tant que le message n'a pas ete soumis.
	// Utilise pour la correlation DLR (message.delivered/message.failed).
	MessageID *string
}

// NewCampaignRecipient cree un destinataire en etat pending.
func NewCampaignRecipient(campaignID, recipient string) *CampaignRecipient {
	return &CampaignRecipient{
		CampaignID: campaignID,
		Recipient:  recipient,
		Status:     RecipientPending,
	}
}

// MarkSent marque le destinataire comme envoye et enregistre le message_id.
// messageID est l'identifiant retourne par le service messaging.
func (r *CampaignRecipient) MarkSent(messageID string) {
	r.MessageID = &messageID
	r.Status = RecipientSent
}

// MarkDelivered marque le destinataire comme livre (DLR positif).
func (r *CampaignRecipient) MarkDelivered() {
	r.Status = RecipientDelivered
}

// MarkFailed marque le destinataire comme en echec (DLR negatif ou erreur d'envoi).
func (r *CampaignRecipient) MarkFailed() {
	r.Status = RecipientFailed
}
