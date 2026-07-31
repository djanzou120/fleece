package coreprocessor

// webhook_persist_context_test.go — teste detachedPersistContext (B1,
// BLOQUANT, revue architecture Phase 3) : la persistance d'un resultat DEJA
// ACQUIS ne doit JAMAIS etre annulee par le SIGTERM/l'arret gracieux qui a
// annule le ctx appelant.
import (
	"context"
	"testing"
	"time"
)

// TestDetachedPersistContext_survivesParentCancellation prouve, au niveau le
// plus pur possible (sans DB), que le contexte retourne N'HERITE PAS de
// l'annulation du parent : Err() est nil et Done() n'est pas ferme, meme si
// le parent est DEJA annule AVANT l'appel (exactement le scenario SIGTERM en
// plein tick/dispatch, D-M43 B1).
func TestDetachedPersistContext_survivesParentCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel() // deja annule AVANT l'appel — simule un SIGTERM survenu pendant le POST.

	ctx, cancelDetached := detachedPersistContext(parent)
	defer cancelDetached()

	if err := ctx.Err(); err != nil {
		t.Fatalf("ctx detache en erreur (%v) malgre context.WithoutCancel — l'annulation du parent ne doit JAMAIS l'affecter", err)
	}
	select {
	case <-ctx.Done():
		t.Fatal("ctx detache deja Done() alors que seul le PARENT etait annule — context.WithoutCancel doit isoler completement l'annulation parent")
	default:
	}
}

// TestDetachedPersistContext_hasTimeout prouve le filet de securite (borne
// webhookPersistTimeout) : un contexte detache n'est PAS annulable par le
// parent, mais reste borne pour ne jamais bloquer indefiniment un arret.
func TestDetachedPersistContext_hasTimeout(t *testing.T) {
	ctx, cancel := detachedPersistContext(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("ctx detache sans deadline — devrait etre borne par webhookPersistTimeout")
	}
	if d := time.Until(deadline); d <= 0 || d > webhookPersistTimeout {
		t.Errorf("deadline dans %v, hors bornes attendues (0, %v]", d, webhookPersistTimeout)
	}
}

// TestFinalizeWebhookRetry_survivesCanceledContext est LE test bout-en-bout
// (via le faux driver database/sql/driver, pas seulement une assertion pure
// sur context.Context) : sans le contexte detache, database/sql renvoie
// "context canceled" AVANT MEME d'atteindre le driver (0 requete executee,
// verifie manuellement lors de l'implementation) des que le ctx est deja
// annule — c'est EXACTEMENT ce qui, avant ce correctif (B1), piegeait
// definitivement une ligne de webhook_deliveries des qu'un SIGTERM survenait
// pendant runWebhookRetryTick. Avec le contexte detache, finalizeWebhookRetry
// persiste son resultat MALGRE un ctx appelant deja annule.
func TestFinalizeWebhookRetry_survivesCanceledContext(t *testing.T) {
	conn := &fakeConn{}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // simule un SIGTERM survenu pendant runWebhookRetryTick.

	nextRetryAt := time.Now().Add(5 * time.Minute)
	if err := svc.finalizeWebhookRetry(ctx, 42, webhookStatusFailed, 2, &nextRetryAt); err != nil {
		t.Fatalf("finalizeWebhookRetry() erreur inattendue avec ctx appelant annule (le contexte detache doit absorber l'annulation) : %v", err)
	}
	if len(conn.queries) != 1 {
		t.Fatalf("requete de finalisation non executee malgre le contexte detache : %d requetes", len(conn.queries))
	}
}
