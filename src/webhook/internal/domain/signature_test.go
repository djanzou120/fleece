package domain

import "testing"

// TestSign_Verify_Nominal verifie que Verify retourne true pour une signature valide.
func TestSign_Verify_Nominal(t *testing.T) {
	secret := "mon-secret-de-test"
	payload := []byte(`{"event":"message.delivered","workspace_id":"ws-1"}`)

	sig := Sign(secret, payload)
	if sig == "" {
		t.Fatal("Sign ne doit pas retourner une chaine vide")
	}

	if !Verify(secret, payload, sig) {
		t.Error("Verify doit retourner true pour une signature valide")
	}
}

// TestVerify_MauvaisSecret verifie que Verify retourne false avec un mauvais secret.
func TestVerify_MauvaisSecret(t *testing.T) {
	payload := []byte(`{"event":"message.delivered"}`)
	sig := Sign("bon-secret", payload)

	if Verify("mauvais-secret", payload, sig) {
		t.Error("Verify doit retourner false avec un secret incorrect")
	}
}

// TestVerify_PayloadModifie verifie que Verify retourne false si le payload est altere.
func TestVerify_PayloadModifie(t *testing.T) {
	secret := "mon-secret"
	payload := []byte(`{"event":"message.delivered"}`)
	sig := Sign(secret, payload)

	payloadAltere := []byte(`{"event":"message.failed"}`)
	if Verify(secret, payloadAltere, sig) {
		t.Error("Verify doit retourner false si le payload a ete modifie")
	}
}

// TestVerify_SignatureMalforme verifie que Verify retourne false pour une signature malformee.
func TestVerify_SignatureMalforme(t *testing.T) {
	secret := "mon-secret"
	payload := []byte(`{"event":"message.delivered"}`)

	// Signature qui n'est pas un hexadecimal valide.
	if Verify(secret, payload, "pas-un-hex-valide!@#") {
		t.Error("Verify doit retourner false pour une signature malformee")
	}

	// Chaine vide.
	if Verify(secret, payload, "") {
		t.Error("Verify doit retourner false pour une signature vide")
	}
}

// TestSign_Deterministe verifie que Sign produit toujours la meme valeur pour les memes entrees.
func TestSign_Deterministe(t *testing.T) {
	secret := "secret"
	payload := []byte("payload")

	sig1 := Sign(secret, payload)
	sig2 := Sign(secret, payload)
	if sig1 != sig2 {
		t.Errorf("Sign doit etre deterministe : sig1=%s sig2=%s", sig1, sig2)
	}
}
