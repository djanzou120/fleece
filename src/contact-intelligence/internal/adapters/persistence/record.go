// Package persistence implemente les repositories du service Contact Intelligence
// via database/sql (schema "contact_intel"). Couche 3 (Interface Adapters).
//
// NOTE DRIVER : ce package n'importe AUCUN driver PostgreSQL concret.
// Le driver doit etre enregistre dans le composition root (main.go) via un
// import anonyme AVANT d'appeler Open().
// Exemple : import _ "github.com/lib/pq"
package persistence

import (
	"database/sql"
	"time"

	"fleece/src/contact-intelligence/internal/domain"
)

// contactRecord est le modele de ligne de contact_intel.contacts.
type contactRecord struct {
	Phone         string
	Country       string
	KnownChannels []string // lu comme TEXT[]
	DeliveryScore int
	LastChannel   string
	LastSuccessAt sql.NullTime
	SuccessCount  int
	FailureCount  int
	UpdatedAt     time.Time
}

// toEntity convertit un contactRecord en entite domaine Contact.
func (r *contactRecord) toEntity() *domain.Contact {
	channels := make([]domain.Channel, 0, len(r.KnownChannels))
	for _, c := range r.KnownChannels {
		ch := domain.Channel(c)
		if ch.IsValid() {
			channels = append(channels, ch)
		}
	}
	var lastSuccessAt *time.Time
	if r.LastSuccessAt.Valid {
		t := r.LastSuccessAt.Time.UTC()
		lastSuccessAt = &t
	}
	return &domain.Contact{
		Phone:         r.Phone,
		Country:       r.Country,
		KnownChannels: channels,
		DeliveryScore: domain.NewDeliveryScore(r.DeliveryScore),
		LastChannel:   domain.Channel(r.LastChannel),
		LastSuccessAt: lastSuccessAt,
		SuccessCount:  r.SuccessCount,
		FailureCount:  r.FailureCount,
		UpdatedAt:     r.UpdatedAt.UTC(),
	}
}

// channelScoreRecord est le modele de ligne de contact_intel.contact_channel_scores.
type channelScoreRecord struct {
	Phone         string
	Channel       string
	Score         int
	SampleSize    int
	LastSuccessAt sql.NullTime
	UpdatedAt     time.Time
}

// toEntity convertit un channelScoreRecord en entite domaine ChannelScore.
// Utilise LoadChannelScore pour deduire les compteurs bruts depuis score et sample_size.
func (r *channelScoreRecord) toEntity() *domain.ChannelScore {
	var lastSuccessAt *time.Time
	if r.LastSuccessAt.Valid {
		t := r.LastSuccessAt.Time.UTC()
		lastSuccessAt = &t
	}
	return domain.LoadChannelScore(
		r.Phone,
		domain.Channel(r.Channel),
		r.Score,
		r.SampleSize,
		lastSuccessAt,
		r.UpdatedAt.UTC(),
	)
}
