package usecases

import (
	"context"
	"testing"

	"fleece/src/webhook/internal/domain"
)

// TestRegisterEndpoint_HappyPath verifie qu'un endpoint valide est persiste et retourne un secret.
func TestRegisterEndpoint_HappyPath(t *testing.T) {
	repo := &mockEndpointRepo{}
	uc := &RegisterEndpoint{Endpoints: repo}

	ep := &domain.WebhookEndpoint{
		ID:          "ep-reg-1",
		WorkspaceID: "ws-reg-1",
		URL:         "https://example.com/hook",
		Events:      []string{"message.delivered"},
	}

	created, err := uc.Execute(context.Background(), ep)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	// L'endpoint doit etre retourne.
	if created == nil {
		t.Fatal("Execute doit retourner l'endpoint cree")
	}

	// Le secret doit etre genere (non vide) car absent a l'entree.
	if created.Secret == "" {
		t.Error("un secret doit etre genere si absent a l'entree")
	}

	// L'endpoint doit etre actif a la creation.
	if !created.Active {
		t.Error("l'endpoint doit etre actif a la creation")
	}

	// L'endpoint doit avoir ete persiste.
	if len(repo.saved) != 1 {
		t.Fatalf("attendu 1 save, obtenu %d", len(repo.saved))
	}
}

// TestRegisterEndpoint_SecretFourniConserve verifie qu'un secret fourni par l'appelant est conserve.
func TestRegisterEndpoint_SecretFourniConserve(t *testing.T) {
	repo := &mockEndpointRepo{}
	uc := &RegisterEndpoint{Endpoints: repo}

	secretFourni := "mon-super-secret"
	ep := &domain.WebhookEndpoint{
		ID:          "ep-reg-2",
		WorkspaceID: "ws-reg-2",
		URL:         "https://example.com/hook",
		Secret:      secretFourni,
		Events:      []string{"wallet.debited"},
	}

	created, err := uc.Execute(context.Background(), ep)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	// Le secret fourni doit etre conserve (non ecrase).
	if created.Secret != secretFourni {
		t.Errorf("secret attendu %q, obtenu %q", secretFourni, created.Secret)
	}
}

// TestRegisterEndpoint_URLVide verifie qu'une URL vide est rejetee avec une erreur.
func TestRegisterEndpoint_URLVide(t *testing.T) {
	repo := &mockEndpointRepo{}
	uc := &RegisterEndpoint{Endpoints: repo}

	ep := &domain.WebhookEndpoint{
		ID:          "ep-reg-3",
		WorkspaceID: "ws-reg-3",
		URL:         "", // vide
		Events:      []string{"message.delivered"},
	}

	_, err := uc.Execute(context.Background(), ep)
	if err == nil {
		t.Fatal("attendu une erreur pour URL vide")
	}

	// Aucun save ne doit etre effectue.
	if len(repo.saved) != 0 {
		t.Errorf("attendu 0 save pour URL vide, obtenu %d", len(repo.saved))
	}
}

// TestRegisterEndpoint_AucunEvenement verifie qu'une liste d'evenements vide est rejetee.
func TestRegisterEndpoint_AucunEvenement(t *testing.T) {
	repo := &mockEndpointRepo{}
	uc := &RegisterEndpoint{Endpoints: repo}

	ep := &domain.WebhookEndpoint{
		ID:          "ep-reg-4",
		WorkspaceID: "ws-reg-4",
		URL:         "https://example.com/hook",
		Events:      []string{}, // vide
	}

	_, err := uc.Execute(context.Background(), ep)
	if err == nil {
		t.Fatal("attendu une erreur pour liste d'evenements vide")
	}

	if len(repo.saved) != 0 {
		t.Errorf("attendu 0 save pour events vide, obtenu %d", len(repo.saved))
	}
}

// TestRegisterEndpoint_AucunEvenementNil verifie qu'une liste d'evenements nil est aussi rejetee.
func TestRegisterEndpoint_AucunEvenementNil(t *testing.T) {
	repo := &mockEndpointRepo{}
	uc := &RegisterEndpoint{Endpoints: repo}

	ep := &domain.WebhookEndpoint{
		ID:          "ep-reg-5",
		WorkspaceID: "ws-reg-5",
		URL:         "https://example.com/hook",
		Events:      nil,
	}

	_, err := uc.Execute(context.Background(), ep)
	if err == nil {
		t.Fatal("attendu une erreur pour events nil")
	}
}

// TestRegisterEndpoint_SecretGenereCryptoRand verifie que le secret genere fait 64 chars hex (256 bits).
func TestRegisterEndpoint_SecretGenereCryptoRand(t *testing.T) {
	repo := &mockEndpointRepo{}
	uc := &RegisterEndpoint{Endpoints: repo}

	ep := &domain.WebhookEndpoint{
		ID:          "ep-reg-6",
		WorkspaceID: "ws-reg-6",
		URL:         "https://example.com/hook",
		Events:      []string{"message.created"},
	}

	created, err := uc.Execute(context.Background(), ep)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	// Un secret hex de 32 octets = 64 caracteres hexadecimaux.
	if len(created.Secret) != 64 {
		t.Errorf("secret genere attendu de longueur 64 (hex 32 octets), obtenu %d: %q",
			len(created.Secret), created.Secret)
	}

	// Verifier que le secret est du hexadecimal valide (chaque caractere est 0-9 ou a-f).
	for i, c := range created.Secret {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("secret genere : caractere invalide %q a la position %d", c, i)
		}
	}
}

// TestRegisterEndpoint_DeuxSecretsDifferents verifie que deux appels successifs sans secret
// generent des secrets differents (aleatoire).
func TestRegisterEndpoint_DeuxSecretsDifferents(t *testing.T) {
	uc := &RegisterEndpoint{Endpoints: &mockEndpointRepo{}}

	ep1 := &domain.WebhookEndpoint{
		ID: "ep-r-7a", WorkspaceID: "ws-r-7", URL: "https://a.com/hook",
		Events: []string{"message.delivered"},
	}
	ep2 := &domain.WebhookEndpoint{
		ID: "ep-r-7b", WorkspaceID: "ws-r-7", URL: "https://b.com/hook",
		Events: []string{"message.delivered"},
	}

	c1, err1 := uc.Execute(context.Background(), ep1)
	c2, err2 := (&RegisterEndpoint{Endpoints: &mockEndpointRepo{}}).Execute(context.Background(), ep2)
	if err1 != nil || err2 != nil {
		t.Fatalf("Execute inattendu: err1=%v err2=%v", err1, err2)
	}

	if c1.Secret == c2.Secret {
		t.Error("deux secrets generes successivement ne doivent pas etre identiques (collision crypto/rand improbable)")
	}
}
