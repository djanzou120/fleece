package coreprocessor

// webhook_persist_context.go — contexte DETACHE utilise pour toute
// persistance/lecture SQL de ce volet webhook sortant qui NE DOIT JAMAIS etre
// perdue/abandonnee a cause de l'annulation du ctx de l'appelant (B1,
// BLOQUANT, revue architecture Phase 3 / D-M43 ; fenetre residuelle refermee
// par E1, 2e revue).
//
// PROBLEME (avant B1) : finalizeWebhookRetry (webhook_retry_scheduler.go) et
// l'ancien recordWebhookDelivery (webhook_dispatch.go) reutilisaient tel quel
// le ctx de l'appelant — celui du handler AMQP (consumer.go) pour la
// premiere tentative, ou le ctx racine (via syncx.Map) pour un tick de
// retry. Ce ctx est annule des le SIGTERM (arret gracieux, deploiement
// Kubernetes NORMAL — pas un crash rare) :
//
//  1. runWebhookRetryTick passe le ctx racine a syncx.Map, dont les workers
//     font `select { case <-ctx.Done(): return }` : au SIGTERM en plein
//     tick, une tache DEJA en cours (POST parti, ou meme deja termine mais
//     pas encore persiste) voit son UPDATE de finalisation echouer
//     immediatement (context.Canceled) -> la ligne reste dans un etat non
//     final, potentiellement plus jamais retentee (meme pattern que D-M31 :
//     ligne piegee).
//  2. Le POST lui-meme peut echouer en context.Canceled (ctx annule pendant
//     l'appel HTTP) ; persister CE resultat (echec) avec le MEME ctx annule
//     echoue tout autant.
//
// CORRECTIF (B1) : toute fonction qui persiste un resultat DEJA CONNU (le
// POST a deja eu lieu, ou son absence de resultat est deja actee) detache son
// ctx de l'annulation de l'appelant (context.WithoutCancel) et le borne a un
// timeout de securite COURT (webhookPersistTimeout) — pour ne jamais bloquer
// indefiniment un arret gracieux, mais laisser le temps a UNE ecriture SQL
// simple de s'executer meme si le ctx appelant vient d'etre annule.
//
// E1 (2e revue, refermeture de la fenetre residuelle de B2) : le MEME
// detachement s'applique desormais aussi AVANT le POST, aux deux requetes de
// dispatchToNewEndpoint/fanOutWebhookEvent qui doivent avoir lieu quel que
// soit le sort du ctx appelant pour que la garantie B2 ("toute delivery
// tentee laisse une trace") tienne vraiment : loadSubscribedWebhookEndpoints
// (chargement des endpoints abonnes) et insertPendingWebhookDelivery (INSERT
// 'pending' avant le POST). Sans ce detachement, un SIGTERM survenant entre le
// commit de la transition de statut et l'une de ces deux requetes laissait ni
// ligne en base ni POST envoye : la meme "livraison fantome, jamais tracee"
// que B2, dans une fenetre plus etroite (~1ms de SQL au lieu de ~10s de POST)
// mais non nulle. Voir webhook_dispatch.go, loadSubscribedWebhookEndpoints et
// insertPendingWebhookDelivery.
//
// CE QUI RESTE SUR LE CTX ANNULABLE D'ORIGINE : le POST HTTP lui-meme
// (postWebhook) — on ne veut JAMAIS prolonger un appel reseau SORTANT
// au-dela de l'arret demande. Toute autre operation SQL de ce volet
// (lecture des endpoints abonnes, INSERT pending, UPDATE de finalisation)
// beneficie du detachement.
import (
	"context"
	"time"
)

// webhookPersistTimeout borne la duree du contexte detache utilise pour
// persister un resultat de livraison webhook deja acquis. Une ecriture SQL
// simple (UPDATE par id) ne devrait jamais approcher cette valeur en
// fonctionnement normal — ce timeout n'est qu'un filet de securite, pas un
// budget de latence attendu.
const webhookPersistTimeout = 5 * time.Second

// detachedPersistContext retourne un contexte DETACHE de l'annulation de ctx
// (context.WithoutCancel) et borne a webhookPersistTimeout (context.WithTimeout).
// A utiliser exclusivement pour persister un resultat DEJA ACQUIS (voir doc de
// tete de fichier) — jamais pour l'appel HTTP sortant lui-meme.
func detachedPersistContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), webhookPersistTimeout)
}
