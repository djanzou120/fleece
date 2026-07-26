package providers

// logging.go — D-M17 (Phase 3, revue d'architecture) : remplace les anciens
// log.Printf (stdlib) de sms.go/telegram.go/whatsapp.go par le logger
// structure partage fleece/src/go/log — regle transverse n°5, ouverte depuis
// 2 phases ("jamais log.Printf, toujours golog.Logger").
//
// Chaque adapter provider recoit un champ EXPORTE optionnel Logger
// *golog.Logger (nil par defaut) — meme pattern que s.AMQP == nil deja
// utilise ailleurs dans le depot (ex. publishMessageSent) : un provider
// construit sans logger (tests, ou composition root qui n'en fournit pas)
// logge simplement rien, sans jamais paniquer.
//
// AUCUN CHANGEMENT FONCTIONNEL : les stubs Send/GetDeliveryStatus produisent
// exactement les memes valeurs de retour qu'avant ce correctif ; seule la
// maniere de logger change (format structure cle/valeur au lieu d'une chaine
// Printf), et uniquement si Logger est renseigne.
import golog "fleece/src/go/log"

// logInfo logge un message au niveau INFO via logger si logger est non nil.
// Partagee par SMSTwilio/WhatsAppMeta/TelegramBot (evite de dupliquer la
// garde nil dans chaque fichier).
func logInfo(logger *golog.Logger, msg string, args ...any) {
	if logger == nil {
		return
	}
	logger.Info(msg, args...)
}
