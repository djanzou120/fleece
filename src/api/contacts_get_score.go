package api

// contacts_get_score.go — GET /contacts/{phone}/score
//
// Contrat JSON de la reponse HTTP :
//
//	{
//	  "phone":          string,
//	  "delivery_score": int,   // 0-100 (Laplace); 50 si inconnu (found=false)
//	  "found":          bool,
//	  "channel_scores": [
//	    { "channel": string, "score": int, "sample_size": int, "last_success_at": string|null }
//	  ]
//	}
//
// Semantique :
//   - Contact inconnu (aucun enregistrement) : 200 + found=false + delivery_score=50
//     (jamais de 404 — conform D26 : prior neutre).
//   - Contact connu : 200 + found=true + delivery_score=Laplace(success_count, failure_count)
//     depuis contact_intel.contacts.
//   - channel_scores trie par score DESC (colonne score de contact_intel.contact_channel_scores).
//
// loadContactScore (fonction ci-dessous) est le point d'acces IN-PROCESS au score
// contact, partage par ce handler et par l'enrichissement routing (highest_delivery)
// de messages_send.go, routing_select.go et routing_estimate.go — plus aucun appel
// HTTP interne (ecart B1 de la revue d'architecture Phase 2 souldé).

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"
)

// contactChannelScoreRow est le resultat d'une ligne SELECT depuis
// contact_intel.contact_channel_scores.
// Colonnes exactes (migration 0014) : phone, channel, score, sample_size, last_success_at.
type contactChannelScoreRow struct {
	Channel       string     `db:"channel"`
	Score         int        `db:"score"`
	SampleSize    int        `db:"sample_size"`
	LastSuccessAt *time.Time `db:"last_success_at"` // nullable
}

// contactRow est le resultat d'un SELECT depuis contact_intel.contacts.
// Colonnes utilisees (migrations 0008 + 0014) : success_count, failure_count,
// last_channel, last_success_at.
type contactRow struct {
	SuccessCount  int        `db:"success_count"`
	FailureCount  int        `db:"failure_count"`
	LastChannel   string     `db:"last_channel"`
	LastSuccessAt *time.Time `db:"last_success_at"` // nullable
}

// contactScoreChannelItem est un element de la liste channel_scores dans la reponse JSON.
type contactScoreChannelItem struct {
	Channel       string     `json:"channel"`
	Score         int        `json:"score"`
	SampleSize    int        `json:"sample_size"`
	LastSuccessAt *time.Time `json:"last_success_at"`
}

// getContactScoreResponse est la reponse JSON de GET /contacts/{phone}/score.
type getContactScoreResponse struct {
	Phone         string                    `json:"phone"`
	DeliveryScore int                       `json:"delivery_score"`
	Found         bool                      `json:"found"`
	ChannelScores []contactScoreChannelItem `json:"channel_scores"`
}

// HandleGetContactScore traite GET /contacts/{phone}/score.
//
// Pipeline :
//  1. Extrait {phone} du PathValue.
//  2. SELECT depuis contact_intel.contact_channel_scores ORDER BY score DESC.
//  3. Delegue a loadContactScore (success_count, failure_count, Laplace) pour le
//     score global.
//  4. Si contact inconnu ET aucun channel score : 200 found=false delivery_score=50.
//  5. Sinon : delivery_score = Laplace(success_count, failure_count) + found=true.
//  6. 200 + JSON.
func (s *Service) HandleGetContactScore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	phone := r.PathValue("phone")
	if phone == "" {
		writeError(w, http.StatusBadRequest, "phone requis dans le chemin")
		return
	}

	// 2. Lecture des scores par canal (triés par score DESC, conforme au contrat D26/T-012).
	var channelRows []contactChannelScoreRow
	if err := s.DB.Select(ctx, &channelRows,
		`SELECT channel, score, sample_size, last_success_at
		   FROM contact_intel.contact_channel_scores
		  WHERE phone = $1
		  ORDER BY score DESC`,
		phone,
	); err != nil {
		s.Logger.Error("contacts/score: select channel_scores", "phone", phone, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 3. Score global (delegue a loadContactScore — meme lecture DB que les
	// points d'enrichissement in-process de messages_send.go/routing_select.go).
	deliveryScore, contactFound, err := s.loadContactScore(ctx, phone)
	if err != nil {
		s.Logger.Error("contacts/score: loadContactScore", "phone", phone, "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}

	// 4. Contact inconnu ET aucun score par canal : reponse degradee (prior neutre).
	if !contactFound && len(channelRows) == 0 {
		writeJSON(w, http.StatusOK, getContactScoreResponse{
			Phone:         phone,
			DeliveryScore: 50, // prior neutre Laplace(0,0)
			Found:         false,
			ChannelScores: []contactScoreChannelItem{},
		})
		return
	}

	// Construire la liste channel_scores (deja triee par score DESC par le SELECT).
	items := make([]contactScoreChannelItem, len(channelRows))
	for i, row := range channelRows {
		items[i] = contactScoreChannelItem{
			Channel:       row.Channel,
			Score:         row.Score,
			SampleSize:    row.SampleSize,
			LastSuccessAt: row.LastSuccessAt,
		}
	}

	// 6. Reponse 200.
	// Note : found=true est retourne des lors que contactFound OU des channel_scores
	// existent (cas de migration ou seul le detail par canal est connu) — deliveryScore
	// reste le prior neutre 50 dans ce cas de figure (voir loadContactScore).
	writeJSON(w, http.StatusOK, getContactScoreResponse{
		Phone:         phone,
		DeliveryScore: deliveryScore,
		Found:         true,
		ChannelScores: items,
	})
}

// loadContactScore lit le score de joignabilite Laplace d'un contact depuis
// contact_intel.contacts, IN-PROCESS (meme binaire, meme connexion DB — aucun
// appel HTTP). Utilisee par HandleGetContactScore ainsi que par les points
// d'enrichissement routing (HandleSendMessage, HandleRoutingSelect,
// HandleRoutingEstimate).
//
// Retour :
//   - score : Laplace(success_count, failure_count) si le contact est connu ;
//     50 (prior neutre) si le contact est inconnu (sql.ErrNoRows).
//   - found : true si un enregistrement existe dans contact_intel.contacts.
//   - err   : non-nil UNIQUEMENT en cas d'echec DB reel (hors "contact inconnu",
//     qui est un cas nominal, jamais une erreur).
//
// Fallback gracieux (D28) : les appelants DOIVENT traiter err != nil comme
// "score indisponible" (ne pas enrichir, ne jamais faire echouer la requete).
func (s *Service) loadContactScore(ctx context.Context, phone string) (score int, found bool, err error) {
	var contact contactRow
	getErr := s.DB.Get(ctx, &contact,
		`SELECT success_count, failure_count, last_channel, last_success_at
		   FROM contact_intel.contacts
		  WHERE phone = $1`,
		phone,
	)
	if getErr != nil {
		if errors.Is(getErr, sql.ErrNoRows) {
			score, found = deriveContactScore(false, 0, 0)
			return score, found, nil
		}
		return 50, false, getErr
	}
	score, found = deriveContactScore(true, contact.SuccessCount, contact.FailureCount)
	return score, found, nil
}

// deriveContactScore est la fonction pure (sans I/O, testable isolement) qui
// derive (delivery_score, found) a partir du resultat de la lecture DB.
// Contact inconnu (contactFound=false) → prior neutre 50, found=false.
// Contact connu → Laplace(successCount, failureCount), found=true.
func deriveContactScore(contactFound bool, successCount, failureCount int) (score int, found bool) {
	if !contactFound {
		return 50, false
	}
	return Laplace(successCount, failureCount), true
}
