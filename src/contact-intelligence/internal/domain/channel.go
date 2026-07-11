package domain

// Channel est un canal de messagerie.
type Channel string

const (
	ChannelSMS      Channel = "sms"
	ChannelWhatsApp Channel = "whatsapp"
	ChannelTelegram Channel = "telegram"
)

// knownChannels est l'ensemble des canaux valides du domaine.
var knownChannels = map[Channel]struct{}{
	ChannelSMS:      {},
	ChannelWhatsApp: {},
	ChannelTelegram: {},
}

// IsValid retourne true si le canal est reconnu.
func (c Channel) IsValid() bool {
	_, ok := knownChannels[c]
	return ok
}
