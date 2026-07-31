package coreprocessor

// webhook_retry_scheduler.go — ticker qui retente les livraisons de webhooks
// sortants en echec (D-M43, T-005 volet livraison).
//
// ANTI-DOUBLE-TRAITEMENT (meme vigilance que campaign_scheduler.go,
// intelligence-processor, critere de QA M-022 — core-processor est deploye a
// 3 replicas, cf. deploy/k8s/services/core-processor.yaml) : la selection ET
// le marquage ("cette livraison est prise en charge par CE tick") sont UNE
// SEULE requete "UPDATE ... FROM ... RETURNING", JAMAIS un SELECT suivi d'un
// UPDATE separe. La requete joint directement webhook.webhook_endpoints
// (URL + secret COURANT) pour eviter tout second aller-retour DB par ligne
// capturee.
//
// B1 (BLOQUANT, revue architecture Phase 3) — MARQUAGE PAR BAIL, PLUS PAR
// NULL : AVANT ce correctif, le marquage consistait a mettre
// next_retry_at = NULL. C'etait correct pour l'anti-double-traitement (une
// ligne a NULL ne matche plus le filtre "next_retry_at IS NOT NULL"), MAIS
// **irreversible sans intervention** : si le worker mourait entre la capture
// et finalizeWebhookRetry (SIGTERM en plein tick — l'arret gracieux NORMAL
// d'un deploiement, pas un crash rare, cf. syncx.Map qui abandonne les taches
// non commencees des ctx.Done()), la ligne restait "status IN ('pending',
// 'failed')" avec next_retry_at = NULL a jamais : plus JAMAIS reselectionnee
// par aucun replica, aucun bail, aucun reaper (meme pattern que D-M31, ligne
// piegee en 'sending'). CORRECTIF : le marquage pose desormais
// next_retry_at a un BAIL futur (now() + webhookRetryClaimBail, quelques
// minutes) plutot qu'a NULL. Une ligne orpheline (worker mort avant
// finalizeWebhookRetry) redevient donc NATURELLEMENT eligible des que le bail
// expire — sans la moindre intervention manuelle. L'anti-double-traitement
// tient TOUJOURS : le filtre "next_retry_at <= now()" ne matche plus une
// ligne dont next_retry_at vient d'etre repousse de plusieurs minutes dans le
// futur, exactement comme il ne matchait plus une ligne a NULL — Postgres
// verrouille la ligne pendant l'UPDATE, un second replica executant la MEME
// requete en concurrence ne peut jamais la capturer une deuxieme fois (il ne
// la voit tout simplement plus une fois le premier UPDATE applique/committe).
//
// ATTEMPTS INCREMENTE A LA CAPTURE (consequence directe du bail) : la meme
// requete UPDATE incremente desormais attempts (SET attempts = wd.attempts + 1)
// EN MEME TEMPS qu'elle pose le bail — la capture a elle seule "consomme" une
// tentative, qu'elle aboutisse ou non a un POST reellement effectue (protection
// contre une boucle infinie sur une ligne orpheline : sans cet increment
// EAGER, une ligne qui expire son bail sans jamais etre finalisee serait
// recapturee indefiniment sans jamais approcher webhookRetryMaxAttempts).
// CONSEQUENCE OBLIGATOIRE : retryWebhookDelivery/finalizeWebhookRetry NE
// DOIVENT PLUS reincrementer attempts (ancien code : "attempts := d.Attempts + 1")
// — d.Attempts EST DEJA la valeur post-capture (le +1 a deja ete applique par
// CETTE requete, RETURNING renvoie la valeur APRES le SET). Un double
// increment fausserait silencieusement le compteur (une livraison
// "exhausted" apres 3 tentatives reelles au lieu de 5, un backoff applique au
// mauvais palier). Voir retryWebhookDelivery ci-dessous : attempts = d.Attempts,
// SANS +1.
//
// Le filtre de selection reste coherent avec un bail futur : "next_retry_at
// IS NOT NULL AND next_retry_at <= now()" continue de fonctionner a
// l'identique (un bail est un next_retry_at futur comme un autre, jusqu'a
// expiration).
//
// E2 (BLOQUANT introduit par B1, 2e revue architecture Phase 3) — CAPTURE
// BORNEE (LIMIT + FOR UPDATE SKIP LOCKED) : AVANT ce correctif, la capture
// n'avait AUCUNE limite — un tick capturait TOUTES les lignes dues d'un
// coup, puis les traitait webhookFanOutConcurrency (5) par 5. La duree totale
// d'un lot est donc N/5 x webhookDispatchTimeout(10s), NON BORNEE : des que
// N depasse ~150, cette duree depasse le bail (webhookRetryClaimBail, 5min)
// -> le bail du lot en cours EXPIRE avant la fin du traitement -> un AUTRE
// replica (core-processor tourne a 3 replicas) recapture les MEMES lignes,
// encore "eligibles" a ses yeux puisque leur bail a expire -> DOUBLE
// LIVRAISON chez le client final ET double increment de attempts (epuisement
// premature, une ligne devient 'exhausted' apres moins de tentatives reelles
// que prevu). CORRECTIF : la selection des lignes eligibles se fait desormais
// dans une sous-requete BORNEE (webhookRetryCaptureLimit lignes maximum) avec
// "FOR UPDATE SKIP LOCKED" — un replica qui capture un lot ne bloque jamais
// les autres replicas sur les lignes qu'il vient de verrouiller (ils
// sautent simplement ces lignes et capturent les suivantes), ce qui reduit
// aussi la contention inter-replicas independamment du dimensionnement de la
// LIMIT. CRITIQUE : cette sous-requete reste PARTIE INTEGRANTE de l'UNIQUE
// requete UPDATE ... RETURNING (aucun aller-retour SELECT puis UPDATE separe
// -> l'atomicite/anti-double-traitement du commentaire ci-dessus tient
// TOUJOURS, voir webhookRetryCaptureLimit pour le dimensionnement chiffre et
// TestRunWebhookRetryTick_singleQuery_notSelectThenUpdate pour la garantie
// testee).
//
// E6 (revue D-M43) — la capture joint desormais EXPLICITEMENT we.active = true :
// avant ce correctif, un endpoint desactive (DELETE logique, SET active =
// false) continuait de recevoir jusqu'a 4 retries sur ~2h36 apres sa
// suppression (la jointure ne filtrait pas sur active). Un endpoint
// desactive n'est plus jamais retente par ce scheduler.
//
// BACKOFF (repris a l'identique de src/webhook/internal/application/usecases/
// retry_delivery.go, service a supprimer par M-023) : tentative 1 -> 1m,
// 2 -> 5m, 3 -> 30m, 4 -> 2h (voir backoffFor). N5 (2e revue) — CE QUI SE
// PASSE REELLEMENT AU-DELA : attempts est INCREMENTE A LA CAPTURE (voir
// ci-dessus) et plafonne a webhookRetryMaxAttempts=5 ; a la 5e tentative
// cumulee (1 premiere livraison + 4 retries), retryWebhookDelivery bascule en
// 'exhausted' AVANT tout appel a backoffFor (voir "case attempts >=
// webhookRetryMaxAttempts" ci-dessous) — le palier "8h et au-dela" du
// commentaire de backoffFor n'est donc JAMAIS atteint en pratique : la
// fenetre totale reelle est 1m + 5m + 30m + 2h ~= 2h36 pour 4 retries (5 POST
// au total). Ce palier 8h inatteignable est CONFORME a l'implementation de
// reference (retry_delivery.go) — backoffFor n'est pas modifiee par ce
// correctif, seule la documentation l'est.
import (
	"context"
	"database/sql"
	"time"

	syncx "fleece/src/go/syncx"
)

// webhookRetryMaxAttempts est le nombre maximum de tentatives cumulees
// (premiere livraison incluse) avant de passer une delivery en 'exhausted'.
const webhookRetryMaxAttempts = 5

// webhookRetryClaimBail est le delai de reservation (bail) applique par la
// capture atomique (runWebhookRetryTick) ET par l'insertion initiale d'une
// delivery (insertPendingWebhookDelivery, webhook_dispatch.go, B2) : le temps
// pendant lequel une ligne "en cours de traitement par CE worker/CE tick" est
// protegee d'une reselection par un autre replica/tick, ET le delai maximum
// avant qu'une ligne ORPHELINE (worker mort avant finalisation) redevienne
// eligible sans intervention (B1). Quelques minutes : suffisant pour largement
// couvrir la duree d'un POST HTTP (webhookDispatchTimeout = 10s) plus sa
// finalisation, sans pour autant retarder indument un vrai rattrapage.
const webhookRetryClaimBail = 5 * time.Minute

// webhookRetryCaptureLimit borne le nombre de lignes capturees par UN SEUL
// tick (E2, 2e revue architecture Phase 3 — voir doc de tete de fichier).
//
// DIMENSIONNEMENT CHIFFRE : un lot de webhookRetryCaptureLimit lignes est
// traite par vagues de webhookFanOutConcurrency (5) livraisons concurrentes,
// chaque livraison bornee a webhookDispatchTimeout (10s) au pire. Duree
// maximale du lot = (webhookRetryCaptureLimit / webhookFanOutConcurrency) *
// webhookDispatchTimeout = (100 / 5) * 10s = 200s. Cette duree doit rester
// tres inferieure au bail (webhookRetryClaimBail = 5min = 300s) pour qu'un
// AUTRE replica ne puisse jamais recapturer les memes lignes avant la fin de
// leur traitement (E2) : 200s < 300s, marge de 100s (33%) qui absorbe la
// latence reseau/DB normale sans remettre en cause l'invariant.
const webhookRetryCaptureLimit = 100

// claimedWebhookDelivery est une ligne capturee par l'UPDATE atomique de
// runWebhookRetryTick : les colonnes de webhook_deliveries necessaires au
// retry, plus url/secret de l'endpoint cible (jointure, voir doc de tete de
// fichier). Attempts est LA VALEUR POST-CAPTURE (deja incrementee par cette
// meme requete, voir doc de tete de fichier) — PLUS JAMAIS incrementee en
// aval.
type claimedWebhookDelivery struct {
	ID          int64  `db:"id"`
	Event       string `db:"event"`
	Payload     []byte `db:"payload"`
	Attempts    int    `db:"attempts"`
	EndpointID  string `db:"endpoint_id"`
	EndpointURL string `db:"url"`
	Secret      string `db:"secret"`
}

// RunWebhookRetryScheduler boucle jusqu'a annulation de ctx, executant
// runWebhookRetryTick toutes les `interval`. Meme pattern que
// src/intelligence-processor/campaign_scheduler.go (RunCampaignScheduler).
func (s *Service) RunWebhookRetryScheduler(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.Logger.Info("core-processor: webhook_retry_scheduler arret demande")
			return
		case <-ticker.C:
			s.runWebhookRetryTick(ctx)
		}
	}
}

// runWebhookRetryTick capture atomiquement les livraisons eligibles au retry
// (une seule requete UPDATE ... RETURNING, voir doc de tete de fichier — la
// capture est BORNEE a webhookRetryCaptureLimit lignes via une sous-requete
// "FOR UPDATE SKIP LOCKED", E2), puis retente chacune (fan-out concurrent
// borne, meme motif que fanOutWebhookEvent). Un echec de tick (panne DB) est
// loggue ERROR mais ne tue jamais le worker — le tick suivant retentera.
func (s *Service) runWebhookRetryTick(ctx context.Context) {
	var claimed []claimedWebhookDelivery
	// N4 : le bail est calcule COTE SERVEUR (now() + make_interval(secs =>
	// $2)), jamais cote Go, pour rester coherent avec le now() du filtre de
	// selection (voir doc de tete de fichier, N4).
	//
	// E2 : la sous-requete IN (SELECT ... FOR UPDATE SKIP LOCKED LIMIT $3)
	// borne la capture a webhookRetryCaptureLimit lignes tout en restant
	// PARTIE de cette UNIQUE requete UPDATE ... RETURNING (atomicite
	// preservee — voir TestRunWebhookRetryTick_singleQuery_notSelectThenUpdate).
	err := s.DB.Select(ctx, &claimed,
		`UPDATE webhook.webhook_deliveries wd
		    SET attempts = wd.attempts + 1,
		        next_retry_at = now() + make_interval(secs => $2)
		   FROM webhook.webhook_endpoints we
		  WHERE wd.endpoint_id = we.id
		    AND we.active = true
		    AND wd.id IN (
		      SELECT id
		        FROM webhook.webhook_deliveries
		       WHERE status IN ('pending', 'failed')
		         AND next_retry_at IS NOT NULL
		         AND next_retry_at <= now()
		         AND attempts < $1
		       ORDER BY next_retry_at
		       FOR UPDATE SKIP LOCKED
		       LIMIT $3
		    )
		RETURNING wd.id, wd.event, wd.payload, wd.attempts, wd.endpoint_id, we.url, we.secret`,
		webhookRetryMaxAttempts, webhookRetryClaimBail.Seconds(), webhookRetryCaptureLimit,
	)
	if err != nil {
		s.Logger.Error("core-processor: webhook_retry_scheduler tick", "err", err)
		return
	}
	if len(claimed) == 0 {
		return
	}

	concurrency := webhookFanOutConcurrency
	if len(claimed) < concurrency {
		concurrency = len(claimed)
	}
	_, _ = syncx.Map(ctx, claimed, concurrency, func(ctx context.Context, d claimedWebhookDelivery) (struct{}, error) {
		s.retryWebhookDelivery(ctx, d)
		return struct{}{}, nil
	})
}

// retryWebhookDelivery retente UNE livraison deja capturee par
// runWebhookRetryTick : signe le payload PERSISTE (colonne payload bytea,
// jamais reconstruit) avec le secret COURANT de l'endpoint (relu par la
// jointure de la requete de capture — un secret peut avoir change depuis la
// premiere tentative), POSTe, puis persiste le nouvel etat (status,
// next_retry_at) via UNE SEULE UPDATE (finalizeWebhookRetry). attempts N'EST
// PLUS recalcule ici (B1) : d.Attempts est DEJA la valeur post-capture
// (incrementee par runWebhookRetryTick), voir doc de tete de fichier.
func (s *Service) retryWebhookDelivery(ctx context.Context, d claimedWebhookDelivery) {
	if len(d.Payload) == 0 {
		// Garde de surete (portee de src/webhook/internal/application/
		// usecases/retry_delivery.go, ErrEmptyPayload) : un payload vide est
		// une anomalie de donnees, jamais rejouable a l'aveugle -> exhausted
		// immediat plutot qu'une boucle de retry infructueuse. d.Attempts
		// (deja incremente par la capture) est persiste tel quel : aucune
		// tentative reseau reelle n'a ete faite, mais la capture a deja
		// "consomme" une tentative pour eviter une recapture infinie de cette
		// ligne anormale.
		s.Logger.Error("core-processor: webhook retry, payload vide (anomalie, passage direct en exhausted)",
			"delivery_id", d.ID, "endpoint_id", d.EndpointID)
		if err := s.finalizeWebhookRetry(ctx, d.ID, webhookStatusExhausted, d.Attempts, nil); err != nil {
			s.Logger.Error("core-processor: webhook retry, finalisation (payload vide)",
				"delivery_id", d.ID, "err", err)
		}
		return
	}

	signature := signWebhookPayload(d.Secret, d.Payload)
	statusCode, dispatchErr := s.postWebhook(ctx, d.EndpointURL, d.Payload, signature)

	// B1 : attempts est DEJA la valeur post-capture (incrementee par
	// runWebhookRetryTick) — NE JAMAIS reincrementer ici (double comptage,
	// backoff/exhausted errones).
	attempts := d.Attempts
	var status string
	var nextRetryAt *time.Time

	switch {
	case dispatchErr == nil && isSuccessStatusCode(statusCode):
		status = webhookStatusDelivered
	case attempts >= webhookRetryMaxAttempts:
		status = webhookStatusExhausted
		if dispatchErr != nil {
			s.Logger.Warn("core-processor: webhook retry epuise (erreur reseau)",
				"delivery_id", d.ID, "endpoint_id", d.EndpointID, "attempts", attempts, "err", dispatchErr)
		} else {
			s.Logger.Warn("core-processor: webhook retry epuise (statut HTTP non-2xx)",
				"delivery_id", d.ID, "endpoint_id", d.EndpointID, "attempts", attempts, "status_code", statusCode)
		}
	default:
		status = webhookStatusFailed
		t := time.Now().Add(backoffFor(attempts))
		nextRetryAt = &t
	}

	if err := s.finalizeWebhookRetry(ctx, d.ID, status, attempts, nextRetryAt); err != nil {
		s.Logger.Error("core-processor: webhook retry, persistance", "delivery_id", d.ID, "err", err)
	}
}

// finalizeWebhookRetry persiste le resultat d'une retentative : nouveau
// statut, attempts (VALEUR POST-CAPTURE, voir doc de tete de fichier — cette
// fonction ne fait qu'ECRIRE la valeur recue, jamais l'incrementer une
// deuxieme fois), prochain next_retry_at (NULL si delivered/exhausted — plus
// jamais retentee).
//
// Contexte DETACHE et borne (B1, voir detachedPersistContext,
// webhook_persist_context.go) : le POST a DEJA EU LIEU (ou l'anomalie payload
// vide deja detectee) a cet instant — un SIGTERM survenu pendant le POST (ctx
// racine annule) ne doit JAMAIS empecher la persistance de son resultat, sous
// peine de laisser la ligne orpheline jusqu'a l'expiration du bail (B1)
// plutot que finalisee immediatement.
func (s *Service) finalizeWebhookRetry(ctx context.Context, id int64, status string, attempts int, nextRetryAt *time.Time) error {
	ctx, cancel := detachedPersistContext(ctx)
	defer cancel()

	var nrt sql.NullTime
	if nextRetryAt != nil {
		nrt = sql.NullTime{Time: *nextRetryAt, Valid: true}
	}
	return s.DB.Exec(ctx,
		`UPDATE webhook.webhook_deliveries
		    SET status = $1, attempts = $2, next_retry_at = $3
		  WHERE id = $4`,
		status, attempts, nrt, id,
	)
}

// backoffFor retourne la duree d'attente avant la prochaine tentative selon
// le schema de backoff exponentiel (repris a l'identique de
// src/webhook/internal/application/usecases/retry_delivery.go, y compris son
// palier "5 et au-dela -> 8h" qui n'est en pratique JAMAIS atteint, voir
// N5 ci-dessous) :
//
//	tentative 1 -> 1 min
//	tentative 2 -> 5 min
//	tentative 3 -> 30 min
//	tentative 4 -> 2 h
//	tentative 5 et au-dela -> 8 h (plafond, INATTEIGNABLE dans ce worker)
//
// N5 (2e revue) : dans runWebhookRetryTick/retryWebhookDelivery,
// webhookRetryMaxAttempts=5 fait basculer une delivery en 'exhausted' DES la
// 5e tentative cumulee, AVANT tout appel a backoffFor(5) — ce palier "8h"
// n'est donc jamais exerce par ce worker ; seuls backoffFor(1..4) le sont
// reellement (fenetre totale ~2h36 pour 4 retries, 5 POST au total). Ce
// comportement est CONFORME a l'implementation de reference et n'est pas
// modifie ici : seule la documentation ci-dessus et de webhook_retry_
// scheduler.go (tete de fichier) reflete desormais ce fait.
//
// Fonction pure, exportee au sens du package (minuscule = privee au package
// coreprocessor) pour rester testable isolement.
func backoffFor(attempts int) time.Duration {
	switch attempts {
	case 1:
		return 1 * time.Minute
	case 2:
		return 5 * time.Minute
	case 3:
		return 30 * time.Minute
	case 4:
		return 2 * time.Hour
	default:
		return 8 * time.Hour
	}
}
