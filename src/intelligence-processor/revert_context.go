package intelligenceprocessor

// revert_context.go — contexte détaché pour les écritures qui NE DOIVENT PAS
// être perdues à cause de l'annulation du contexte appelant (D-M31).
//
// Jumeau de src/core-processor/webhook_persist_context.go, et pour la même
// raison : un `ctx` de handler AMQP est annulé dès le SIGTERM, c'est-à-dire à
// chaque rolling deploy. Toute écriture qui ENREGISTRE UN FAIT DÉJÀ ACQUIS ne
// doit pas dépendre de ce contexte, sans quoi la ligne concernée reste dans un
// état intermédiaire dont plus aucun chemin ne la sort.
//
// CE QUI RESTE SUR LE CTX ANNULABLE : les appels réseau SORTANTS (le
// POST /messages de sendCampaignMessage). On ne prolonge jamais un envoi
// au-delà de l'arrêt demandé — c'est précisément ce qu'il ne faut pas faire.
// Seule la trace de son issue est détachée.

import (
	"context"
	"time"
)

// revertPersistTimeout borne le contexte détaché. Un UPDATE par identifiant ne
// devrait jamais en approcher : ce n'est pas un budget de latence attendu mais
// un filet, pour qu'un arrêt gracieux ne puisse pas être retardé indéfiniment
// par une base qui ne répond plus.
const revertPersistTimeout = 5 * time.Second

// detachedRevertContext retourne un contexte détaché de l'annulation de ctx et
// borné à revertPersistTimeout. À réserver aux écritures enregistrant un
// résultat déjà connu (voir doc de tête de fichier).
func detachedRevertContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), revertPersistTimeout)
}
