package coreprocessor

// webhook_signature.go — signature HMAC-SHA256 des payloads de webhooks
// SORTANTS (D-M43, T-005 volet livraison).
//
// Portage a l'identique de src/webhook/internal/domain/signature.go (service
// a supprimer par M-023) : stdlib pure (crypto/hmac, crypto/sha256,
// encoding/hex), aucune nouvelle dependance. Sign est utilisee par
// webhook_dispatch.go (premiere tentative) et webhook_retry_scheduler.go
// (retentatives, avec le secret COURANT de l'endpoint relu en base). Verify
// est portee pour completude/parite avec le domaine de reference et testee
// (DoD de la tache) ; elle n'est appelee par aucun chemin de production ici
// (ce worker ne fait que SIGNER des payloads sortants, jamais verifier une
// signature entrante) — conservee au cas ou une validation d'entree deviendrait
// necessaire (ex. futur endpoint de confirmation cote client).
import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// signWebhookPayload calcule la signature HMAC-SHA256 du payload avec la cle
// secrete donnee. La signature est retournee en hexadecimal (header
// X-Fleece-Signature, voir webhook_dispatch.go).
func signWebhookPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyWebhookPayload verifie qu'une signature hexadecimale correspond au
// HMAC-SHA256 du payload calcule avec la cle secrete. Comparaison en temps
// constant via hmac.Equal (protection contre les attaques par timing).
func verifyWebhookPayload(secret string, payload []byte, signature string) bool {
	expected := signWebhookPayload(secret, payload)
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}
	gotBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return hmac.Equal(expectedBytes, gotBytes)
}
