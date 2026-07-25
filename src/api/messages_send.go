package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// sendMessageRequest est le corps JSON de POST /messages.
type sendMessageRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Recipient   string `json:"recipient"`
	Body        string `json:"body"`
	Strategy    string `json:"strategy"`
}

// sendMessageResponse est la reponse JSON de POST /messages (201 Created).
type sendMessageResponse struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Cost       int64  `json:"cost"`
	ProviderID string `json:"provider_id"`
}

// providerScoreRow est le resultat d'une ligne SELECT depuis routing.provider_scores.
// La table n'a pas de colonne "cost" ni "enabled" (migration 0004).
type providerScoreRow struct {
	Provider string `db:"provider"`
	Channel  string `db:"channel"`
	Score    int    `db:"score"`
}

// HandleSendMessage traite POST /messages.
//
// Pipeline :
//  1. Decode + validation du corps JSON.
//  2. Lecture des provider scores depuis routing.provider_scores.
//  3. Si strategy=highest_delivery : enrichissement du score via loadContactScore
//     (lecture DB in-process — fallback silencieux sur erreur DB, D28).
//  4. Selection du provider via SelectProvider.
//  5. Appel au provider (Send).
//  6. Persistance dans messaging.messages.
//  7. Publication AMQP "message.sent" (non-bloquant — echec = log warn seulement).
//  8. Reponse 201 + JSON.
func (s *Service) HandleSendMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Decode + validation.
	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corps de requete invalide")
		return
	}
	if req.WorkspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id est requis")
		return
	}
	if req.Recipient == "" {
		writeError(w, http.StatusBadRequest, "recipient est requis")
		return
	}
	if req.Body == "" {
		writeError(w, http.StatusBadRequest, "body est requis")
		return
	}
	if !isValidStrategy(req.Strategy) {
		writeError(w, http.StatusBadRequest, "strategy doit etre l'une de : lowest_cost, highest_delivery, round_robin")
		return
	}
	if _, err := parseUUID(req.WorkspaceID); err != nil {
		writeError(w, http.StatusBadRequest, "workspace_id : format UUID invalide")
		return
	}

	// 2. Lecture des provider scores.
	// Note : routing.provider_scores n'a pas de colonne "enabled" (migration 0004).
	// Tous les providers presents en base sont consideres actifs.
	var rows []providerScoreRow
	if err := s.DB.Select(ctx, &rows,
		`SELECT provider, channel, score FROM routing.provider_scores ORDER BY score DESC`,
	); err != nil {
		s.Logger.Error("messages: select provider_scores", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur interne")
		return
	}
	if len(rows) == 0 {
		// 503 : le service est operationnel mais aucun provider n'est configure.
		writeError(w, http.StatusServiceUnavailable, "aucun provider disponible")
		return
	}

	// Convertir les lignes en []ProviderScore pour les fonctions pures.
	scores := make([]ProviderScore, len(rows))
	for i, r := range rows {
		scores[i] = ProviderScore{
			ProviderID: r.Provider,
			Channel:    r.Channel,
			Score:      r.Score,
			Cost:       0, // cost absent de routing.provider_scores (voir migration 0004)
		}
	}

	// 3. Enrichissement du score contact pour la strategie highest_delivery.
	// loadContactScore lit contact_intel.contacts IN-PROCESS (meme binaire/DB) ;
	// aucun appel HTTP interne (voir contacts_get_score.go).
	if req.Strategy == "highest_delivery" && req.Recipient != "" {
		contactScore, _, err := s.loadContactScore(ctx, req.Recipient)
		if err != nil {
			// Fallback gracieux (D28) : le routage ne doit jamais echouer a cause
			// du score contact — enrichScoresWithContact laisse les scores inchanges.
			s.Logger.Warn("messages: contact score indisponible (fallback)", "err", err)
		}
		scores = enrichScoresWithContact(scores, contactScore, err)
	}

	// 4. Selection du provider.
	providerID, err := SelectProvider(scores, req.Strategy)
	if err != nil {
		s.Logger.Error("messages: select provider", "err", err)
		writeError(w, http.StatusServiceUnavailable, "aucun provider disponible")
		return
	}

	// 5. Appel au provider.
	prov, ok := s.Providers[providerID]
	if !ok {
		s.Logger.Error("messages: provider non configure", "provider_id", providerID)
		writeError(w, http.StatusInternalServerError, "provider non configure")
		return
	}

	// Determiner le canal depuis le score selectionne.
	var channel string
	for _, sc := range scores {
		if sc.ProviderID == providerID {
			channel = sc.Channel
			break
		}
	}

	result, err := prov.Send(ctx, req.Recipient, req.Body)
	if err != nil {
		s.Logger.Error("messages: provider.Send", "provider_id", providerID, "err", err)
		// On persiste le message en etat "failed" avant de repondre.
		msgID := newUUID()
		_ = s.insertMessage(ctx, msgID, req.WorkspaceID, req.Recipient, req.Body, providerID, channel, "failed", "", 0)
		writeError(w, http.StatusInternalServerError, "echec d'envoi via le provider")
		return
	}

	// Mapper le status du ProviderResult vers les statuts de la table messaging.messages.
	// Valeurs possibles de ProviderResult.Status : "sent", "failed", "rejected".
	status := mapProviderStatus(result.Status)

	// 6. Persistance dans messaging.messages.
	msgID := newUUID()
	if err := s.insertMessage(ctx, msgID, req.WorkspaceID, req.Recipient, req.Body, providerID, channel, status, result.ExternalID, result.Cost); err != nil {
		s.Logger.Error("messages: insert", "err", err)
		writeError(w, http.StatusInternalServerError, "erreur de persistance")
		return
	}

	// 7. Publication AMQP (non-bloquant).
	s.publishMessageSent(ctx, msgID, req.WorkspaceID, req.Recipient, providerID, status)

	// 8. Reponse 201.
	writeJSON(w, http.StatusCreated, sendMessageResponse{
		ID:         msgID,
		Status:     status,
		Cost:       result.Cost,
		ProviderID: providerID,
	})
}

// insertMessage insere un message dans messaging.messages.
// Colonnes issues de la migration 0003 : id, workspace_id, recipient, content,
// status, channel, created_at.
// Colonnes issues de la migration additive 0018_messaging_dlr_correlation :
// provider_id, external_id, cost — necessaires a la correlation des DLR
// (voir webhooks_telegram.go) et a la reponse API (sendMessageResponse).
//
// externalID et cost peuvent legitimement etre vides/nuls (provider qui n'a
// rien retourne, ou envoi en echec avant reponse provider) : on insere alors
// NULL plutot que la chaine vide / 0, pour ne pas polluer les index partiels
// crees par la migration 0018 (WHERE external_id IS NOT NULL).
func (s *Service) insertMessage(
	ctx context.Context,
	id, workspaceID, recipient, content, providerID, channel, status, externalID string,
	cost int64,
) error {
	var extID sql.NullString
	if externalID != "" {
		extID = sql.NullString{String: externalID, Valid: true}
	}
	var costCents sql.NullInt64
	if cost != 0 {
		costCents = sql.NullInt64{Int64: cost, Valid: true}
	}
	return s.DB.Exec(ctx,
		`INSERT INTO messaging.messages
		   (id, workspace_id, recipient, content, status, channel, provider_id, external_id, cost, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		id, workspaceID, recipient, content, status, channel, providerID, extID, costCents, time.Now().UTC(),
	)
}

// mapProviderStatus convertit le statut retourne par un provider vers les
// valeurs reconnues par la table messaging.messages.
func mapProviderStatus(providerStatus string) string {
	switch providerStatus {
	case "sent", "delivered":
		return string(MessageSent)
	case "rejected":
		return string(MessageRejected)
	case "failed":
		return string(MessageFailed)
	default:
		return string(MessagePending)
	}
}

// publishMessageSent publie un evenement AMQP "message.sent".
// Si AMQP est nil ou si la publication echoue, on log un warning et on continue.
// Le message est deja persiste — l'echec AMQP ne doit pas faire echouer la requete.
func (s *Service) publishMessageSent(
	ctx context.Context,
	messageID, workspaceID, recipient, providerID, status string,
) {
	if s.AMQP == nil {
		return
	}

	event := struct {
		MessageID   string `json:"message_id"`
		WorkspaceID string `json:"workspace_id"`
		Recipient   string `json:"recipient"`
		ProviderID  string `json:"provider_id"`
		Status      string `json:"status"`
	}{
		MessageID:   messageID,
		WorkspaceID: workspaceID,
		Recipient:   recipient,
		ProviderID:  providerID,
		Status:      status,
	}

	body, err := json.Marshal(event)
	if err != nil {
		s.Logger.Warn("messages: marshal amqp event", "err", err)
		return
	}

	if err := s.AMQP.Publish(ctx, "fleece", "message.sent", body); err != nil {
		s.Logger.Warn("messages: amqp publish", "err", err)
	}
}
