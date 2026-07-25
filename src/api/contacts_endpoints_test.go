package api

// contacts_endpoints_test.go — Tests des handlers HTTP contacts via net/http/httptest.
//
// Perimetre teste (sans base de donnees) :
//   - POST /contacts/outcomes : corps JSON invalide → 400 ; phone manquant → 400 ;
//     channel manquant → 400 ; phone ET channel manquants → 400.
//   - GET /contacts/{phone}/score : extraction du PathValue (parsing), phone vide → 400.
//   - reconstructSuccess : inversion de Laplace (pur, sans I/O).
//   - Verification Content-Type application/json dans les reponses d'erreur.
//
// Perimetre NON teste (necessite une base de donnees reelle) :
//   - Chemins nominaux UPSERT (contact_channel_scores + contacts + history).
//   - Contact inconnu → found=false, delivery_score=50.
//   - Contact connu → found=true, delivery_score=Laplace(success_count, failure_count).
//   - Tri des channel_scores par score DESC.
//   - Atomicite transactionnelle (rollback sur erreur).
//   - Recalcul incremental du score par inversion Laplace (±1 pt, dette D26).
// Ces cas sont couverts par les tests d'acceptance (QA) avec une DB de test.

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---- POST /contacts/outcomes ----

func TestHandleRecordOutcome_invalidBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/contacts/outcomes",
		bytes.NewBufferString(`not valid json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleRecordOutcome(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRecordOutcome_missingPhone(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"channel": "sms",
		"success": true,
	}
	req := jsonRequest(t, http.MethodPost, "/contacts/outcomes", payload)
	rr := httptest.NewRecorder()

	svc.HandleRecordOutcome(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (phone manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRecordOutcome_missingChannel(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"phone":   "+237600000001",
		"success": true,
	}
	req := jsonRequest(t, http.MethodPost, "/contacts/outcomes", payload)
	rr := httptest.NewRecorder()

	svc.HandleRecordOutcome(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (channel manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRecordOutcome_missingPhoneAndChannel(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"success": false,
	}
	req := jsonRequest(t, http.MethodPost, "/contacts/outcomes", payload)
	rr := httptest.NewRecorder()

	svc.HandleRecordOutcome(rr, req)

	// phone manquant → 400 (premier check)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (phone et channel manquants)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRecordOutcome_emptyJSON(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/contacts/outcomes",
		bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleRecordOutcome(rr, req)

	// body JSON vide = phone manquant → 400
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (JSON vide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRecordOutcome_emptyPhone(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"phone":   "",
		"channel": "sms",
		"success": true,
	}
	req := jsonRequest(t, http.MethodPost, "/contacts/outcomes", payload)
	rr := httptest.NewRecorder()

	svc.HandleRecordOutcome(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (phone vide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRecordOutcome_emptyChannel(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"phone":   "+237600000001",
		"channel": "",
		"success": false,
	}
	req := jsonRequest(t, http.MethodPost, "/contacts/outcomes", payload)
	rr := httptest.NewRecorder()

	svc.HandleRecordOutcome(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (channel vide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- GET /contacts/{phone}/score ----

func TestHandleGetContactScore_emptyPhone(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/contacts//score", nil)
	// PathValue vide simule un phone absent du chemin.
	req.SetPathValue("phone", "")
	rr := httptest.NewRecorder()

	svc.HandleGetContactScore(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (phone vide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleGetContactScore_phoneInPathValue(t *testing.T) {
	// Verifie que le handler lit bien r.PathValue("phone") et ne panics pas.
	// Si la DB etait connectee, il irait en base — ici DB est nil donc il paniquerait
	// au Select. On ne test que la validation du PathValue non vide via un phone valide,
	// ce qui atteint le Select et declenche une panique (DB nil) — ce cas est donc
	// marque "necessite DB" et non execute ici. On se limite a phone vide.
	//
	// Ce test s'assure uniquement que le handler extrait correctement le PathValue
	// et retourne 400 quand il est vide.
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/contacts//score", nil)
	req.SetPathValue("phone", "")
	rr := httptest.NewRecorder()

	svc.HandleGetContactScore(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d", rr.Code, http.StatusBadRequest)
	}
}

// ---- reconstructSuccess (pur, sans I/O) ----

// TestReconstructSuccess verifie l'inversion de Laplace pour le recalcul incremental
// du score par canal (D26). Precision documentee : ±1 pt.
func TestReconstructSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		score      int
		sampleSize int
		wantMin    int // borne inferieure acceptable (precision ±1)
		wantMax    int // borne superieure acceptable
	}{
		// Laplace(0,0)=50, n=0 → s_approx doit etre 0
		{"sample_size=0 → 0", 50, 0, 0, 0},
		// Laplace(1,0)=66, n=1 → s = floor(66*3/100)-1 = floor(1.98)-1 = 1-1 = 0
		// Mais l'inversion avec s_approx=0 → f=1 → Laplace(0+1, 1+0)=Laplace(1,1)=50 ≠ 66.
		// La precision ±1 signifie qu'on accepte s_approx in {0,1}.
		{"Laplace(1,0)=66 n=1", 66, 1, 0, 1},
		// Laplace(0,1)=33, n=1 → s = floor(33*3/100)-1 = floor(0.99)-1 = 0-1 = -1 → 0
		{"Laplace(0,1)=33 n=1", 33, 1, 0, 0},
		// Laplace(5,5)=50, n=10 → s = floor(50*12/100)-1 = floor(6)-1 = 5
		{"Laplace(5,5)=50 n=10", 50, 10, 4, 5},
		// Laplace(10,0)=91, n=10 → s = floor(91*12/100)-1 = floor(10.92)-1 = 10-1 = 9
		{"Laplace(10,0)=91 n=10", 91, 10, 9, 10},
		// Laplace(0,10)=8, n=10 → s = floor(8*12/100)-1 = floor(0.96)-1 = 0-1 = -1 → 0
		{"Laplace(0,10)=8 n=10", 8, 10, 0, 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := reconstructSuccess(tc.score, tc.sampleSize)
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("reconstructSuccess(score=%d, n=%d) = %d, voulu dans [%d,%d]",
					tc.score, tc.sampleSize, got, tc.wantMin, tc.wantMax)
			}
		})
	}
}

// TestReconstructSuccess_boundedOutput verifie que reconstructSuccess retourne
// toujours une valeur dans [0, sample_size] (invariant requis pour l'incremental).
//
// Note : la precision de l'inversion Laplace (score, n) → (s_approx, f_approx) est
// deliberement approximee (dette connue D26 — colonne success_count par canal =
// amelioration future). Le test ne verifie PAS que round-trip(Laplace(s,f)) == Laplace(s,f) ;
// il verifie uniquement que s_approx est dans [0, n] (invariant de bornage obligatoire).
//
// La vraie correction serait d'ajouter des colonnes success_count/failure_count dans
// contact_channel_scores (migration additive, tache ulterieure).
// ---- deriveContactScore (fonction pure, sans I/O — extraite de loadContactScore) ----

// TestDeriveContactScore_unknownContact verifie le cas "inconnu" (contactFound=false) :
// prior neutre 50, found=false, quels que soient success/failureCount fournis
// (ignores par la fonction quand contactFound=false).
func TestDeriveContactScore_unknownContact(t *testing.T) {
	t.Parallel()

	score, found := deriveContactScore(false, 0, 0)
	if found {
		t.Errorf("found = true, voulu false (contact inconnu)")
	}
	if score != 50 {
		t.Errorf("score = %d, voulu 50 (prior neutre)", score)
	}
}

// TestDeriveContactScore_knownContact verifie le cas "connu" (contactFound=true) :
// score = Laplace(successCount, failureCount), found=true.
func TestDeriveContactScore_knownContact(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		successCount int
		failureCount int
		wantScore    int
	}{
		{"aucun historique", 0, 0, Laplace(0, 0)},    // 50
		{"succes uniquement", 10, 0, Laplace(10, 0)}, // 91
		{"echecs uniquement", 0, 10, Laplace(0, 10)}, // 8
		{"melange", 7, 3, Laplace(7, 3)},             // 66
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			score, found := deriveContactScore(true, tc.successCount, tc.failureCount)
			if !found {
				t.Errorf("found = false, voulu true (contact connu)")
			}
			if score != tc.wantScore {
				t.Errorf("score = %d, voulu %d", score, tc.wantScore)
			}
		})
	}
}

func TestReconstructSuccess_boundedOutput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		score      int
		sampleSize int
	}{
		{50, 0},
		{66, 1},
		{33, 1},
		{50, 10},
		{91, 10},
		{8, 10},
		{57, 5},
		{77, 25},
		{0, 100},
		{99, 100},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := reconstructSuccess(tc.score, tc.sampleSize)
			// Invariant : s_approx dans [0, sample_size].
			if got < 0 {
				t.Errorf("reconstructSuccess(score=%d, n=%d) = %d < 0 (negatif)",
					tc.score, tc.sampleSize, got)
			}
			if got > tc.sampleSize {
				t.Errorf("reconstructSuccess(score=%d, n=%d) = %d > n=%d (depasse sample_size)",
					tc.score, tc.sampleSize, got, tc.sampleSize)
			}
			// Invariant : le score recalcule est dans [0, 100].
			fApprox := tc.sampleSize - got
			if fApprox < 0 {
				fApprox = 0
			}
			newScore := Laplace(got, fApprox)
			if newScore < 0 || newScore > 100 {
				t.Errorf("Laplace(%d, %d) = %d hors [0,100]", got, fApprox, newScore)
			}
		})
	}
}
