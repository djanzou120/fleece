package api

// webhooks_endpoints_test.go — Tests des handlers HTTP webhooks via net/http/httptest.
//
// Perimetre teste (sans base de donnees, sans AMQP) :
//
//   POST /webhooks/om — secret configure (Service.OMWebhookSecret non vide) :
//     - Body illisible (simule par body ferme) : 200 (Telegram/payment ne doit pas reessayer).
//     - JSON invalide : 200 (meme logique, secret VIDE pour atteindre le parsing JSON).
//     - Header X-OM-Signature invalide → 401.
//     - Header X-OM-Signature absent → 401 (signature obligatoire quand le secret est configure).
//     - Header X-OM-Signature valide → 200.
//
//   POST /webhooks/om — secret VIDE (Service.OMWebhookSecret == "") :
//     - Aucune signature : 200 (log WARN, validation impossible sans cle).
//     - Payload valide sans signature : 200.
//
//   POST /webhooks/mtn — meme politique que OM (secret configure vs. vide) :
//     - Body JSON invalide : 200.
//     - Header X-MTN-Signature invalide → 401.
//     - Header X-MTN-Signature absent + secret configure → 401.
//     - Header X-MTN-Signature absent + secret vide → 200.
//
//   POST /webhooks/telegram :
//     - JSON invalide : 200 (Telegram attend toujours 200).
//     - Update sans message (autre type d'update) : 200.
//     - Update avec message sans delivery_status : 200.
//     - Update avec delivery_status mais sans fleece_message_id : 200
//       (correlation impossible — log + evenement AMQP non publie car AMQP nil).
//     - Update avec fleece_message_id invalide (non UUID) : 200.
//     - Update avec delivery_status "delivered" → routing key "message.delivered".
//     - Update avec delivery_status "failed" → routing key "message.failed".
//
//   validateHMACSignature (fonction pure) :
//     - Signature correcte : true.
//     - Signature incorrecte : false.
//     - Signature vide : false.
//
//   mapTelegramStatusToRoutingKey (fonction pure) :
//     - "delivered" → "message.delivered".
//     - "read" → "message.delivered".
//     - "failed" → "message.failed".
//     - valeur inconnue → "message.failed".
//
//   mapTelegramStatusToFleeceStatus (fonction pure) :
//     - "delivered" → "delivered".
//     - "read" → "delivered".
//     - "failed" → "failed".
//     - valeur inconnue → "failed".
//
//   mapOMStatus / mapMTNStatus (fonctions pures) :
//     - Tous les statuts operateur reconnus + statut inconnu → "".
//
//   Reconciliation wallet (POST /webhooks/om et /webhooks/mtn) :
//     - Callback sans transaction_id/reference_id → 200, aucun acces DB
//       (s.DB reste nil dans ces tests — un acces reel paniquerait).
//     - Callback avec transaction_id/reference_id mais statut operateur inconnu
//       → 200, aucun acces DB (mapping vide → reconciliation ignoree en amont).
//
// Perimetre NON teste (necessite DB ou AMQP reel) :
//   - UPDATE messaging.messages SET status (necessite DB).
//   - Publication AMQP "message.delivered" / "message.failed" (necessite AMQP reel).
//   - UPDATE wallet.wallet_transactions reel (necessite DB) : voir wallet_endpoints_test.go
//     et les tests d'acceptance QA pour le chemin nominal (rowsAffected 0/1/>1).
//   - Validation HMAC avec la vraie cle de production.
// Ces cas sont couverts par les tests d'acceptance (QA) avec infrastructure reelle.
//
// NOTE IMPORTANTE : les tests ci-dessous qui exercent un callback "valide" (payload
// complet + signature correcte) utilisent volontairement un transaction_id/
// reference_id VIDE. Des lors que ce champ est present et que le statut operateur
// est reconnu, le handler appelle s.DB.ExecRows — hors DB reelle (s.DB nil dans ces
// tests), cela paniquerait. Les cas "transaction_id present + statut reconnu" ne
// sont donc pas exercables ici sans DB ; voir les tests d'acceptance QA.

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	golog "fleece/src/go/log"
)

// ============================================================
// Helpers de test webhook
// ============================================================

// testOMSecret / testMTNSecret sont les secrets HMAC utilises par les tests qui
// exercent la politique "secret configure" (signature obligatoire).
const (
	testOMSecret  = "test-om-secret"
	testMTNSecret = "test-mtn-secret"
)

// newTestServiceWithLogger cree un *Service minimal avec un Logger initialise et
// AUCUN secret webhook configure (OMWebhookSecret/MTNWebhookSecret vides) —
// reproduit la politique "secret non configure" : validation HMAC desactivee,
// callback toujours accepte (log WARN).
// Necessaire pour les handlers webhook qui appellent s.Logger dans tous les chemins
// (y compris sur JSON invalide — contrairement aux handlers de validation rapide
// qui returnent avant d'atteindre le Logger).
// DB et AMQP restent nil : tout acces a ces champs planterait le test (cas marques
// "necessite DB/AMQP" dans le commentaire de perimetre).
func newTestServiceWithLogger() *Service {
	return &Service{
		Providers: make(map[string]Provider),
		Logger:    golog.Init("warn", "text"), // niveau warn : evite les sorties verbose dans les tests.
	}
}

// newTestServiceWithSecrets cree un *Service avec des secrets webhook configures —
// reproduit la politique "secret configure" : signature HMAC obligatoire.
func newTestServiceWithSecrets() *Service {
	return &Service{
		Providers:        make(map[string]Provider),
		Logger:           golog.Init("warn", "text"),
		OMWebhookSecret:  testOMSecret,
		MTNWebhookSecret: testMTNSecret,
	}
}

// buildHMACSignature genere un header HMAC-SHA256 correct pour le body donne.
// Utilise le meme algorithme que validateHMACSignature.
func buildHMACSignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// ============================================================
// Tests POST /webhooks/om
// ============================================================

func TestHandleWebhookOM_invalidJSON(t *testing.T) {
	svc := newTestServiceWithLogger()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/om",
		bytes.NewBufferString(`not valid json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookOM(rr, req)

	// JSON invalide : le handler doit quand meme repondre 200
	// (Orange Money ne doit pas reessayer sur un payload malformed).
	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (JSON invalide → 200 toujours)", rr.Code, http.StatusOK)
	}
}

func TestHandleWebhookOM_validSignature(t *testing.T) {
	svc := newTestServiceWithSecrets()
	// transaction_id volontairement vide : voir NOTE IMPORTANTE en tete de fichier
	// (evite un acces a s.DB, nil dans ce test offline).
	payload := omPayload{
		TransactionID: "",
		Status:        "SUCCESS",
		Amount:        5000,
		Currency:      "XAF",
	}
	body, _ := json.Marshal(payload)
	sig := buildHMACSignature(body, testOMSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/om", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OM-Signature", sig)
	rr := httptest.NewRecorder()

	svc.HandleWebhookOM(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (signature valide → 200)", rr.Code, http.StatusOK)
	}
}

func TestHandleWebhookOM_invalidSignature(t *testing.T) {
	svc := newTestServiceWithSecrets()
	payload := omPayload{
		TransactionID: "OM-TXN-002",
		Status:        "SUCCESS",
		Amount:        1000,
		Currency:      "XAF",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/om", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Signature intentionnellement incorrecte.
	req.Header.Set("X-OM-Signature", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	rr := httptest.NewRecorder()

	svc.HandleWebhookOM(rr, req)

	// Signature presente mais invalide → 401.
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, voulu %d (signature invalide → 401)", rr.Code, http.StatusUnauthorized)
	}
	assertJSONError(t, rr)
}

func TestHandleWebhookOM_noSignatureHeader_secretConfigured(t *testing.T) {
	svc := newTestServiceWithSecrets()
	payload := omPayload{
		TransactionID: "OM-TXN-003",
		Status:        "PENDING",
		Amount:        2000,
		Currency:      "XAF",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/om", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Pas de header X-OM-Signature, secret configure → signature obligatoire.
	rr := httptest.NewRecorder()

	svc.HandleWebhookOM(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, voulu %d (secret configure, signature absente → 401)", rr.Code, http.StatusUnauthorized)
	}
	assertJSONError(t, rr)
}

func TestHandleWebhookOM_noSignatureHeader_secretEmpty(t *testing.T) {
	svc := newTestServiceWithLogger() // OMWebhookSecret == "" (non configure)
	// transaction_id volontairement vide : voir NOTE IMPORTANTE en tete de fichier.
	payload := omPayload{
		TransactionID: "",
		Status:        "PENDING",
		Amount:        2000,
		Currency:      "XAF",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/om", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Pas de header X-OM-Signature, secret vide → validation impossible, accepte (log WARN).
	rr := httptest.NewRecorder()

	svc.HandleWebhookOM(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (secret non configure → 200)", rr.Code, http.StatusOK)
	}
}

func TestHandleWebhookOM_validPayloadNoSignature(t *testing.T) {
	svc := newTestServiceWithLogger()
	// Payload avec message_id correle (champ optionnel). transaction_id
	// volontairement vide : voir NOTE IMPORTANTE en tete de fichier.
	payload := map[string]any{
		"transaction_id": "",
		"status":         "SUCCESS",
		"amount":         10000,
		"currency":       "XAF",
		"message_id":     "550e8400-e29b-41d4-a716-446655440000",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/om", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookOM(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (payload valide sans sig → 200)", rr.Code, http.StatusOK)
	}
}

// ============================================================
// Tests POST /webhooks/mtn
// ============================================================

func TestHandleWebhookMTN_invalidJSON(t *testing.T) {
	svc := newTestServiceWithLogger()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/mtn",
		bytes.NewBufferString(`{broken json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookMTN(rr, req)

	// MTN ne doit pas reessayer sur un payload invalide → 200.
	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (JSON invalide → 200)", rr.Code, http.StatusOK)
	}
}

func TestHandleWebhookMTN_invalidSignature(t *testing.T) {
	svc := newTestServiceWithSecrets()
	payload := mtnPayload{
		ReferenceID: "MTN-REF-001",
		Status:      "SUCCESSFUL",
		Amount:      "5000",
		Currency:    "XAF",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/mtn", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Signature intentionnellement incorrecte.
	req.Header.Set("X-MTN-Signature", "0000000000000000000000000000000000000000000000000000000000000000")
	rr := httptest.NewRecorder()

	svc.HandleWebhookMTN(rr, req)

	// Signature presente mais invalide → 401.
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, voulu %d (signature invalide → 401)", rr.Code, http.StatusUnauthorized)
	}
	assertJSONError(t, rr)
}

func TestHandleWebhookMTN_validSignature(t *testing.T) {
	svc := newTestServiceWithSecrets()
	// reference_id volontairement vide : voir NOTE IMPORTANTE en tete de fichier
	// (evite un acces a s.DB, nil dans ce test offline).
	payload := mtnPayload{
		ReferenceID: "",
		Status:      "SUCCESSFUL",
		Amount:      "3000",
		Currency:    "XAF",
	}
	body, _ := json.Marshal(payload)
	sig := buildHMACSignature(body, testMTNSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/mtn", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MTN-Signature", sig)
	rr := httptest.NewRecorder()

	svc.HandleWebhookMTN(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (signature valide → 200)", rr.Code, http.StatusOK)
	}
}

func TestHandleWebhookMTN_noSignatureHeader_secretConfigured(t *testing.T) {
	svc := newTestServiceWithSecrets()
	payload := mtnPayload{
		ReferenceID: "MTN-REF-003",
		Status:      "PENDING",
		Amount:      "1000",
		Currency:    "XAF",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/mtn", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookMTN(rr, req)

	// Secret configure, header absent → signature obligatoire → 401.
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, voulu %d (secret configure, signature absente → 401)", rr.Code, http.StatusUnauthorized)
	}
	assertJSONError(t, rr)
}

func TestHandleWebhookMTN_noSignatureHeader_secretEmpty(t *testing.T) {
	svc := newTestServiceWithLogger() // MTNWebhookSecret == "" (non configure)
	// reference_id volontairement vide : voir NOTE IMPORTANTE en tete de fichier.
	payload := mtnPayload{
		ReferenceID: "",
		Status:      "PENDING",
		Amount:      "1000",
		Currency:    "XAF",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/mtn", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookMTN(rr, req)

	// Secret vide → validation impossible, accepte (log WARN).
	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (secret non configure → 200)", rr.Code, http.StatusOK)
	}
}

// ============================================================
// Tests reconciliation wallet (POST /webhooks/om, /webhooks/mtn)
// ============================================================

// TestHandleWebhookOM_reconcile_noTransactionID verifie que l'absence de
// transaction_id court-circuite toute tentative de reconciliation (donc tout
// acces a s.DB, nil dans ce test) et repond 200.
func TestHandleWebhookOM_reconcile_noTransactionID(t *testing.T) {
	svc := newTestServiceWithLogger()
	payload := omPayload{
		TransactionID: "", // absent
		Status:        "SUCCESS",
		Amount:        1500,
		Currency:      "XAF",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/om", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookOM(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (transaction_id absent → 200, pas d'acces DB)", rr.Code, http.StatusOK)
	}
}

// TestHandleWebhookOM_reconcile_unknownStatus verifie qu'un statut operateur non
// reconnu court-circuite la reconciliation (mapOMStatus retourne "") et repond 200,
// sans tenter d'acces DB.
func TestHandleWebhookOM_reconcile_unknownStatus(t *testing.T) {
	svc := newTestServiceWithLogger()
	payload := omPayload{
		TransactionID: "OM-TXN-999", // present, mais statut inconnu
		Status:        "SOMETHING_ELSE",
		Amount:        1500,
		Currency:      "XAF",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/om", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookOM(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (statut operateur inconnu → 200, pas d'acces DB)", rr.Code, http.StatusOK)
	}
}

// TestHandleWebhookMTN_reconcile_noReferenceID verifie l'equivalent MTN de
// TestHandleWebhookOM_reconcile_noTransactionID.
func TestHandleWebhookMTN_reconcile_noReferenceID(t *testing.T) {
	svc := newTestServiceWithLogger()
	payload := mtnPayload{
		ReferenceID: "", // absent
		Status:      "SUCCESSFUL",
		Amount:      "1500",
		Currency:    "XAF",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/mtn", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookMTN(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (reference_id absent → 200, pas d'acces DB)", rr.Code, http.StatusOK)
	}
}

// TestHandleWebhookMTN_reconcile_unknownStatus verifie l'equivalent MTN de
// TestHandleWebhookOM_reconcile_unknownStatus.
func TestHandleWebhookMTN_reconcile_unknownStatus(t *testing.T) {
	svc := newTestServiceWithLogger()
	payload := mtnPayload{
		ReferenceID: "MTN-REF-999", // present, mais statut inconnu
		Status:      "SOMETHING_ELSE",
		Amount:      "1500",
		Currency:    "XAF",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/mtn", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookMTN(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (statut operateur inconnu → 200, pas d'acces DB)", rr.Code, http.StatusOK)
	}
}

// ============================================================
// Tests POST /webhooks/telegram
// ============================================================

func TestHandleWebhookTelegram_invalidJSON(t *testing.T) {
	svc := newTestServiceWithLogger()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram",
		bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookTelegram(rr, req)

	// Telegram exige 200 meme sur JSON invalide.
	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (JSON invalide → 200 toujours Telegram)", rr.Code, http.StatusOK)
	}
}

func TestHandleWebhookTelegram_updateWithoutMessage(t *testing.T) {
	svc := newTestServiceWithLogger()
	// Update sans champ "message" (ex. callback_query, inline_query).
	update := telegramUpdate{
		UpdateID: 100001,
	}
	body, _ := json.Marshal(update)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookTelegram(rr, req)

	// Update non reconnue → 200 systematiquement.
	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (update sans message → 200)", rr.Code, http.StatusOK)
	}
}

func TestHandleWebhookTelegram_messageWithoutDeliveryStatus(t *testing.T) {
	svc := newTestServiceWithLogger()
	update := telegramUpdate{
		UpdateID: 100002,
		Message: &telegramMessage{
			MessageID: 42,
			Text:      "Hello",
			// Pas de delivery_status : message entrant ordinaire.
		},
	}
	body, _ := json.Marshal(update)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookTelegram(rr, req)

	// Pas de DLR → 200.
	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (message sans delivery_status → 200)", rr.Code, http.StatusOK)
	}
}

func TestHandleWebhookTelegram_dlrWithoutFleeceMsgID(t *testing.T) {
	svc := newTestServiceWithLogger()
	// DLR reconnaissable mais sans identifiant exploitable pour la correlation
	// (ni message_id Telegram, ni fleece_message_id) → correlation impossible.
	// NOTE : depuis la migration 0018, resolveTelegramCorrelation privilegie
	// message_id (→ external_id) des qu'il est non nul (voir webhooks_telegram.go) ;
	// pour exercer reellement le chemin "aucune correlation possible" (qui ne doit
	// toucher aucune DB — s.DB reste nil dans ce test offline), message_id doit
	// donc etre absent (zero) ici, et pas seulement fleece_message_id.
	update := telegramUpdate{
		UpdateID: 100003,
		Message: &telegramMessage{
			DeliveryStatus: "delivered",
			// Pas de MessageID (0), pas de FleeceMsgID.
		},
	}
	body, _ := json.Marshal(update)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookTelegram(rr, req)

	// 200 meme sans correlation.
	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (DLR sans fleece_message_id → 200)", rr.Code, http.StatusOK)
	}
}

func TestHandleWebhookTelegram_dlrWithInvalidFleeceMsgID(t *testing.T) {
	svc := newTestServiceWithLogger()
	// message_id volontairement absent (0) : sinon resolveTelegramCorrelation
	// privilegierait la correlation external_id (voir webhooks_telegram.go) et le
	// handler tenterait un acces DB reel — s.DB reste nil dans ce test offline.
	// Ce test exerce donc specifiquement le chemin de compatibilite
	// fleece_message_id, avec un UUID invalide → aucune correlation possible.
	update := telegramUpdate{
		UpdateID: 100004,
		Message: &telegramMessage{
			DeliveryStatus: "delivered",
			FleeceMsgID:    "not-a-uuid", // UUID invalide.
		},
	}
	body, _ := json.Marshal(update)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookTelegram(rr, req)

	// 200 meme avec UUID invalide.
	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (fleece_message_id invalide → 200)", rr.Code, http.StatusOK)
	}
}

func TestHandleWebhookTelegram_emptyBody(t *testing.T) {
	svc := newTestServiceWithLogger()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/telegram",
		bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleWebhookTelegram(rr, req)

	// Body vide mais JSON valide (update_id=0, pas de message) → 200.
	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (body JSON vide → 200)", rr.Code, http.StatusOK)
	}
}

// ============================================================
// Tests fonctions pures
// ============================================================

func TestValidateHMACSignature_correct(t *testing.T) {
	body := []byte(`{"test":"data"}`)
	secret := "super-secret-key"
	sig := buildHMACSignature(body, secret)

	if !validateHMACSignature(body, sig, secret) {
		t.Error("validateHMACSignature: signature correcte retourne false")
	}
}

func TestValidateHMACSignature_incorrect(t *testing.T) {
	body := []byte(`{"test":"data"}`)
	secret := "super-secret-key"

	if validateHMACSignature(body, "badhash", secret) {
		t.Error("validateHMACSignature: signature incorrecte retourne true")
	}
}

func TestValidateHMACSignature_empty(t *testing.T) {
	body := []byte(`{"test":"data"}`)
	secret := "super-secret-key"

	if validateHMACSignature(body, "", secret) {
		t.Error("validateHMACSignature: signature vide retourne true")
	}
}

func TestValidateHMACSignature_bodyMismatch(t *testing.T) {
	secret := "secret"
	originalBody := []byte(`{"amount":100}`)
	tamperedBody := []byte(`{"amount":999}`)
	sig := buildHMACSignature(originalBody, secret)

	// Signature du body original ne doit pas valider un body tampered.
	if validateHMACSignature(tamperedBody, sig, secret) {
		t.Error("validateHMACSignature: corps modifie valide la signature originale (erreur)")
	}
}

// ---- mapOMStatus ----

func TestMapOMStatus(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"SUCCESS", "completed"},
		{"FAILED", "failed"},
		{"PENDING", "pending"},
		{"", ""},
		{"unknown", ""},
		{"success", ""}, // casse non normalisee : seule la casse operateur exacte est reconnue
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := mapOMStatus(tc.input)
			if got != tc.want {
				t.Errorf("mapOMStatus(%q) = %q, voulu %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---- mapMTNStatus ----

func TestMapMTNStatus(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"SUCCESSFUL", "completed"},
		{"FAILED", "failed"},
		{"PENDING", "pending"},
		{"", ""},
		{"unknown", ""},
		{"SUCCESS", ""}, // vocabulaire OM, pas reconnu par MTN (mapMTNStatus attend "SUCCESSFUL")
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := mapMTNStatus(tc.input)
			if got != tc.want {
				t.Errorf("mapMTNStatus(%q) = %q, voulu %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---- mapTelegramStatusToRoutingKey ----

func TestMapTelegramStatusToRoutingKey(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"delivered", "message.delivered"},
		{"read", "message.delivered"},
		{"failed", "message.failed"},
		{"", "message.failed"},
		{"unknown_status", "message.failed"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := mapTelegramStatusToRoutingKey(tc.input)
			if got != tc.want {
				t.Errorf("mapTelegramStatusToRoutingKey(%q) = %q, voulu %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---- resolveTelegramCorrelation ----

// TestResolveTelegramCorrelation couvre la fonction pure de decision de
// correlation DLR Telegram (D-M13) : aucune I/O, testable sans DB. Depuis la
// Phase 3, la fonction retourne aussi ChatID (chat_id Telegram, second membre
// de la clef de correlation (chat_id, message_id) ≡ (recipient, external_id)).
func TestResolveTelegramCorrelation(t *testing.T) {
	cases := []struct {
		name       string
		msg        *telegramMessage
		wantColumn string
		wantValue  string
		wantChatID string
	}{
		{
			name:       "message_id non nul -> correlation par external_id",
			msg:        &telegramMessage{MessageID: 42},
			wantColumn: "external_id",
			wantValue:  "42",
			wantChatID: "",
		},
		{
			name:       "message_id non nul + chat.id present -> correlation par external_id avec chat_id (D-M13)",
			msg:        &telegramMessage{MessageID: 42, Chat: &telegramChat{ID: 987654321}},
			wantColumn: "external_id",
			wantValue:  "42",
			wantChatID: "987654321",
		},
		{
			name:       "message_id non nul + chat present mais chat.id == 0 -> chat_id absent (D-M13, repli)",
			msg:        &telegramMessage{MessageID: 42, Chat: &telegramChat{ID: 0}},
			wantColumn: "external_id",
			wantValue:  "42",
			wantChatID: "",
		},
		{
			name:       "message_id nul + fleece_message_id UUID valide -> correlation par id",
			msg:        &telegramMessage{FleeceMsgID: "550e8400-e29b-41d4-a716-446655440000"},
			wantColumn: "id",
			wantValue:  "550e8400-e29b-41d4-a716-446655440000",
			wantChatID: "",
		},
		{
			name:       "message_id nul + fleece_message_id invalide -> aucune correlation",
			msg:        &telegramMessage{FleeceMsgID: "not-a-uuid"},
			wantColumn: "",
			wantValue:  "",
			wantChatID: "",
		},
		{
			name:       "message_id nul + fleece_message_id absent -> aucune correlation",
			msg:        &telegramMessage{},
			wantColumn: "",
			wantValue:  "",
			wantChatID: "",
		},
		{
			name:       "message_id non nul prioritaire sur fleece_message_id valide",
			msg:        &telegramMessage{MessageID: 7, FleeceMsgID: "550e8400-e29b-41d4-a716-446655440000"},
			wantColumn: "external_id",
			wantValue:  "7",
			wantChatID: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveTelegramCorrelation(tc.msg)
			if got.Column != tc.wantColumn {
				t.Errorf("resolveTelegramCorrelation(%+v).Column = %q, voulu %q", tc.msg, got.Column, tc.wantColumn)
			}
			if got.Value != tc.wantValue {
				t.Errorf("resolveTelegramCorrelation(%+v).Value = %q, voulu %q", tc.msg, got.Value, tc.wantValue)
			}
			if got.ChatID != tc.wantChatID {
				t.Errorf("resolveTelegramCorrelation(%+v).ChatID = %q, voulu %q", tc.msg, got.ChatID, tc.wantChatID)
			}
		})
	}
}

// ---- mapTelegramStatusToFleeceStatus ----

func TestMapTelegramStatusToFleeceStatus(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"delivered", string(MessageDelivered)},
		{"read", string(MessageDelivered)},
		{"failed", string(MessageFailed)},
		{"", string(MessageFailed)},
		{"unknown", string(MessageFailed)},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := mapTelegramStatusToFleeceStatus(tc.input)
			if got != tc.want {
				t.Errorf("mapTelegramStatusToFleeceStatus(%q) = %q, voulu %q", tc.input, got, tc.want)
			}
		})
	}
}
