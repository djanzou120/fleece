package intelligenceprocessor

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================
// parseCampaignRunEvent — fonction pure.
// ============================================================

func TestParseCampaignRunEvent(t *testing.T) {
	got, err := parseCampaignRunEvent([]byte(`{"campaign_id":"` + testCampaignID + `","run_id":"42"}`))
	if err != nil {
		t.Fatalf("parseCampaignRunEvent() erreur inattendue : %v", err)
	}
	want := campaignRunEvent{CampaignID: testCampaignID, RunID: "42"}
	if got != want {
		t.Errorf("parseCampaignRunEvent() = %+v, voulu %+v", got, want)
	}

	if _, err := parseCampaignRunEvent([]byte("not json")); err == nil {
		t.Error("parseCampaignRunEvent() sur JSON illisible attendu en erreur")
	}
}

// ============================================================
// handleCampaignRun — validation, avant toute I/O.
// ============================================================

func TestHandleCampaignRun_payloadIllisible(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	err := handleCampaignRun(context.Background(), svc, []byte("not json"))
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleCampaignRun() sur JSON illisible = %v, voulu une permanentError", err)
	}
}

func TestHandleCampaignRun_campaignIDVide(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	err := handleCampaignRun(context.Background(), svc, []byte(`{"campaign_id":"","run_id":"1"}`))
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleCampaignRun() sur campaign_id vide = %v, voulu une permanentError", err)
	}
}

func TestHandleCampaignRun_campaignIDInvalide(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	err := handleCampaignRun(context.Background(), svc, []byte(`{"campaign_id":"pas-un-uuid","run_id":"1"}`))
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleCampaignRun() sur campaign_id invalide = %v, voulu une permanentError", err)
	}
}

func TestHandleCampaignRun_runIDVide(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	body := []byte(`{"campaign_id":"` + testCampaignID + `","run_id":""}`)
	err := handleCampaignRun(context.Background(), svc, body)
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleCampaignRun() sur run_id vide = %v, voulu une permanentError", err)
	}
}

func TestHandleCampaignRun_runIDNonNumerique(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	body := []byte(`{"campaign_id":"` + testCampaignID + `","run_id":"pas-un-nombre"}`)
	err := handleCampaignRun(context.Background(), svc, body)
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleCampaignRun() sur run_id non numerique = %v, voulu une permanentError", err)
	}
}

// TestHandleCampaignRun_campaignIntrouvable_permanentRejection verifie le
// rejet permanent quand campaign.campaigns ne connait pas l'id.
func TestHandleCampaignRun_campaignIntrouvable_permanentRejection(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"workspace_id", "message_body", "channel_strategy", "rate_limit"})},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	body := campaignRunBody(testCampaignID, "1")
	err := handleCampaignRun(context.Background(), svc, body)
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleCampaignRun() sur campagne introuvable = %v, voulu une permanentError", err)
	}
}

func TestHandleCampaignRun_dbErrorOnCampaignLookup(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{{err: errors.New("connexion perdue")}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	err := handleCampaignRun(context.Background(), svc, campaignRunBody(testCampaignID, "1"))
	if err == nil {
		t.Fatal("handleCampaignRun() attendu en erreur (panne DB campagne)")
	}
	if isPermanentError(err) {
		t.Errorf("erreur DB classee permanente a tort : %v", err)
	}
}

func TestHandleCampaignRun_dbErrorOnRecipientsSelect(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: campaignDetailsFixture(0)},
		{err: errors.New("connexion perdue")},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	err := handleCampaignRun(context.Background(), svc, campaignRunBody(testCampaignID, "1"))
	if err == nil {
		t.Fatal("handleCampaignRun() attendu en erreur (panne DB destinataires)")
	}
	if isPermanentError(err) {
		t.Errorf("erreur DB classee permanente a tort : %v", err)
	}
}

// TestHandleCampaignRun_recipientsQuery_onlyPending verifie que la requete de
// selection des destinataires filtre bien sur status='pending' (reprise d'un
// run interrompu -- voir doc de tete de fichier).
func TestHandleCampaignRun_recipientsQuery_onlyPending(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: campaignDetailsFixture(0)},
		{rows: fakeRowsOfCols([]string{"id", "recipient"})}, // aucun destinataire
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	if err := handleCampaignRun(context.Background(), svc, campaignRunBody(testCampaignID, "1")); err != nil {
		t.Fatalf("handleCampaignRun() erreur inattendue : %v", err)
	}
	if !strings.Contains(conn.queries[1], "status = 'pending'") {
		t.Errorf("requete destinataires inattendue (pas de filtre pending) : %q", conn.queries[1])
	}
}

// ============================================================
// handleCampaignRun — pipeline HTTP complet via httptest.Server.
// ============================================================

// campaignRunBody encode le contrat FIGE {"campaign_id":..., "run_id":...}.
func campaignRunBody(campaignID, runID string) []byte {
	b, _ := json.Marshal(campaignRunEvent{CampaignID: campaignID, RunID: runID})
	return b
}

// campaignDetailsFixture construit un fakeRows a une ligne pour la lecture
// autonome de campaign.campaigns (colonnes dans l'ordre exact de la requete
// de on_campaign_run.go).
func campaignDetailsFixture(rateLimit int) *fakeRows {
	return fakeRowsOfCols(
		[]string{"workspace_id", "message_body", "channel_strategy", "rate_limit"},
		[]driver.Value{testWorkspaceID, "Bonjour !", "cheapest", int64(rateLimit)},
	)
}

// recipientsFixture construit un fakeRows pour campaign.campaign_recipients
// (id, recipient), une ligne par destinataire fourni.
func recipientsFixture(pairs ...struct {
	id        int64
	recipient string
}) *fakeRows {
	rows := make([][]driver.Value, len(pairs))
	for i, p := range pairs {
		rows[i] = []driver.Value{p.id, p.recipient}
	}
	return &fakeRows{columns: []string{"id", "recipient"}, data: rows}
}

func onePendingRecipient(id int64, recipient string) *fakeRows {
	return fakeRowsOfCols([]string{"id", "recipient"}, []driver.Value{id, recipient})
}

// TestHandleCampaignRun_success_singleRecipient couvre le chemin nominal :
// POST reussi (2xx), le message_id retourne persiste dans
// campaign_recipients.message_id, le destinataire passe a 'sent',
// sent_count incremente.
func TestHandleCampaignRun_success_singleRecipient(t *testing.T) {
	var requestsReceived int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestsReceived, 1)
		var req sendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("corps de requete illisible cote serveur : %v", err)
		}
		if req.WorkspaceID != testWorkspaceID || req.Recipient != testRecipient || req.Body != "Bonjour !" || req.Strategy != "cheapest" {
			t.Errorf("requete HTTP inattendue : %+v", req)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(sendMessageResponse{ID: testMessageID, Status: "sent", Cost: 5, ProviderID: "telegram-bot"})
	}))
	defer server.Close()

	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: campaignDetailsFixture(0)},
			{rows: onePendingRecipient(7, testRecipient)},
			{rows: fakeRowsOfCols([]string{"id"}, []driver.Value{int64(7)})}, // markRecipientSent UPDATE ... RETURNING id
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}}, // E6 : reserveRecipientForSending ('pending' -> 'sending')
			{result: fakeExecResult{rows: 1}}, // UPDATE campaign_runs sent_count
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger(), APIURL: server.URL, HTTPClient: server.Client()}

	if err := handleCampaignRun(context.Background(), svc, campaignRunBody(testCampaignID, "1")); err != nil {
		t.Fatalf("handleCampaignRun() erreur inattendue : %v", err)
	}
	if atomic.LoadInt32(&requestsReceived) != 1 {
		t.Fatalf("requetes HTTP recues = %d, voulu 1", requestsReceived)
	}
	// E6, point 1 : reservation atomique 'pending' -> 'sending' AVANT l'envoi HTTP.
	if idx, args, found := findQueryContaining(conn, "SET status = 'sending'"); !found {
		t.Error("aucune requete de reservation ('sending') emise avant l'envoi HTTP")
	} else {
		if args[0].Value != int64(7) {
			t.Errorf("id reserve = %v, voulu 7", args[0].Value)
		}
		if idx != 2 {
			t.Errorf("la reservation doit preceder l'envoi HTTP et le marquage 'sent' (idx=%d, voulu 2)", idx)
		}
	}
	// Marquage 'sent' (depuis 'sending', E6 point 3) avec le message_id retourne par l'API.
	if _, args, found := findQueryContaining(conn, "SET status = 'sent'"); !found {
		t.Error("aucune requete de marquage 'sent' emise")
	} else {
		if args[0].Value != testMessageID {
			t.Errorf("message_id persiste = %v, voulu %q (id retourne par l'API)", args[0].Value, testMessageID)
		}
	}
	if conn.commitCount != 1 {
		t.Errorf("commitCount = %d, voulu 1 (transaction courte markRecipientSent)", conn.commitCount)
	}
	if countQueriesContaining(conn, "sent_count = sent_count + 1") != 1 {
		t.Error("increment sent_count absent")
	}
}

// TestHandleCampaignRun_http4xx_recipientStaysPending verifie qu'un statut
// 4xx ne bloque pas le run : log WARN, destinataire NON marque sent, pas
// d'erreur remontee (un echec isole ne bloque jamais tout le run).
func TestHandleCampaignRun_http4xx_recipientStaysPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: campaignDetailsFixture(0)},
			{rows: onePendingRecipient(7, testRecipient)},
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}}, // E6 : reservation reussie -> l'envoi HTTP a bien lieu
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger(), APIURL: server.URL, HTTPClient: server.Client()}

	if err := handleCampaignRun(context.Background(), svc, campaignRunBody(testCampaignID, "1")); err != nil {
		t.Fatalf("handleCampaignRun() erreur inattendue (echec isole ne doit pas bloquer le run) : %v", err)
	}
	if _, _, found := findQueryContaining(conn, "SET status = 'sending'"); !found {
		t.Error("aucune requete de reservation ('sending') emise")
	}
	// E6, point 4 : echec HTTP isole -> repli 'sending' -> 'pending' (retry possible), jamais de marquage 'sent'.
	if _, args, found := findQueryContaining(conn, "SET status = 'pending'"); !found {
		t.Error("aucun repli 'sending' -> 'pending' emis apres l'echec HTTP")
	} else if args[0].Value != int64(7) {
		t.Errorf("id reverti = %v, voulu 7", args[0].Value)
	}
	if countQueriesContaining(conn, "SET status = 'sent'") != 0 {
		t.Error("marquage 'sent' inattendu sur echec HTTP")
	}
}

// TestHandleCampaignRun_http5xx_recipientStaysPending idem pour un 5xx.
func TestHandleCampaignRun_http5xx_recipientStaysPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: campaignDetailsFixture(0)},
			{rows: onePendingRecipient(7, testRecipient)},
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}}, // E6 : reservation reussie -> l'envoi HTTP a bien lieu
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger(), APIURL: server.URL, HTTPClient: server.Client()}

	if err := handleCampaignRun(context.Background(), svc, campaignRunBody(testCampaignID, "1")); err != nil {
		t.Fatalf("handleCampaignRun() erreur inattendue : %v", err)
	}
	if _, _, found := findQueryContaining(conn, "SET status = 'pending'"); !found {
		t.Error("aucun repli 'sending' -> 'pending' emis apres le 5xx")
	}
	if countQueriesContaining(conn, "SET status = 'sent'") != 0 {
		t.Error("marquage 'sent' inattendu sur 5xx")
	}
}

// TestHandleCampaignRun_invalidJSONBody couvre une reponse 2xx mais corps
// illisible : traite comme un echec (recipient reste pending), pas d'erreur
// globale.
func TestHandleCampaignRun_invalidJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: campaignDetailsFixture(0)},
			{rows: onePendingRecipient(7, testRecipient)},
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}}, // E6 : reservation reussie -> l'envoi HTTP a bien lieu
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger(), APIURL: server.URL, HTTPClient: server.Client()}

	if err := handleCampaignRun(context.Background(), svc, campaignRunBody(testCampaignID, "1")); err != nil {
		t.Fatalf("handleCampaignRun() erreur inattendue : %v", err)
	}
	if _, _, found := findQueryContaining(conn, "SET status = 'pending'"); !found {
		t.Error("aucun repli 'sending' -> 'pending' emis apres le corps invalide")
	}
	if countQueriesContaining(conn, "SET status = 'sent'") != 0 {
		t.Error("marquage 'sent' inattendu sur corps invalide")
	}
}

// TestHandleCampaignRun_emptyIDInResponse couvre une reponse 2xx decodable
// mais sans id -- traite comme un echec (meme raison : impossible de savoir
// quel message a ete cree).
func TestHandleCampaignRun_emptyIDInResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(sendMessageResponse{ID: "", Status: "sent"})
	}))
	defer server.Close()

	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: campaignDetailsFixture(0)},
			{rows: onePendingRecipient(7, testRecipient)},
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}}, // E6 : reservation reussie -> l'envoi HTTP a bien lieu
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger(), APIURL: server.URL, HTTPClient: server.Client()}

	if err := handleCampaignRun(context.Background(), svc, campaignRunBody(testCampaignID, "1")); err != nil {
		t.Fatalf("handleCampaignRun() erreur inattendue : %v", err)
	}
	if _, _, found := findQueryContaining(conn, "SET status = 'pending'"); !found {
		t.Error("aucun repli 'sending' -> 'pending' emis apres l'id vide")
	}
	if countQueriesContaining(conn, "SET status = 'sent'") != 0 {
		t.Error("marquage 'sent' inattendu sur id vide")
	}
}

// TestHandleCampaignRun_timeout verifie qu'un timeout HTTP est traite comme
// un echec (recipient reste pending), avec un client au timeout tres court
// pour ne jamais ralentir la suite de tests.
func TestHandleCampaignRun_timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // largement > timeout client ci-dessous
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: campaignDetailsFixture(0)},
			{rows: onePendingRecipient(7, testRecipient)},
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}}, // E6 : reservation reussie -> l'envoi HTTP a bien lieu
		},
	}
	svc := &Service{
		DB:         newFakeGosqlDB(t, conn),
		Logger:     testLogger(),
		APIURL:     server.URL,
		HTTPClient: &http.Client{Timeout: 5 * time.Millisecond},
	}

	if err := handleCampaignRun(context.Background(), svc, campaignRunBody(testCampaignID, "1")); err != nil {
		t.Fatalf("handleCampaignRun() erreur inattendue (timeout doit etre gere comme un echec isole) : %v", err)
	}
	if _, _, found := findQueryContaining(conn, "SET status = 'pending'"); !found {
		t.Error("aucun repli 'sending' -> 'pending' emis apres le timeout")
	}
	if countQueriesContaining(conn, "SET status = 'sent'") != 0 {
		t.Error("marquage 'sent' inattendu sur timeout")
	}
}

// TestHandleCampaignRun_resume_onlyPendingProcessed verifie la reprise : sur
// N=2 destinataires 'pending' retournes par le SELECT (le filtre SQL lui-meme
// est deja verifie par TestHandleCampaignRun_recipientsQuery_onlyPending),
// les DEUX sont bien traites dans l'ordre.
func TestHandleCampaignRun_resume_onlyPendingProcessed(t *testing.T) {
	var order []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sendMessageRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		order = append(order, req.Recipient)
		_ = json.NewEncoder(w).Encode(sendMessageResponse{ID: testMessageID, Status: "sent"})
	}))
	defer server.Close()

	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: campaignDetailsFixture(0)},
			{rows: recipientsFixture(
				struct {
					id        int64
					recipient string
				}{1, "+237600000001"},
				struct {
					id        int64
					recipient string
				}{2, "+237600000002"},
			)},
			{rows: fakeRowsOfCols([]string{"id"}, []driver.Value{int64(1)})},
			{rows: fakeRowsOfCols([]string{"id"}, []driver.Value{int64(2)})},
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}}, // E6 : reservation recipient 1
			{result: fakeExecResult{rows: 1}}, // sent_count increment recipient 1
			{result: fakeExecResult{rows: 1}}, // E6 : reservation recipient 2
			{result: fakeExecResult{rows: 1}}, // sent_count increment recipient 2
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger(), APIURL: server.URL, HTTPClient: server.Client()}

	if err := handleCampaignRun(context.Background(), svc, campaignRunBody(testCampaignID, "1")); err != nil {
		t.Fatalf("handleCampaignRun() erreur inattendue : %v", err)
	}
	if len(order) != 2 || order[0] != "+237600000001" || order[1] != "+237600000002" {
		t.Fatalf("ordre de traitement inattendu : %v", order)
	}
	if countQueriesContaining(conn, "SET status = 'sending'") != 2 {
		t.Error("les deux destinataires auraient du etre reserves ('sending') avant leur envoi HTTP (E6)")
	}
}

// TestHandleCampaignRun_contextCanceledMidRun verifie que l'annulation du
// contexte AVANT le premier destinataire arrete la boucle immediatement (au
// prochain point de controle) et retourne ctx.Err() (erreur "nue" -> requeue,
// reprise garantie par le filtre status='pending').
func TestHandleCampaignRun_contextCanceledMidRun(t *testing.T) {
	var requestsReceived int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestsReceived, 1)
		_ = json.NewEncoder(w).Encode(sendMessageResponse{ID: testMessageID, Status: "sent"})
	}))
	defer server.Close()

	conn := &fakeConn{querySteps: []queryStep{
		{rows: campaignDetailsFixture(0)},
		{rows: onePendingRecipient(7, testRecipient)},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger(), APIURL: server.URL, HTTPClient: server.Client()}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // annule AVANT l'appel : le point de controle en tete de boucle doit intercepter immediatement.

	err := handleCampaignRun(ctx, svc, campaignRunBody(testCampaignID, "1"))
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("handleCampaignRun() apres annulation = %v, voulu context.Canceled", err)
	}
	if isPermanentError(err) {
		t.Error("l'annulation de contexte ne doit jamais etre classee comme erreur permanente (doit etre requeue)")
	}
	if atomic.LoadInt32(&requestsReceived) != 0 {
		t.Errorf("requetes HTTP recues = %d, voulu 0 (contexte deja annule avant le premier envoi)", requestsReceived)
	}
}

// ============================================================
// E6 — reservation atomique 'pending' -> 'sending' : pas de double envoi.
// ============================================================

// TestHandleCampaignRun_reservationLost_noDuplicateSend couvre le coeur du
// correctif E6 : si reserveRecipientForSending ne touche AUCUNE ligne (0 ligne
// affectee -- le destinataire a deja ete pris en charge par un traitement
// precedent de ce meme campaign.run, ex. redelivery AMQP), handleCampaignRun
// NE DOIT PAS effectuer de second POST /messages pour ce destinataire (ce qui
// prevendrait un message duplique ET un double debit wallet, voir doc de tete
// de fichier). AVANT ce correctif, la sequence POST-puis-marquage ne
// protegeait pas contre ce scenario.
func TestHandleCampaignRun_reservationLost_noDuplicateSend(t *testing.T) {
	var requestsReceived int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestsReceived, 1)
		_ = json.NewEncoder(w).Encode(sendMessageResponse{ID: testMessageID, Status: "sent"})
	}))
	defer server.Close()

	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: campaignDetailsFixture(0)},
			{rows: onePendingRecipient(7, testRecipient)},
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 0}}, // reservation perdue : 0 ligne affectee
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger(), APIURL: server.URL, HTTPClient: server.Client()}

	if err := handleCampaignRun(context.Background(), svc, campaignRunBody(testCampaignID, "1")); err != nil {
		t.Fatalf("handleCampaignRun() erreur inattendue : %v", err)
	}
	if atomic.LoadInt32(&requestsReceived) != 0 {
		t.Errorf("requetes HTTP recues = %d, voulu 0 (reservation perdue -> pas de second envoi, E6)", requestsReceived)
	}
	if countQueriesContaining(conn, "SET status = 'sent'") != 0 {
		t.Error("marquage 'sent' inattendu apres une reservation perdue")
	}
}

// TestHandleCampaignRun_reservationDBError_requeue verifie qu'une panne
// technique DURANT la reservation (avant tout envoi HTTP) est une erreur
// "nue" (transitoire -> requeue), jamais une erreur permanente, et qu'aucun
// POST /messages n'est tente tant que la reservation n'a pas reussi.
func TestHandleCampaignRun_reservationDBError_requeue(t *testing.T) {
	var requestsReceived int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestsReceived, 1)
	}))
	defer server.Close()

	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: campaignDetailsFixture(0)},
			{rows: onePendingRecipient(7, testRecipient)},
		},
		execSteps: []execStep{
			{err: errors.New("connexion perdue")},
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger(), APIURL: server.URL, HTTPClient: server.Client()}

	err := handleCampaignRun(context.Background(), svc, campaignRunBody(testCampaignID, "1"))
	if err == nil {
		t.Fatal("handleCampaignRun() attendu en erreur (panne DB reservation)")
	}
	if isPermanentError(err) {
		t.Errorf("erreur DB de reservation classee permanente a tort : %v", err)
	}
	if atomic.LoadInt32(&requestsReceived) != 0 {
		t.Errorf("requetes HTTP recues = %d, voulu 0 (reservation en echec -> pas d'envoi)", requestsReceived)
	}
}

// TestReserveRecipientForSending_success/alreadyReserved/dbError et
// TestRevertRecipientToPending_success/dbError testent reserveRecipientForSending/
// revertRecipientToPending ISOLEMENT de handleCampaignRun (deja couvertes
// bout-en-bout ci-dessus, complements unitaires ici sur le SQL emis).

func TestReserveRecipientForSending_success(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{result: fakeExecResult{rows: 1}}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn)}

	reserved, err := svc.reserveRecipientForSending(context.Background(), 7)
	if err != nil {
		t.Fatalf("reserveRecipientForSending() erreur inattendue : %v", err)
	}
	if !reserved {
		t.Error("reserveRecipientForSending() = false, voulu true (1 ligne affectee)")
	}
	if !strings.Contains(conn.lastQuery(), "SET status = 'sending'") || !strings.Contains(conn.lastQuery(), "status = 'pending'") {
		t.Errorf("requete de reservation inattendue : %q", conn.lastQuery())
	}
}

func TestReserveRecipientForSending_alreadyReserved(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{result: fakeExecResult{rows: 0}}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn)}

	reserved, err := svc.reserveRecipientForSending(context.Background(), 7)
	if err != nil {
		t.Fatalf("reserveRecipientForSending() erreur inattendue : %v", err)
	}
	if reserved {
		t.Error("reserveRecipientForSending() = true, voulu false (0 ligne affectee -> deja pris en charge)")
	}
}

func TestReserveRecipientForSending_dbError(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{err: errors.New("connexion perdue")}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn)}

	if _, err := svc.reserveRecipientForSending(context.Background(), 7); err == nil {
		t.Fatal("reserveRecipientForSending() attendu en erreur (panne DB)")
	}
}

func TestRevertRecipientToPending_success(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{result: fakeExecResult{rows: 1}}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn)}

	if err := svc.revertRecipientToPending(context.Background(), 7); err != nil {
		t.Fatalf("revertRecipientToPending() erreur inattendue : %v", err)
	}
	if !strings.Contains(conn.lastQuery(), "SET status = 'pending'") || !strings.Contains(conn.lastQuery(), "status = 'sending'") {
		t.Errorf("requete de repli inattendue : %q", conn.lastQuery())
	}
}

func TestRevertRecipientToPending_dbError(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{err: errors.New("connexion perdue")}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn)}

	if err := svc.revertRecipientToPending(context.Background(), 7); err == nil {
		t.Fatal("revertRecipientToPending() attendu en erreur (panne DB)")
	}
}

// ============================================================
// httpClient() — defaut vs injection.
// ============================================================

func TestHTTPClient_defaultsWhenNil(t *testing.T) {
	svc := &Service{}
	c := svc.httpClient()
	if c == nil {
		t.Fatal("httpClient() ne doit jamais retourner nil")
	}
	if c.Timeout != 10*time.Second {
		t.Errorf("timeout par defaut = %v, voulu 10s", c.Timeout)
	}
}

func TestHTTPClient_usesInjected(t *testing.T) {
	injected := &http.Client{Timeout: 3 * time.Second}
	svc := &Service{HTTPClient: injected}
	if got := svc.httpClient(); got != injected {
		t.Error("httpClient() devrait retourner le client injecte")
	}
}

// ============================================================
// markRecipientSent — teste isolement de handleCampaignRun.
// ============================================================

func TestMarkRecipientSent_success(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{{rows: fakeRowsOfCols([]string{"id"}, []driver.Value{int64(7)})}},
		execSteps:  []execStep{{result: fakeExecResult{rows: 1}}},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	if err := svc.markRecipientSent(context.Background(), 7, testMessageID, 1); err != nil {
		t.Fatalf("markRecipientSent() erreur inattendue : %v", err)
	}
	if conn.commitCount != 1 {
		t.Errorf("commitCount = %d, voulu 1", conn.commitCount)
	}
}

// TestMarkRecipientSent_alreadySent_idempotent couvre l'idempotence : 0 ligne
// affectee (deja marque 'sent' par un traitement precedent) -> commit propre,
// AUCUN increment de sent_count.
func TestMarkRecipientSent_alreadySent_idempotent(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{{rows: fakeRowsOfCols([]string{"id"})}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	if err := svc.markRecipientSent(context.Background(), 7, testMessageID, 1); err != nil {
		t.Fatalf("markRecipientSent() erreur inattendue sur double-traitement : %v", err)
	}
	if conn.commitCount != 1 {
		t.Errorf("commitCount = %d, voulu 1 (rien a annuler)", conn.commitCount)
	}
	if conn.execPos != 0 {
		t.Errorf("execPos = %d, voulu 0 (aucun increment de sent_count sur double-traitement)", conn.execPos)
	}
}

func TestMarkRecipientSent_dbError_rollback(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{{err: errors.New("connexion perdue")}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	err := svc.markRecipientSent(context.Background(), 7, testMessageID, 1)
	if err == nil {
		t.Fatal("markRecipientSent() attendu en erreur")
	}
	if conn.commitCount != 0 {
		t.Errorf("commitCount = %d, voulu 0 (aucun commit sur erreur)", conn.commitCount)
	}
}

func TestMarkRecipientSent_dbErrorOnRunIncrement_rollback(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{{rows: fakeRowsOfCols([]string{"id"}, []driver.Value{int64(7)})}},
		execSteps:  []execStep{{err: errors.New("connexion perdue")}},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	err := svc.markRecipientSent(context.Background(), 7, testMessageID, 1)
	if err == nil {
		t.Fatal("markRecipientSent() attendu en erreur (panne increment sent_count)")
	}
	if conn.commitCount != 0 {
		t.Errorf("commitCount = %d, voulu 0 (aucun commit partiel)", conn.commitCount)
	}
}

// ============================================================
// sendCampaignMessage — teste isolement (deja largement couvert par les tests
// bout-en-bout ci-dessus, complements ici pour les erreurs de transport).
// ============================================================

func TestSendCampaignMessage_networkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	badURL := server.URL
	server.Close() // ferme immediatement : toute requete echouera au niveau transport.

	svc := &Service{APIURL: badURL, HTTPClient: &http.Client{Timeout: time.Second}}
	_, err := svc.sendCampaignMessage(context.Background(), svc.httpClient(), campaignDetailsRow{WorkspaceID: testWorkspaceID}, testRecipient)
	if err == nil {
		t.Fatal("sendCampaignMessage() attendu en erreur (serveur ferme)")
	}
}
