package api

// contacts_record_outcome.go — POST /contacts/outcomes
//
// Methode de recalcul du score par canal (D26) :
//
//   La table contact_intel.contact_channel_scores stocke (score, sample_size) mais pas
//   (success_count, failure_count) par canal. Pour l'UPSERT incremental, on reconstruit
//   le nombre de succes en inversant la formule de Laplace :
//
//     Formule directe :  score = floor((s+1)*100 / (s+f+2))  avec n=s+f=sample_size
//     Inversion :        s_approx = max(0, floor(score*(n+2)/100) - 1)
//                        f_approx = max(0, n - s_approx)
//
//   Precision : ±1 pt (dette connue D26 — colonne success_count par canal = amelioration future).
//   Cette approche est documentee dans D26 et acceptee comme dette connue.
//
//   Pour un nouvel enregistrement (canal inexistant) :
//     success=true  → score=Laplace(1,0)=66, sample_size=1
//     success=false → score=Laplace(0,1)=33, sample_size=1
//
//   Pour une mise a jour (ON CONFLICT) :
//     1. Lire (score_courant, sample_size_courant).
//     2. Reconstruire s_approx et f_approx par inversion.
//     3. Appliquer le nouvel outcome.
//     4. Recalculer score = Laplace(s_approx+delta_s, f_approx+delta_f).
//     5. Incrementer sample_size de 1.
//
// Toutes les modifications sont dans une transaction unique (contact_channel_scores
// + contacts + optionnellement contact_channel_history).

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	gosql "fleece/src/go/sql"
)

// recordOutcomeRequest est le corps JSON de POST /contacts/outcomes.
type recordOutcomeRequest struct {
	Phone     string `json:"phone"`
	Channel   string `json:"channel"`
	Success   bool   `json:"success"`
	LatencyMs int    `json:"latency_ms"` // optionnel
}

// HandleRecordOutcome traite POST /contacts/outcomes.
//
// Pipeline :
//  1. Decode + validation (phone et channel non vides → 400).
//  2. BEGIN transaction.
//  3. UPSERT contact_intel.contact_channel_scores avec recalcul incremental du score.
//  4. UPSERT contact_intel.contacts avec increment de success_count ou failure_count.
//  5. INSERT dans contact_intel.contact_channel_history (historique, optionnel).
//  6. COMMIT. Sur erreur → ROLLBACK + 500.
//  7. 204 No Content.
func (s *Service) HandleRecordOutcome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Decode + validation.
	var req recordOutcomeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requete invalide")
		return
	}
	if req.Phone == "" {
		writeError(w, http.StatusBadRequest, "phone est requis")
		return
	}
	if req.Channel == "" {
		writeError(w, http.StatusBadRequest, "channel est requis")
		return
	}

	// 2. BEGIN transaction.
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		s.Logger.Error("contacts/outcomes: begin tx", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	// Defer rollback ; annule si la transaction a deja ete committee.
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()

	// 3. UPSERT contact_intel.contact_channel_scores.
	if err := upsertChannelScore(ctx, tx, req.Phone, req.Channel, req.Success, now); err != nil {
		s.Logger.Error("contacts/outcomes: upsert channel_scores",
			"phone", req.Phone, "channel", req.Channel, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 4. UPSERT contact_intel.contacts.
	if err := upsertContact(ctx, tx, req.Phone, req.Channel, req.Success, now); err != nil {
		s.Logger.Error("contacts/outcomes: upsert contacts",
			"phone", req.Phone, "channel", req.Channel, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 5. Historique (optionnel — ne bloque pas si l'INSERT echoue, mais on rollback quand meme).
	if err := insertHistory(ctx, tx, req.Phone, req.Channel, req.Success, now); err != nil {
		s.Logger.Error("contacts/outcomes: insert history",
			"phone", req.Phone, "channel", req.Channel, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 6. COMMIT.
	if err := tx.Commit(); err != nil {
		s.Logger.Error("contacts/outcomes: commit tx", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 7. 204 No Content.
	w.WriteHeader(http.StatusNoContent)
}

// channelScoreCurrentRow lit le score et sample_size courants depuis
// contact_intel.contact_channel_scores pour un (phone, channel) donne.
// Retourne sql.ErrNoRows si l'enregistrement n'existe pas encore.
type channelScoreCurrentRow struct {
	Score      int `db:"score"`
	SampleSize int `db:"sample_size"`
}

// upsertChannelScore effectue l'UPSERT de contact_intel.contact_channel_scores
// dans la transaction donnee.
//
// Methode de recalcul (D26) :
//   - Si le canal est nouveau (ErrNoRows) : score initial selon outcome.
//   - Sinon : inversion de Laplace pour reconstruire (success, failure) approches,
//     applique le nouvel outcome, recalcule Laplace.
func upsertChannelScore(
	ctx context.Context,
	tx *gosql.Tx,
	phone, channel string,
	success bool,
	now time.Time,
) error {
	// Lire le score et sample_size courants.
	var current channelScoreCurrentRow
	err := tx.Get(ctx, &current,
		`SELECT score, sample_size
		   FROM contact_intel.contact_channel_scores
		  WHERE phone = $1 AND channel = $2`,
		phone, channel,
	)

	var newScore, newSampleSize int

	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		// Nouvel enregistrement : sample_size=1, score selon outcome.
		newSampleSize = 1
		if success {
			newScore = Laplace(1, 0) // 66
		} else {
			newScore = Laplace(0, 1) // 33
		}
	} else {
		// Enregistrement existant : reconstruire (success, failure) par inversion.
		n := current.SampleSize
		sApprox := reconstructSuccess(current.Score, n)
		fApprox := n - sApprox
		if fApprox < 0 {
			fApprox = 0
		}

		// Appliquer le nouvel outcome.
		if success {
			sApprox++
		} else {
			fApprox++
		}
		newScore = Laplace(sApprox, fApprox)
		newSampleSize = n + 1
	}

	// Construction des valeurs last_success_at conditionnelles.
	var lastSuccessAt *time.Time
	if success {
		lastSuccessAt = &now
	}

	if lastSuccessAt != nil {
		return tx.Exec(ctx,
			`INSERT INTO contact_intel.contact_channel_scores
			       (phone, channel, score, sample_size, last_success_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (phone, channel) DO UPDATE
			   SET score          = EXCLUDED.score,
			       sample_size    = EXCLUDED.sample_size,
			       last_success_at = EXCLUDED.last_success_at,
			       updated_at     = EXCLUDED.updated_at`,
			phone, channel, newScore, newSampleSize, lastSuccessAt, now,
		)
	}
	// Si pas de succes, on ne met pas a jour last_success_at (garder la valeur precedente).
	return tx.Exec(ctx,
		`INSERT INTO contact_intel.contact_channel_scores
		       (phone, channel, score, sample_size, updated_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (phone, channel) DO UPDATE
		   SET score       = EXCLUDED.score,
		       sample_size = EXCLUDED.sample_size,
		       updated_at  = EXCLUDED.updated_at`,
		phone, channel, newScore, newSampleSize, now,
	)
}

// upsertContact effectue l'UPSERT de contact_intel.contacts dans la transaction donnee.
//
// A l'INSERT initial :
//   - known_channels = '{channel}' (tableau avec le canal courant).
//   - delivery_score = Laplace(1,0) ou Laplace(0,1) selon outcome.
//   - country = ” (non connu a ce stade).
//   - last_channel = channel.
//   - success_count / failure_count selon outcome.
//
// A la mise a jour (ON CONFLICT) :
//   - Incremente success_count ou failure_count.
//   - Met a jour last_channel, updated_at.
//   - Met a jour last_success_at si success=true.
//   - Recalcule delivery_score via Laplace (approxime : on lit les compteurs actuels
//     et on incremente ; la formule dans le DO UPDATE lit les nouvelles valeurs).
//
// Note : known_channels n'est pas mis a jour dynamiquement ici (amelioration future
// si on veut suivre tous les canaux utilises). Le canal courant est enregistre via
// last_channel.
func upsertContact(
	ctx context.Context,
	tx *gosql.Tx,
	phone, channel string,
	success bool,
	now time.Time,
) error {
	// Calcul du delivery_score pour l'INSERT initial.
	var initScore int
	if success {
		initScore = Laplace(1, 0) // 66
	} else {
		initScore = Laplace(0, 1) // 33
	}

	if success {
		// Succes : incremente success_count, met a jour last_success_at.
		return tx.Exec(ctx,
			`INSERT INTO contact_intel.contacts
			       (phone, known_channels, delivery_score, country,
			        last_channel, last_success_at, success_count, failure_count, updated_at)
			 VALUES ($1, ARRAY[$2]::text[], $3, '', $2, $4, 1, 0, $4)
			 ON CONFLICT (phone) DO UPDATE
			   SET success_count  = contact_intel.contacts.success_count + 1,
			       delivery_score = floor(
			           (contact_intel.contacts.success_count + 1 + 1)::numeric * 100
			           / (contact_intel.contacts.success_count + 1 + contact_intel.contacts.failure_count + 2)
			       )::int,
			       last_channel    = EXCLUDED.last_channel,
			       last_success_at = EXCLUDED.last_success_at,
			       updated_at      = EXCLUDED.updated_at`,
			phone, channel, initScore, now,
		)
	}

	// Echec : incremente failure_count, ne met pas a jour last_success_at.
	return tx.Exec(ctx,
		`INSERT INTO contact_intel.contacts
		       (phone, known_channels, delivery_score, country,
		        last_channel, last_success_at, success_count, failure_count, updated_at)
		 VALUES ($1, ARRAY[$2]::text[], $3, '', $2, NULL, 0, 1, $4)
		 ON CONFLICT (phone) DO UPDATE
		   SET failure_count = contact_intel.contacts.failure_count + 1,
		       delivery_score = floor(
		           (contact_intel.contacts.success_count + 1)::numeric * 100
		           / (contact_intel.contacts.success_count + contact_intel.contacts.failure_count + 1 + 2)
		       )::int,
		       last_channel = EXCLUDED.last_channel,
		       updated_at   = EXCLUDED.updated_at`,
		phone, channel, initScore, now,
	)
}

// insertHistory insere une ligne dans contact_intel.contact_channel_history.
// Colonnes (migration 0008) : phone, channel, success, created_at.
// id est un bigserial AUTO, created_at utilise la valeur now passee.
func insertHistory(
	ctx context.Context,
	tx *gosql.Tx,
	phone, channel string,
	success bool,
	now time.Time,
) error {
	return tx.Exec(ctx,
		`INSERT INTO contact_intel.contact_channel_history (phone, channel, success, created_at)
		 VALUES ($1, $2, $3, $4)`,
		phone, channel, success, now,
	)
}

// reconstructSuccess reconstruit le nombre de succes approche a partir du score
// Laplace stocke et du sample_size (n = success + failure).
//
// Inversion de : score = floor((s+1)*100 / (n+2))
// => s_approx = max(0, floor(score*(n+2)/100) - 1)
//
// Precision : ±1 pt (dette connue D26 — colonne success_count par canal = amelioration future).
func reconstructSuccess(score, sampleSize int) int {
	if sampleSize == 0 {
		return 0
	}
	s := score*(sampleSize+2)/100 - 1
	if s < 0 {
		return 0
	}
	if s > sampleSize {
		return sampleSize
	}
	return s
}
