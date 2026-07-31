package coreprocessor

// webhook_dispatch.go — fan-out des webhooks SORTANTS a la reception d'un
// evenement "message.delivered"/"message.failed" (D-M43, T-005 volet
// livraison).
//
// APPELE PAR on_message_delivered.go / on_message_failed.go, UNIQUEMENT :
//
//  1. APRES le commit de la transition de statut (jamais depuis l'interieur
//     d'une transaction SQL — un appel HTTP externe tenant des verrous
//     Postgres pendant jusqu'a 10s serait inacceptable, cf. doc de tete de
//     ces deux fichiers).
//  2. UNIQUEMENT si la transition a REELLEMENT eu lieu (RETURNING a renvoye
//     une ligne, workspace_id connu) — exactement la meme garde que le
//     remboursement wallet (on_message_failed.go). Telegram rejoue ses
//     updates jusqu'a 24h (D-M25) : sans cette garde, chaque rejeu enverrait
//     un webhook EN DOUBLE au client final. C'est la garde d'idempotence de
//     bout en bout de ce volet — voir webhook_dispatch_test.go,
//     TestHandleMessageDelivered_rows0_neverFansOutWebhook /
//     TestHandleMessageFailed_rows0_neverFansOutWebhook (non-regression
//     B1/D-M26).
//
// VOCABULAIRE D'EVENEMENTS (D-M43) : les webhook_endpoints s'abonnent via la
// colonne text[] "events" a des noms d'evenements alignes sur les routing
// keys AMQP deja publiees par src/api (D-M21) : "message.delivered" et
// "message.failed" (ce worker ne consomme QUE ces deux routing keys — voir
// consumer.go, BoundRoutingKeys() INCHANGEE par cette tache, aucun nouveau
// binding). Confirme par src/platform-app/app/(dashboard)/dashboard/webhooks/
// page.tsx (AVAILABLE_EVENTS, commentaire "API-03 / TDD §4.6") qui propose au
// dashboard 7 evenements — CE WORKER NE FAIT LE FAN-OUT QUE POUR CES DEUX-LA
// (les seuls qu'il consomme). E3 (revue D-M43) : src/api valide desormais a
// la creation d'un webhook_endpoint que "events" ne contient QUE des noms
// reellement livrables (voir src/api/webhook_endpoints_create.go,
// validateWebhookEvents) — un client ne peut plus s'abonner silencieusement
// a un evenement qui ne sera jamais livre.
//
// ECHEC DE FAN-OUT = JAMAIS UNE ERREUR DE TRAITEMENT DU STATUT (regle 4) :
// toutes les fonctions de ce fichier appelees depuis on_message_delivered.go/
// on_message_failed.go sont void (aucune erreur retournee) — un echec HTTP ou
// SQL ici est loggue (WARN/ERROR) et, pour l'echec HTTP, persiste en base pour
// le scheduler de retry (webhook_retry_scheduler.go), mais ne remonte JAMAIS
// au consumer (consumer.go) : le message AMQP reste Ack, jamais requeue a
// cause d'un webhook client indisponible.
//
// B2 (BLOQUANT, revue architecture Phase 3) — ORDRE INSERT-AVANT-POST :
// AVANT ce correctif, dispatchToNewEndpoint ne persistait la delivery
// (INSERT) qu'APRES le POST HTTP, sur le MEME ctx (celui du handler AMQP,
// consumer.go). Au SIGTERM en plein handleMessageDelivered/handleMessageFailed,
// le POST ET l'INSERT etaient tous deux annules -> AUCUNE ligne inseree dans
// webhook_deliveries alors que messaging.messages.status etait deja committe
// et le message AMQP Ack (le handler retourne nil AVANT ce fan-out void) :
// le webhook client n'etait alors JAMAIS envoye, JAMAIS retente, et AUCUNE
// trace n'en subsistait (aucune ligne pour le scheduler de retry, aucune
// entree dashboard). CORRECTIF : dispatchToNewEndpoint insere desormais une
// ligne 'pending' AVANT le POST (insertPendingWebhookDelivery), avec
// next_retry_at deja positionne au bail de reservation (webhookRetryClaimBail,
// voir webhook_retry_scheduler.go) — toute ligne orpheline (ctx annule avant
// le POST, pendant le POST, ou avant la finalisation) est alors NATURELLEMENT
// rattrapee par le scheduler de retry, sans aucune intervention. Le resultat
// (succes/echec) est ensuite persiste par UNE UPDATE (finalizeNewWebhookDelivery),
// sur un contexte DETACHE et borne (voir detachedPersistContext,
// webhook_persist_context.go) — meme motif que finalizeWebhookRetry (B1) :
// un resultat DEJA ACQUIS (le POST a eu lieu) ne doit jamais etre perdu par
// une annulation de ctx survenue APRES le POST.
//
// E1 (2e revue architecture, fenetre residuelle de B2) — CE CORRECTIF NE
// COUVRAIT PAS ENCORE l'instant AVANT insertPendingWebhookDelivery : jusqu'a
// E1, loadSubscribedWebhookEndpoints et insertPendingWebhookDelivery
// s'executaient sur le ctx annulable du handler AMQP. Un SIGTERM survenant
// entre le commit de la transition de statut et l'une de ces deux requetes
// laissait ni ligne en base ni POST envoye : exactement le symptome B2,
// dans une fenetre etroite (~1ms de SQL au lieu de ~10s de POST) mais non
// nulle, et sans aucun rattrapage automatique. CORRECTIF E1 : ces deux
// requetes s'executent desormais elles aussi sur un contexte DETACHE et
// borne (voir detachedPersistContext) — SEUL le POST HTTP (postWebhook) reste
// sur le ctx annulable. Comportement resultant au SIGTERM : la ligne
// 'pending' est inseree -> le POST echoue en context.Canceled -> la
// finalisation (deja detachee) ecrit 'failed' + next_retry_at au backoff ->
// rattrapage normal par le scheduler de retry.
import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	syncx "fleece/src/go/syncx"
)

// Statuts de webhook.webhook_deliveries.status (portage de
// src/webhook/internal/domain/status.go — service a supprimer par M-023).
// "pending" est desormais PRODUIT par dispatchToNewEndpoint (B2, voir doc de
// tete de fichier) : AVANT ce correctif, aucun chemin de production ne
// produisait 'pending' (la premiere tentative etait entierement synchrone,
// INSERT-apres-POST) — c'est desormais le statut initial de toute livraison,
// remplace par 'delivered'/'failed' des que le resultat du POST est connu. Le
// scheduler de retry (webhook_retry_scheduler.go) traite 'pending' exactement
// comme 'failed' a l'endpoint de selection (une ligne 'pending' orpheline,
// c'est-a-dire jamais finalisee car le worker est mort entre l'INSERT et la
// finalisation, redevient eligible au retry apres le bail).
const (
	webhookStatusPending   = "pending"
	webhookStatusDelivered = "delivered"
	webhookStatusFailed    = "failed"
	webhookStatusExhausted = "exhausted"
)

// webhookDispatchTimeout borne toute tentative HTTP de livraison (premiere
// tentative ET retries) a 10s, conformement au dispatcher de reference
// (src/webhook/internal/adapters/dispatcher/http_dispatcher.go).
const webhookDispatchTimeout = 10 * time.Second

// webhookFanOutConcurrency borne le nombre de livraisons HTTP concurrentes
// pour UN meme evenement (plusieurs endpoints abonnes) via syncx.Map — un
// endpoint lent ou indisponible ne doit jamais retarder indefiniment les
// autres endpoints abonnes au meme evenement.
const webhookFanOutConcurrency = 5

// defaultWebhookHTTPClient construit le client HTTP par defaut (timeout 10s),
// utilise si Service.WebhookHTTPClient n'a pas ete injecte explicitement (voir
// Service.Init(), service.go).
func defaultWebhookHTTPClient() *http.Client {
	return &http.Client{Timeout: webhookDispatchTimeout}
}

// webhookEventPayload est le corps JSON POSTe aux endpoints webhook abonnes.
// Reconstruit a partir du SEUL messageEvent AMQP deja recu par ce worker (pas
// de nouvelle lecture de messaging.messages au-dela de workspace_id — voir
// on_message_delivered.go/on_message_failed.go, qui relisent workspace_id via
// la MEME requete RETURNING que la transition de statut, jamais une seconde
// requete, voir E2) : message_id/external_id/provider_id/source. Le champ
// Status N'EST PAS recopie tel quel depuis evt.Status (non fiable pour une
// donnee de decision, D-M21 — le payload AMQP est un enrichissement, jamais
// une source de verite) : il est deduit du NOM D'EVENEMENT passe par
// l'appelant (on_message_delivered.go / on_message_failed.go, donc du
// routing key AMQP effectivement reçu et du WHERE/RETURNING SQL qui vient de
// s'executer), voir webhookEventStatus.
type webhookEventPayload struct {
	Event      string `json:"event"`
	MessageID  string `json:"message_id"`
	ExternalID string `json:"external_id,omitempty"`
	ProviderID string `json:"provider_id,omitempty"`
	Status     string `json:"status"`
	Source     string `json:"source,omitempty"`
	OccurredAt string `json:"occurred_at"`
}

// webhookEventStatus deduit le statut public a partir du nom d'evenement
// (fonction pure, testable isolement). Volontairement distinct de
// evt.Status — voir doc de webhookEventPayload.
func webhookEventStatus(event string) string {
	switch event {
	case "message.delivered":
		return "delivered"
	case "message.failed":
		return "failed"
	default:
		return event
	}
}

// buildWebhookPayload construit le corps JSON envoye a un endpoint pour
// l'evenement donne. Fonction pure a l'horodatage pres (time.Now, injecte via
// nowFn pour rester testable sans dependre de l'horloge reelle).
func buildWebhookPayload(nowFn func() time.Time, event string, evt messageEvent) ([]byte, error) {
	return json.Marshal(webhookEventPayload{
		Event:      event,
		MessageID:  evt.MessageID,
		ExternalID: evt.ExternalID,
		ProviderID: evt.ProviderID,
		Status:     webhookEventStatus(event),
		Source:     evt.Source,
		OccurredAt: nowFn().UTC().Format(time.RFC3339),
	})
}

// webhookEndpointRow est le resultat de la selection des endpoints actifs
// abonnes a un evenement (voir loadSubscribedWebhookEndpoints).
type webhookEndpointRow struct {
	ID     string `db:"id"`
	URL    string `db:"url"`
	Secret string `db:"secret"`
}

// loadSubscribedWebhookEndpoints selectionne les endpoints ACTIFS du
// workspace abonnes a event ("$2 = ANY(events)", events etant un text[] —
// meme pattern de colonne que src/api/webhook_endpoints_list.go, mais ici on
// compare une valeur scalaire a la colonne, pas besoin de pq.Array/
// pq.StringArray puisqu'aucun slice Go n'est passe en parametre ni scanne).
//
// E1 (2e revue, fenetre residuelle de B2) : contexte DETACHE et borne (voir
// detachedPersistContext, webhook_persist_context.go). Cette selection a lieu
// AVANT le POST : si elle s'executait sur le ctx annulable du handler AMQP et
// qu'un SIGTERM survenait pile a cet instant, dispatchToNewEndpoint n'irait
// jamais jusqu'a insertPendingWebhookDelivery -> aucune trace du tout (ni
// ligne en base, ni POST), exactement le symptome B2 dans une fenetre plus
// etroite. Le detachement referme cette fenetre.
func (s *Service) loadSubscribedWebhookEndpoints(ctx context.Context, workspaceID, event string) ([]webhookEndpointRow, error) {
	ctx, cancel := detachedPersistContext(ctx)
	defer cancel()

	var rows []webhookEndpointRow
	err := s.DB.Select(ctx, &rows,
		`SELECT id, url, secret
		   FROM webhook.webhook_endpoints
		  WHERE workspace_id = $1
		    AND active = true
		    AND $2 = ANY(events)`,
		workspaceID, event,
	)
	return rows, err
}

// fanOutWebhookEvent orchestre la livraison de l'evenement `event` a tous les
// endpoints actifs du workspace abonnes a cet evenement. Void (regle 4) :
// toute erreur est loguee et absorbee ici, jamais remontee a l'appelant.
//
// APPELEE UNIQUEMENT APRES COMMIT de la transaction/instruction de transition
// de statut (jamais depuis l'interieur), et UNIQUEMENT si la transition a
// reellement eu lieu (workspace_id connu, lu depuis le MEME RETURNING que la
// transition — voir on_message_delivered.go/on_message_failed.go, E2) et doc
// de tete de fichier.
func (s *Service) fanOutWebhookEvent(ctx context.Context, event string, evt messageEvent, workspaceID string) {
	if workspaceID == "" {
		s.Logger.Error("core-processor: webhook fan-out annule (workspace_id vide)",
			"event", event, "message_id", evt.MessageID)
		return
	}

	endpoints, err := s.loadSubscribedWebhookEndpoints(ctx, workspaceID, event)
	if err != nil {
		s.Logger.Error("core-processor: webhook fan-out, chargement endpoints",
			"workspace_id", workspaceID, "event", event, "err", err)
		return
	}
	if len(endpoints) == 0 {
		// Aucun endpoint abonne : operation no-op, pas une anomalie (meme
		// convention que l'ancien DeliverEvent.Execute).
		return
	}

	payload, err := buildWebhookPayload(time.Now, event, evt)
	if err != nil {
		s.Logger.Error("core-processor: webhook fan-out, construction payload",
			"event", event, "message_id", evt.MessageID, "err", err)
		return
	}

	concurrency := webhookFanOutConcurrency
	if len(endpoints) < concurrency {
		concurrency = len(endpoints)
	}

	// syncx.Map : fan-out concurrent borne vers N endpoints. La fonction
	// passee ne retourne JAMAIS d'erreur (dispatchToNewEndpoint absorbe tout
	// en interne) : un echec de livraison vers un endpoint ne doit JAMAIS
	// annuler le contexte partage de syncx.Map et empecher la livraison aux
	// AUTRES endpoints abonnes au meme evenement (regle explicite de la
	// tache : "l'echec de l'un ne doit pas empecher les autres").
	_, _ = syncx.Map(ctx, endpoints, concurrency, func(ctx context.Context, ep webhookEndpointRow) (struct{}, error) {
		s.dispatchToNewEndpoint(ctx, ep, event, payload)
		return struct{}{}, nil
	})
}

// dispatchToNewEndpoint signe puis POSTe le payload vers un endpoint pour sa
// PREMIERE tentative. Void : voir doc de tete de fichier (regle 4).
//
// B2 (correctif D-M43) — ORDRE INSERT-AVANT-POST : la delivery est inseree en
// 'pending' AVANT le POST (insertPendingWebhookDelivery), jamais apres. Si le
// ctx est annule a n'importe quel instant a partir d'ici (SIGTERM), la ligne
// 'pending' deja inseree (avec next_retry_at au bail, voir
// insertPendingWebhookDelivery) est naturellement rattrapee par le scheduler
// de retry — plus jamais de livraison "fantome" sans aucune trace (voir doc
// de tete de fichier). Le resultat final est persiste par UNE SEULE UPDATE
// (finalizeNewWebhookDelivery), sur un contexte DETACHE et borne : un
// resultat DEJA ACQUIS (le POST a eu lieu, succes ou echec) ne doit jamais
// etre perdu par l'annulation du ctx appelant survenue APRES le POST.
func (s *Service) dispatchToNewEndpoint(ctx context.Context, ep webhookEndpointRow, event string, payload []byte) {
	deliveryID, err := s.insertPendingWebhookDelivery(ctx, ep.ID, event, payload)
	if err != nil {
		s.Logger.Error("core-processor: webhook fan-out, persistance delivery (pending)",
			"endpoint_id", ep.ID, "event", event, "err", err)
		return
	}

	signature := signWebhookPayload(ep.Secret, payload)
	statusCode, dispatchErr := s.postWebhook(ctx, ep.URL, payload, signature)

	status := webhookStatusFailed
	var nextRetryAt *time.Time
	switch {
	case dispatchErr == nil && isSuccessStatusCode(statusCode):
		status = webhookStatusDelivered
	case dispatchErr != nil:
		s.Logger.Warn("core-processor: webhook livraison echouee (erreur reseau)",
			"endpoint_id", ep.ID, "event", event, "err", dispatchErr)
		t := time.Now().Add(backoffFor(1))
		nextRetryAt = &t
	default:
		s.Logger.Warn("core-processor: webhook livraison echouee (statut HTTP non-2xx)",
			"endpoint_id", ep.ID, "event", event, "status_code", statusCode)
		t := time.Now().Add(backoffFor(1))
		nextRetryAt = &t
	}

	if err := s.finalizeNewWebhookDelivery(ctx, deliveryID, status, nextRetryAt); err != nil {
		s.Logger.Error("core-processor: webhook fan-out, persistance delivery (finalisation)",
			"endpoint_id", ep.ID, "event", event, "delivery_id", deliveryID, "err", err)
	}
}

// postWebhook effectue le POST HTTP signe vers url. Retourne le code HTTP de
// la reponse (0 en cas d'erreur reseau/timeout) et l'erreur eventuelle.
//
// Headers : Content-Type: application/json, X-Fleece-Signature: <hex HMAC>.
// Le timeout est porte par s.WebhookHTTPClient (10s par defaut, voir
// Service.Init()) — les tests peuvent injecter un client a timeout plus court
// pour exercer le cas timeout sans attendre 10 secondes reelles.
//
// ctx N'EST JAMAIS detache ici (contrairement a la persistance du resultat,
// voir detachedPersistContext) : on ne veut jamais prolonger un appel reseau
// sortant au-dela de l'arret gracieux demande (B1, revue D-M43).
func (s *Service) postWebhook(ctx context.Context, url string, payload []byte, signature string) (int, error) {
	client := s.WebhookHTTPClient
	if client == nil {
		client = defaultWebhookHTTPClient()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Fleece-Signature", signature)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

// isSuccessStatusCode retourne true si code est un code HTTP 2xx. Fonction
// pure, testable isolement.
func isSuccessStatusCode(code int) bool {
	return code >= http.StatusOK && code < http.StatusMultipleChoices
}

// insertPendingWebhookDelivery insere une ligne 'pending' pour endpointID/
// event AVANT toute tentative HTTP (B2, voir doc de tete de fichier) et
// retourne son id (bigserial, migration 0006_webhook.sql). attempts=1 (cette
// toute premiere tentative, en vol). next_retry_at est deja positionne au
// bail de reservation (webhookRetryClaimBail, voir webhook_retry_scheduler.go) :
// si le worker meurt avant finalizeNewWebhookDelivery (POST jamais tente ou
// resultat jamais persiste), cette ligne 'pending' redevient naturellement
// eligible au scheduler de retry des que le bail expire — aucune intervention
// necessaire, aucune ligne definitivement perdue.
//
// N4 (2e revue) — BAIL CALCULE COTE SERVEUR : next_retry_at = now() +
// make_interval(secs => $5), PLUS JAMAIS time.Now().Add(...) calcule cote Go
// puis envoye comme timestamp litteral. Le filtre de selection du scheduler
// de retry compare next_retry_at a now() COTE POSTGRES (voir
// webhook_retry_scheduler.go) : une derive d'horloge pod/DB de plus de
// webhookRetryClaimBail rendrait un bail calcule cote Go inoperant (bail deja
// expire du point de vue Postgres au moment meme ou il est pose, ou au
// contraire jamais expire) -> reselection prematuree/tardive possible par un
// autre replica. Calculer le bail dans la MEME expression SQL que le now()
// du filtre elimine cette classe de derive.
//
// E1 (2e revue, fenetre residuelle de B2) : contexte DETACHE et borne (voir
// detachedPersistContext, webhook_persist_context.go). Cet INSERT a lieu
// AVANT le POST : si le ctx appelant (handler AMQP) etait annule (SIGTERM)
// pile avant/pendant cet INSERT, la ligne 'pending' ne serait jamais creee et
// aucune trace ne subsisterait (meme symptome que B2, fenetre de ~1ms de SQL
// au lieu de ~10s de POST, mais non nulle). Le POST qui suit reste, lui, sur
// le ctx annulable (voir dispatchToNewEndpoint) : s'il echoue en
// context.Canceled, la finalisation (deja detachee) ecrit 'failed' +
// next_retry_at au backoff -> rattrapage normal par le scheduler de retry.
func (s *Service) insertPendingWebhookDelivery(ctx context.Context, endpointID, event string, payload []byte) (int64, error) {
	ctx, cancel := detachedPersistContext(ctx)
	defer cancel()

	var row struct {
		ID int64 `db:"id"`
	}
	// N4 : bail calcule COTE SERVEUR (now() + make_interval(secs => $5)),
	// jamais cote Go — voir doc de fonction.
	err := s.DB.Get(ctx, &row,
		`INSERT INTO webhook.webhook_deliveries (endpoint_id, event, status, attempts, payload, next_retry_at)
		 VALUES ($1, $2, $3, 1, $4, now() + make_interval(secs => $5))
		 RETURNING id`,
		endpointID, event, webhookStatusPending, payload, webhookRetryClaimBail.Seconds(),
	)
	return row.ID, err
}

// finalizeNewWebhookDelivery persiste le resultat de la PREMIERE tentative
// (le POST a deja eu lieu, voir dispatchToNewEndpoint) : nouveau statut et
// next_retry_at. attempts N'EST PAS touche ici : deja pose a 1 par
// insertPendingWebhookDelivery, AVANT le POST (B2) — une seconde ecriture ici
// serait un no-op au mieux, une source de confusion arithmetique au pire.
//
// Contexte DETACHE et borne (B1, voir detachedPersistContext,
// webhook_persist_context.go) : le POST a DEJA EU LIEU a cet instant — un
// SIGTERM survenu pendant ce POST (ctx du handler AMQP annule) ne doit
// JAMAIS empecher la persistance de son resultat, sous peine de laisser la
// ligne 'pending' orpheline (rattrapee seulement apres le bail, au prix d'un
// aller-retour de retry evitable).
func (s *Service) finalizeNewWebhookDelivery(ctx context.Context, id int64, status string, nextRetryAt *time.Time) error {
	ctx, cancel := detachedPersistContext(ctx)
	defer cancel()

	var nrt sql.NullTime
	if nextRetryAt != nil {
		nrt = sql.NullTime{Time: *nextRetryAt, Valid: true}
	}
	return s.DB.Exec(ctx,
		`UPDATE webhook.webhook_deliveries SET status = $1, next_retry_at = $2 WHERE id = $3`,
		status, nrt, id,
	)
}
