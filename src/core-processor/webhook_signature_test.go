package coreprocessor

import "testing"

// webhook_signature_test.go — teste signWebhookPayload/verifyWebhookPayload
// (D-M43), fonctions pures stdlib-only.

func TestSignWebhookPayload_deterministic(t *testing.T) {
	payload := []byte(`{"event":"message.delivered"}`)
	sig1 := signWebhookPayload("secret", payload)
	sig2 := signWebhookPayload("secret", payload)
	if sig1 != sig2 {
		t.Fatalf("signWebhookPayload() non deterministe : %q != %q", sig1, sig2)
	}
	if sig1 == "" {
		t.Fatal("signWebhookPayload() a retourne une signature vide")
	}
}

func TestSignWebhookPayload_differentSecretsDiffer(t *testing.T) {
	payload := []byte(`{"event":"message.failed"}`)
	if signWebhookPayload("secret-a", payload) == signWebhookPayload("secret-b", payload) {
		t.Fatal("deux secrets differents ont produit la meme signature")
	}
}

func TestSignWebhookPayload_differentPayloadsDiffer(t *testing.T) {
	sig1 := signWebhookPayload("secret", []byte(`{"a":1}`))
	sig2 := signWebhookPayload("secret", []byte(`{"a":2}`))
	if sig1 == sig2 {
		t.Fatal("deux payloads differents ont produit la meme signature")
	}
}

func TestVerifyWebhookPayload_nominal(t *testing.T) {
	payload := []byte(`{"event":"message.delivered","message_id":"abc"}`)
	sig := signWebhookPayload("le-secret", payload)
	if !verifyWebhookPayload("le-secret", payload, sig) {
		t.Fatal("verifyWebhookPayload() = false, voulu true (signature nominale valide)")
	}
}

func TestVerifyWebhookPayload_wrongSecret(t *testing.T) {
	payload := []byte(`{"event":"message.delivered"}`)
	sig := signWebhookPayload("bon-secret", payload)
	if verifyWebhookPayload("mauvais-secret", payload, sig) {
		t.Fatal("verifyWebhookPayload() = true avec un mauvais secret, voulu false")
	}
}

func TestVerifyWebhookPayload_tamperedPayload(t *testing.T) {
	original := []byte(`{"event":"message.delivered","status":"delivered"}`)
	tampered := []byte(`{"event":"message.delivered","status":"failed"}`)
	sig := signWebhookPayload("secret", original)
	if verifyWebhookPayload("secret", tampered, sig) {
		t.Fatal("verifyWebhookPayload() = true pour un payload modifie apres signature, voulu false")
	}
}

func TestVerifyWebhookPayload_malformedSignature(t *testing.T) {
	payload := []byte(`{"event":"message.delivered"}`)
	if verifyWebhookPayload("secret", payload, "pas-hexadecimal-!!") {
		t.Fatal("verifyWebhookPayload() = true pour une signature non-hexadecimale, voulu false")
	}
	if verifyWebhookPayload("secret", payload, "") {
		t.Fatal("verifyWebhookPayload() = true pour une signature vide, voulu false")
	}
}

// TestVerifyWebhookPayload_usesHMACEqual ne peut pas observer directement
// l'usage de hmac.Equal (temps constant), mais verifie au moins que la
// comparaison n'est pas une simple egalite de chaine sensible a la casse (un
// piege frequent d'implementations non conformes) : une signature en
// majuscules (hex valide mais differente au sens strict de ==) doit echouer
// puisque hex.DecodeString est insensible a la casse, MAIS produit les memes
// octets, donc hmac.Equal doit accepter les deux casses.
func TestVerifyWebhookPayload_hexCaseInsensitive(t *testing.T) {
	payload := []byte(`{"event":"message.delivered"}`)
	sig := signWebhookPayload("secret", payload)

	upper := make([]byte, len(sig))
	for i, c := range []byte(sig) {
		if c >= 'a' && c <= 'f' {
			upper[i] = c - 'a' + 'A'
		} else {
			upper[i] = c
		}
	}
	if !verifyWebhookPayload("secret", payload, string(upper)) {
		t.Fatal("verifyWebhookPayload() rejette une signature hex valide en majuscules (hex.DecodeString est insensible a la casse)")
	}
}
