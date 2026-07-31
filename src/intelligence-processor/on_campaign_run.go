package intelligenceprocessor

// on_campaign_run.go — handler de l'evenement AMQP "campaign.run" (routing
// key "campaign.run").
//
// CONTRAT FIGE PAR CE WORKER (le plan de migration ne l'imposait pas — voir
// campaign_scheduler.go pour le cote producteur, meme contrat documente des
// deux cotes) :
//
//	{"campaign_id": "<uuid campaign.campaigns.id>", "run_id": "<bigint campaign.campaign_runs.id, encode en decimal string>"}
//
// Volontairement MINIMAL (2 champs) : comme pour message.sent/delivered/failed
// (§Autonomie, consumer.go), ce handler ne fait JAMAIS confiance au payload
// pour les donnees d'agregation — workspace_id/message_body/channel_strategy/
// rate_limit sont TOUJOURS relus depuis campaign.campaigns par campaign_id.
//
// ═══════════════════════════════════════════════════════════════════════════
// E6 (ECART, IMPACT MONETAIRE, Phase 3) — RESERVATION AVANT ENVOI
// ═══════════════════════════════════════════════════════════════════════════
//
// AVANT ce correctif, la sequence etait POST /messages PUIS markRecipientSent
// (garde "WHERE status='pending'"). Si le marquage echouait techniquement, ou
// si le worker mourait entre les deux, l'evenement campaign.run etait requeue
// et le destinataire — TOUJOURS 'pending' puisque jamais marque 'sent' — etait
// RE-SOLLICITE au prochain traitement du meme evenement (ou d'un evenement
// campaign.run republie) : message duplique chez l'utilisateur final ET cout
// debite deux fois (chaque POST /messages declenche un debit wallet reel, cf.
// src/api/wallet_deduct.go).
//
// CORRECTIF — reserver AVANT d'envoyer (chaque etape DANS SA PROPRE
// transaction courte, jamais de transaction longue autour de l'appel HTTP) :
//  1. reserveRecipientForSending : UPDATE ... SET status='sending'
//     WHERE id=$1 AND status='pending' — reservation ATOMIQUE. 0 ligne
//     affectee => deja pris en charge (redelivery) => le destinataire est
//     saute (pas de second POST).
//  2. POST /messages (inchange).
//  3. Succes -> markRecipientSent transitionne 'sending' -> 'sent' (la garde
//     change de 'pending' a 'sending' : c'est desormais l'etat REEL du
//     destinataire a ce point du pipeline) + incremente sent_count.
//  4. Echec (POST ou corps invalide) -> revertRecipientToPending retransitionne
//     'sending' -> 'pending' : DECISION PM tranchee ICI (documentee) : le
//     retry reste possible par republication de campaign.run (comportement
//     historique de ce fichier, conserve), PLUTOT qu'un statut 'failed'
//     terminal — un echec HTTP isole (timeout, 5xx transitoire) ne doit pas
//     necessairement etre definitif pour CE destinataire.
//
// LIMITE RESIDUELLE ASSUMEE (aucune migration, pas de table de verrou
// dediee) : deux scenarios etroits laissent un destinataire bloque en
// 'sending' indefiniment (plus jamais selectionne par l'etape 3 du pipeline,
// WHERE status='pending', meme apres republication de campaign.run) :
//   - le worker meurt EXACTEMENT entre l'etape 1 (reservation) et l'etape 4
//     (revert), sans jamais reussir ni l'un ni l'autre (crash process, jamais
//     de retry du revert) ;
//   - le POST /messages REUSSIT (message reellement envoye et facture) mais
//     markRecipientSent echoue techniquement (panne DB) : le message est
//     bel et bien parti UNE SEULE FOIS (pas de double envoi, l'essentiel du
//     correctif E6 tient), mais campaign_recipients.status ne bascule jamais
//     vers 'sent' — desynchronisation de suivi, PAS de double facturation.
//
// Ces deux residus sont ETROITS (fenetre = la duree d'un appel HTTP + son
// eventuel revert/marquage, pas la duree du run entier) et analogues a
// d'autres limites deja documentees et acceptees ailleurs dans ce depot (ex.
// wallet introuvable dans src/core-processor/on_message_failed.go) — PAS
// resolus ici (necessiterait un scan periodique des lignes 'sending' trop
// anciennes, hors perimetre). A tracer par le PM comme dette si juge
// necessaire. Compare AU BUG D'ORIGINE (E6) : le pire residu possible ici est
// une INCOHERENCE DE STATUT/SUIVI, jamais un double envoi ni un double debit
// wallet — c'est precisement l'objectif du correctif.
//
// Pipeline :
//  1. Parse + validation minimale (campaign_id UUID non vide, run_id numerique).
//  2. Lecture AUTONOME de campaign.campaigns (workspace_id, message_body,
//     channel_strategy, rate_limit).
//  3. Chargement des destinataires 'pending' de campaign.campaign_recipients
//     (colonne status, migration 0016 — c'est ce qui permet la reprise d'un
//     run interrompu : un destinataire deja 'sending'/'sent'/'delivered'/'failed'
//     n'est PLUS selectionne).
//  4. Pour chaque destinataire, DANS L'ORDRE, UN A LA FOIS (jamais de batch
//     global — un crash en milieu de run ne doit jamais re-tirer un message
//     deja parti) :
//     a. rate limiter (token bucket sur campaign.campaigns.rate_limit,
//        ratelimit.go).
//     b. reserveRecipientForSending (E6) : reservation atomique 'pending' ->
//        'sending'. 0 ligne affectee -> deja pris en charge, on passe au
//        destinataire suivant SANS second POST.
//     c. POST {API_URL}/messages (src/api) — AUCUN import Go de
//        fleece/src/api (regle transverse n°5), c'est un appel reseau
//        net/http, timeout explicite (10s par defaut).
//     d. Succes (2xx) -> marque le destinataire 'sent' (depuis 'sending') +
//        message_id ET incremente sent_count sur le run, DANS UNE MEME
//        transaction courte (markRecipientSent).
//     e. Echec (4xx/5xx/timeout/corps invalide) -> log WARN, revertRecipientToPending
//        retransitionne 'sending' -> 'pending' (reprise possible par
//        republication de campaign.run, E6 point 4), on continue au
//        destinataire suivant (un echec isole ne doit jamais bloquer tout le run).
//  5. Annulation de contexte (arret du worker, SIGINT/SIGTERM) : la boucle
//     s'arrete PROPREMENT des le prochain point de controle et retourne
//     ctx.Err() — une erreur "nue" (voir consumer.go/isPermanentError),
//     donc REQUEUE : au redemarrage du worker, la redelivery de CE MEME
//     campaign.run reprend exactement ou l'on s'est arrete, grace au filtre
//     status='pending' de l'etape 3 (aucun destinataire deja 'sending'/'sent'
//     n'est jamais re-tire).
//
// Corps aligne sur src/api/messages_send.go (sendMessageRequest/
// sendMessageResponse) — VERIFIE dans le code source avant d'ecrire ce
// fichier (memes noms JSON, aucun import Go).
import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// campaignRunEvent est le contrat FIGE par ce worker pour la routing key
// "campaign.run" (voir doc de tete de fichier et campaign_scheduler.go, cote
// producteur).
type campaignRunEvent struct {
	CampaignID string `json:"campaign_id"`
	RunID      string `json:"run_id"`
}

// parseCampaignRunEvent decode le corps JSON d'une delivery "campaign.run".
// Fonction pure, testable isolement.
func parseCampaignRunEvent(body []byte) (campaignRunEvent, error) {
	var evt campaignRunEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return campaignRunEvent{}, fmt.Errorf("parseCampaignRunEvent: json invalide: %w", err)
	}
	return evt, nil
}

// campaignDetailsRow est le resultat de la lecture autonome de
// campaign.campaigns (colonnes issues de la migration 0016).
type campaignDetailsRow struct {
	WorkspaceID     string `db:"workspace_id"`
	MessageBody     string `db:"message_body"`
	ChannelStrategy string `db:"channel_strategy"`
	RateLimit       int    `db:"rate_limit"`
}

// pendingRecipientRow est une ligne de campaign.campaign_recipients dont le
// statut vaut encore 'pending'.
type pendingRecipientRow struct {
	ID        int64  `db:"id"`
	Recipient string `db:"recipient"`
}

// sendMessageRequest/sendMessageResponse sont ALIGNES sur
// src/api/messages_send.go (memes types, memes noms JSON) — AUCUN import Go
// de fleece/src/api, seule la FORME du contrat HTTP est partagee.
type sendMessageRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Recipient   string `json:"recipient"`
	Body        string `json:"body"`
	Strategy    string `json:"strategy"`
}

type sendMessageResponse struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Cost       int64  `json:"cost"`
	ProviderID string `json:"provider_id"`
}

// handleCampaignRun traite l'evenement "campaign.run".
func handleCampaignRun(ctx context.Context, s *Service, body []byte) error {
	evt, err := parseCampaignRunEvent(body)
	if err != nil {
		return newPermanentError(err)
	}
	if evt.CampaignID == "" {
		return newPermanentError(errors.New("campaign.run: campaign_id vide"))
	}
	if !uuidLike.MatchString(evt.CampaignID) {
		return newPermanentError(fmt.Errorf("campaign.run: campaign_id syntaxiquement invalide (pas un UUID): %q", evt.CampaignID))
	}
	if evt.RunID == "" {
		return newPermanentError(errors.New("campaign.run: run_id vide"))
	}
	runID, err := strconv.ParseInt(evt.RunID, 10, 64)
	if err != nil {
		return newPermanentError(fmt.Errorf("campaign.run: run_id non numerique: %q", evt.RunID))
	}

	// 2. Lecture autonome de la campagne (jamais depuis le payload).
	var campaign campaignDetailsRow
	err = s.DB.Get(ctx, &campaign,
		`SELECT workspace_id, message_body, channel_strategy, rate_limit
		   FROM campaign.campaigns WHERE id = $1`,
		evt.CampaignID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		s.Logger.Warn("intelligence-processor: campaign.run ignore, campagne introuvable",
			"campaign_id", evt.CampaignID, "run_id", runID)
		return newPermanentError(fmt.Errorf("campaign.run: campagne %s introuvable", evt.CampaignID))
	}
	if err != nil {
		return err // transitoire -> requeue
	}

	// 3. Destinataires 'pending' (reprise de run : cf. doc de tete de fichier).
	var recipients []pendingRecipientRow
	if err := s.DB.Select(ctx, &recipients,
		`SELECT id, recipient FROM campaign.campaign_recipients
		   WHERE campaign_id = $1 AND status = 'pending' ORDER BY id`,
		evt.CampaignID,
	); err != nil {
		return err // transitoire -> requeue
	}

	limiter := newRateLimiter(campaign.RateLimit)
	client := s.httpClient()

	sentCount := 0
	for _, recipient := range recipients {
		// 5. Point de controle d'annulation, avant CHAQUE destinataire.
		select {
		case <-ctx.Done():
			s.Logger.Warn("intelligence-processor: campaign.run interrompu (arret worker), reprise ulterieure",
				"campaign_id", evt.CampaignID, "run_id", runID, "sent_so_far", sentCount, "restants", len(recipients)-sentCount)
			return ctx.Err() // erreur "nue" -> requeue
		default:
		}

		if err := limiter.Wait(ctx); err != nil {
			return err // annulation pendant l'attente du rate limiter -> requeue
		}

		// E6, point b : reservation atomique AVANT tout envoi HTTP. 0 ligne
		// affectee => deja pris en charge par un traitement precedent
		// (redelivery de campaign.run) => on saute ce destinataire SANS
		// second POST (c'est precisement ce qui empeche le double envoi).
		reserved, err := s.reserveRecipientForSending(ctx, recipient.ID)
		if err != nil {
			s.Logger.Error("intelligence-processor: reservation 'sending' echouee",
				"campaign_id", evt.CampaignID, "recipient_id", recipient.ID, "err", err)
			return err // transitoire -> requeue
		}
		if !reserved {
			s.Logger.Info("intelligence-processor: destinataire deja pris en charge (redelivery), saute sans second envoi",
				"campaign_id", evt.CampaignID, "recipient_id", recipient.ID)
			continue
		}

		msgID, sendErr := s.sendCampaignMessage(ctx, client, campaign, recipient.Recipient)
		if sendErr != nil {
			s.Logger.Warn("intelligence-processor: envoi campagne echoue, destinataire repasse 'pending' (retry possible, E6)",
				"campaign_id", evt.CampaignID, "run_id", runID, "recipient", recipient.Recipient, "err", sendErr)
			if revertErr := s.revertRecipientToPending(ctx, recipient.ID); revertErr != nil {
				s.Logger.Error("intelligence-processor: repli 'sending' -> 'pending' echoue (destinataire potentiellement bloque en 'sending', voir limite residuelle E6)",
					"campaign_id", evt.CampaignID, "recipient_id", recipient.ID, "err", revertErr)
				return revertErr // transitoire -> requeue
			}
			continue
		}

		if err := s.markRecipientSent(ctx, recipient.ID, msgID, runID); err != nil {
			s.Logger.Error("intelligence-processor: marquage 'sent' echoue",
				"campaign_id", evt.CampaignID, "recipient_id", recipient.ID, "err", err)
			return err // transitoire -> requeue (redelivery reprendra a ce destinataire : deja 'sending', pas de second POST, voir reserveRecipientForSending)
		}
		sentCount++
	}

	s.Logger.Info("intelligence-processor: campaign.run traite",
		"campaign_id", evt.CampaignID, "run_id", runID, "destinataires", len(recipients), "envoyes", sentCount)
	return nil
}

// httpClient retourne s.HTTPClient si injecte (tests, httptest.Server), sinon
// un client par defaut avec un timeout explicite de 10s.
func (s *Service) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// sendCampaignMessage appelle POST {API_URL}/messages pour un destinataire.
// Retourne l'id du message cree (sendMessageResponse.ID) en cas de succes
// (statut 2xx + corps decodable + id non vide).
func (s *Service) sendCampaignMessage(ctx context.Context, client *http.Client, campaign campaignDetailsRow, recipient string) (string, error) {
	reqBody, err := json.Marshal(sendMessageRequest{
		WorkspaceID: campaign.WorkspaceID,
		Recipient:   recipient,
		Body:        campaign.MessageBody,
		Strategy:    campaign.ChannelStrategy,
	})
	if err != nil {
		return "", fmt.Errorf("sendCampaignMessage: encodage requete: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.APIURL+"/messages", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("sendCampaignMessage: construction requete: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sendCampaignMessage: requete http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("sendCampaignMessage: statut http inattendu: %d", resp.StatusCode)
	}

	var out sendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("sendCampaignMessage: corps de reponse invalide: %w", err)
	}
	if out.ID == "" {
		return "", errors.New("sendCampaignMessage: reponse sans id de message")
	}
	return out.ID, nil
}

// reserveRecipientForSending (E6, point b) reserve ATOMIQUEMENT un
// destinataire avant tout envoi HTTP : UPDATE ... SET status = 'sending'
// WHERE id = $1 AND status = 'pending'. Retourne (true, nil) si la ligne a
// ete reservee par CET appel (0 ligne precedente en 'sending'/'sent'/etc.),
// (false, nil) si 0 ligne affectee (deja pris en charge par un traitement
// precedent — redelivery de campaign.run — donc PAS de second POST), ou une
// erreur technique (transitoire -> requeue).
func (s *Service) reserveRecipientForSending(ctx context.Context, recipientID int64) (bool, error) {
	rows, err := s.DB.ExecRows(ctx,
		`UPDATE campaign.campaign_recipients SET status = 'sending' WHERE id = $1 AND status = 'pending'`,
		recipientID,
	)
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// revertRecipientToPending (E6, point 4) retransitionne un destinataire de
// 'sending' vers 'pending' apres un echec d'envoi HTTP isole (timeout,
// 4xx/5xx, corps invalide) — DECISION PM documentee en tete de fichier :
// retry possible par republication de campaign.run, plutot qu'un statut
// 'failed' terminal. Garde "AND status = 'sending'" : idempotent par
// construction (ne retransitionne jamais une ligne deja 'sent'/'pending' par
// un autre chemin).
//
// CORRECTIF D-M31 — CONTEXTE DETACHE. Avant ce correctif, ce revert recevait le
// MEME ctx que le POST /messages qui venait d'echouer. Or la cause d'echec la
// plus courante en exploitation n'est pas un 5xx du provider : c'est un
// SIGTERM, c'est-a-dire l'arret gracieux NORMAL d'un rolling deploy. Le ctx est
// alors deja annule quand on arrive ici, l'UPDATE echoue immediatement en
// context.Canceled, et la ligne reste PIEGEE en 'sending' — la reprise de
// campagne filtre sur status='pending' et ne la reprend donc JAMAIS. Un
// destinataire perdu a chaque deploiement, sans aucune trace.
//
// Ce revert persiste un fait DEJA ACQUIS (l'envoi a echoue, on le sait) : il
// doit survivre a l'annulation, exactement comme les persistances du volet
// webhook sortant de core-processor (B1/E1, webhook_persist_context.go) — meme
// raisonnement, meme borne de securite courte pour ne jamais retarder un arret
// gracieux.
//
// LIMITE ASSUMEE : cela ferme la fenetre SIGTERM, PAS celle d'un crash brutal
// (SIGKILL, OOM) survenant entre la reservation et le revert. Un balayage
// periodique des 'sending' anciens reste necessaire pour ce cas — a fusionner
// avec D-M30.
func (s *Service) revertRecipientToPending(ctx context.Context, recipientID int64) error {
	ctx, cancel := detachedRevertContext(ctx)
	defer cancel()

	return s.DB.Exec(ctx,
		`UPDATE campaign.campaign_recipients SET status = 'pending' WHERE id = $1 AND status = 'sending'`,
		recipientID,
	)
}

// markRecipientSent transitionne UN destinataire de 'sending' (reserve par
// reserveRecipientForSending, E6) vers 'sent' + incremente sent_count sur le
// run, dans une transaction courte (les deux ecritures doivent etre
// atomiques : un crash entre les deux laisserait sinon sent_count
// desynchronise du nombre reel de destinataires 'sent'). Garde
// "AND status = 'sending'" (E6 : la reservation atomique a deja transitionne
// le destinataire hors de 'pending' AVANT l'appel HTTP — c'est desormais le
// SEUL etat depuis lequel cette fonction doit accepter de transitionner vers
// 'sent') : idempotence naturelle — si le destinataire a deja ete marque
// 'sent' par un traitement precedent, 0 ligne affectee, pas de double
// increment de sent_count.
func (s *Service) markRecipientSent(ctx context.Context, recipientID int64, messageID string, runID int64) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var touched struct {
		ID int64 `db:"id"`
	}
	err = tx.Get(ctx, &touched,
		`UPDATE campaign.campaign_recipients SET status = 'sent', message_id = $1
		   WHERE id = $2 AND status = 'sending'
		 RETURNING id`,
		messageID, recipientID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// Deja marque 'sent' par un traitement precedent (redelivery) : rien
		// a incrementer, commit propre (rien a annuler).
		return tx.Commit()
	}
	if err != nil {
		return err
	}

	if err := tx.Exec(ctx,
		`UPDATE campaign.campaign_runs SET sent_count = sent_count + 1 WHERE id = $1`,
		runID,
	); err != nil {
		return err
	}

	return tx.Commit()
}
