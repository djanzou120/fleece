package domain

import "time"

// ChannelScore represente le score de delivrabilite d'un contact sur un canal specifique.
// Cle composite : (Phone, Channel). Correspond a contact_intel.contact_channel_scores.
//
// Stockage en base : seuls score, sample_size et last_success_at sont persistes.
// Les compteurs bruts (successCount, failureCount) sont deduits depuis score et sample_size
// lors du chargement et mis a jour en memoire a chaque RecordOutcome.
//
// Deduction depuis score et sample_size (schema 0014 sans success_count) :
//
//	n = sample_size, s = score (valeur [0,100])
//	successCount ≈ round( s * (n+2) / 100 ) - 1   (inversion de la formule Laplace)
//	failureCount  = n - successCount
//
// Les erreurs d'arrondi introduisent une inexactitude negligeable sur de grands
// echantillons (< 1 point de score). Pour une precision absolue, il faudrait
// stocker success_count en base (amelioration future).
type ChannelScore struct {
	Phone         string
	Channel       Channel
	Score         DeliveryScore
	SampleSize    int
	LastSuccessAt *time.Time
	UpdatedAt     time.Time

	// successCount et failureCount sont derives, non persistes.
	// Ils sont calcules par inferSuccessFailure() apres chargement depuis la DB,
	// ou maintenus directement lors de la creation d'un nouveau ChannelScore.
	successCount int
	failureCount int
}

// NewChannelScore cree un ChannelScore initial (prior neutre).
// Retourne ErrEmptyPhone si phone est vide, ErrInvalidChannel si channel est inconnu.
func NewChannelScore(phone string, ch Channel) (*ChannelScore, error) {
	if phone == "" {
		return nil, ErrEmptyPhone
	}
	if !ch.IsValid() {
		return nil, ErrInvalidChannel
	}
	return &ChannelScore{
		Phone:        phone,
		Channel:      ch,
		Score:        ComputeScore(0, 0), // prior neutre ≈ 50
		SampleSize:   0,
		UpdatedAt:    time.Now().UTC(),
		successCount: 0,
		failureCount: 0,
	}, nil
}

// LoadChannelScore reconstruit un ChannelScore depuis les valeurs persistees en DB.
// Les compteurs successCount/failureCount sont deduits depuis score et sampleSize.
func LoadChannelScore(phone string, ch Channel, score, sampleSize int, lastSuccessAt *time.Time, updatedAt time.Time) *ChannelScore {
	s, f := inferSuccessFailure(score, sampleSize)
	return &ChannelScore{
		Phone:         phone,
		Channel:       ch,
		Score:         NewDeliveryScore(score),
		SampleSize:    sampleSize,
		LastSuccessAt: lastSuccessAt,
		UpdatedAt:     updatedAt,
		successCount:  s,
		failureCount:  f,
	}
}

// inferSuccessFailure deduit les compteurs bruts depuis score et sample_size.
// Inversion approchee de la formule Laplace : score = (s+1)/(s+f+2)*100.
//   s ≈ score*(n+2)/100 - 1
//   f = n - s
func inferSuccessFailure(score, n int) (s, f int) {
	if n == 0 {
		return 0, 0
	}
	// s = round(score*(n+2)/100) - 1
	s = (score*(n+2)+50)/100 - 1 // +50 pour arrondi correct
	if s < 0 {
		s = 0
	}
	if s > n {
		s = n
	}
	f = n - s
	return s, f
}

// RecordOutcome met a jour le score de facon incrementale sur un nouvel outcome.
// Le score est recalcule via ComputeScore (formule de Laplace) sur les compteurs bruts.
func (cs *ChannelScore) RecordOutcome(success bool, at time.Time) {
	if success {
		cs.successCount++
		t := at.UTC()
		cs.LastSuccessAt = &t
	} else {
		cs.failureCount++
	}
	cs.SampleSize++
	cs.Score = ComputeScore(cs.successCount, cs.failureCount)
	cs.UpdatedAt = at.UTC()
}
