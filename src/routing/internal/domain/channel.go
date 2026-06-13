package domain

// Channel represente le canal de communication.
// Les valeurs correspondent aux chaînes stockees en base de donnees.
type Channel string

const (
	// ChannelSMS est le canal SMS.
	ChannelSMS Channel = "sms"
	// ChannelWhatsApp est le canal WhatsApp.
	ChannelWhatsApp Channel = "whatsapp"
	// ChannelTelegram est le canal Telegram.
	ChannelTelegram Channel = "telegram"
)
