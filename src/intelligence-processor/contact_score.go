package intelligenceprocessor

// contact_score.go — mise a jour du scoring de joignabilite (contact_intel),
// DANS la transaction unique de delivery_outcome.go.
//
// DUPLICATION ASSUMEE (documentee, requise par la regle transverse n°5 : ce
// worker ne doit importer NI les 8 anciens services Go NI fleece/src/api) :
//   - Laplace()/clamp() : copie exacte de src/api/scorer.go / src/api/routing.go
//     (fonction pure de 5 lignes chacune — une nouvelle lib partagee src/go/*
//     serait disproportionnee pour 10 lignes de calcul, et un import de
//     fleece/src/api forcerait ce worker a embarquer tout le service HTTP
//     unifie, cf. src/core-processor/on_message_delivered.go pour le meme
//     raisonnement applique a Message.Transition()).
//   - upsertChannelScore()/upsertContactCounters()/reconstructSuccess() :
//     adaptation du pipeline transactionnel de
//     src/api/contacts_record_outcome.go (HandleRecordOutcome), qui realise
//     DEJA exactement cet upsert transactionnel — reference explicite du plan
//     M-021 ("src/api/contacts_record_outcome.go... aligne-toi dessus, c'est
//     la reference"). La methode de reconstruction (succes, echec) approches
//     par inversion de la formule de Laplace (dette connue D26, precision
//     ±1 pt) est identique.
//
// Colonnes exactes utilisees (verifiees dans migrations/0008_contact_intel.sql
// et migrations/0014_contact_intel_scoring.sql avant d'ecrire ce fichier) :
//   - contact_intel.contact_channel_scores(phone, channel, score, sample_size,
//     last_success_at, updated_at) — PRIMARY KEY (phone, channel), migration 0014.
//   - contact_intel.contacts(phone, known_channels, delivery_score, country,
//     last_channel, last_success_at, success_count, failure_count, updated_at)
//     — PRIMARY KEY (phone), migrations 0008 + 0014.
import (
	"context"
	"database/sql"
	"errors"
	"time"

	gosql "fleece/src/go/sql"
)

// Laplace calcule un score de joignabilite par lissage de Laplace (alpha=1).
// Identique a src/api/scorer.go (voir doc de tete de fichier — duplication
// assumee).
//
// Formule : floor((success+1)/(success+failure+2)*100), borne [0,100].
func Laplace(success, failure int) int {
	if success < 0 {
		success = 0
	}
	if failure < 0 {
		failure = 0
	}
	numerator := (success + 1) * 100
	denominator := success + failure + 2
	score := numerator / denominator
	return clamp(score, 0, 100)
}

// clamp borne v dans [lo, hi]. Identique a src/api/routing.go (duplication
// assumee, voir doc de tete de fichier).
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// channelScoreCurrentRow lit le score et sample_size courants depuis
// contact_intel.contact_channel_scores pour un (phone, channel) donne.
type channelScoreCurrentRow struct {
	Score      int `db:"score"`
	SampleSize int `db:"sample_size"`
}

// upsertChannelScore effectue l'UPSERT de contact_intel.contact_channel_scores
// dans la transaction donnee. Adaptation de
// src/api/contacts_record_outcome.go:upsertChannelScore.
func upsertChannelScore(ctx context.Context, tx *gosql.Tx, phone, channel string, success bool, now time.Time) error {
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
		newSampleSize = 1
		if success {
			newScore = Laplace(1, 0)
		} else {
			newScore = Laplace(0, 1)
		}
	} else {
		n := current.SampleSize
		sApprox := reconstructSuccess(current.Score, n)
		fApprox := n - sApprox
		if fApprox < 0 {
			fApprox = 0
		}
		if success {
			sApprox++
		} else {
			fApprox++
		}
		newScore = Laplace(sApprox, fApprox)
		newSampleSize = n + 1
	}

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
			   SET score           = EXCLUDED.score,
			       sample_size     = EXCLUDED.sample_size,
			       last_success_at = EXCLUDED.last_success_at,
			       updated_at      = EXCLUDED.updated_at`,
			phone, channel, newScore, newSampleSize, lastSuccessAt, now,
		)
	}
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

// upsertContactCounters effectue l'UPSERT de contact_intel.contacts dans la
// transaction donnee : incremente success_count/failure_count, met a jour
// last_channel/last_success_at/updated_at. Adaptation de
// src/api/contacts_record_outcome.go:upsertContact (renommee ici pour eviter
// toute confusion avec un hypothetique type "Contact" du domaine).
func upsertContactCounters(ctx context.Context, tx *gosql.Tx, phone, channel string, success bool, now time.Time) error {
	var initScore int
	if success {
		initScore = Laplace(1, 0)
	} else {
		initScore = Laplace(0, 1)
	}

	if success {
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

// reconstructSuccess reconstruit le nombre de succes approche a partir du
// score Laplace stocke et du sample_size (n = success + failure). Identique a
// src/api/contacts_record_outcome.go:reconstructSuccess (dette D26, precision
// ±1 pt — duplication assumee, voir doc de tete de fichier).
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
