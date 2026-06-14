package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// Sign calcule la signature HMAC-SHA256 du payload avec la cle secrete donnee.
// La signature est retournee en hexadecimal.
//
// Utilise exclusivement la stdlib (crypto/hmac, crypto/sha256, encoding/hex).
func Sign(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify verifie que la signature hexadecimale correspond au HMAC-SHA256 du payload
// calcule avec la cle secrete. La comparaison est effectuee en temps constant
// via hmac.Equal pour se proteger des attaques par timing.
func Verify(secret string, payload []byte, signature string) bool {
	expected := Sign(secret, payload)
	// Decoder les deux signatures en bytes pour pouvoir appeler hmac.Equal.
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
