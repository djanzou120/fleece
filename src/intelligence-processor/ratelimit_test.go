package intelligenceprocessor

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ============================================================
// rateLimiter — testable sans horloge reelle (nowFn/sleepFn injectables).
// Aucun test ici ne dort reellement.
// ============================================================

// TestNewRateLimiter_zeroRate_unlimited verifie que rate_limit<=0 (0=illimite,
// documente par la migration 0016) ne bloque JAMAIS Wait, quel que soit le
// nombre d'appels.
func TestNewRateLimiter_zeroRate_unlimited(t *testing.T) {
	r := newRateLimiter(0)
	sleepCalls := 0
	r.sleepFn = func(ctx context.Context, d time.Duration) error {
		sleepCalls++
		return nil
	}
	r.nowFn = func() time.Time { return time.Unix(0, 0) }

	for i := 0; i < 5; i++ {
		if err := r.Wait(context.Background()); err != nil {
			t.Fatalf("Wait() erreur inattendue (illimite) : %v", err)
		}
	}
	if sleepCalls != 0 {
		t.Fatalf("sleepFn appele %d fois, voulu 0 (rate_limit=0 = illimite, jamais d'attente)", sleepCalls)
	}
}

// TestNewRateLimiter_negativeRate_unlimited couvre rate_limit<0 (jamais en
// pratique cote colonne DB, mais defensif).
func TestNewRateLimiter_negativeRate_unlimited(t *testing.T) {
	r := newRateLimiter(-5)
	if r.interval > 0 {
		t.Fatalf("newRateLimiter(-5).interval = %v, voulu <= 0 (illimite)", r.interval)
	}
}

// TestRateLimiter_boundedRate verifie qu'un rate_limit > 0 espace bien les
// appels d'au moins `interval`, via une horloge factice avancee manuellement
// par le sleepFn factice (jamais de vrai time.Sleep).
func TestRateLimiter_boundedRate(t *testing.T) {
	r := newRateLimiter(60) // 60/minute => 1 par seconde
	if r.interval != time.Second {
		t.Fatalf("interval = %v, voulu 1s pour rate_limit=60", r.interval)
	}

	clock := time.Unix(1000, 0)
	r.nowFn = func() time.Time { return clock }
	var sleptFor []time.Duration
	r.sleepFn = func(ctx context.Context, d time.Duration) error {
		sleptFor = append(sleptFor, d)
		clock = clock.Add(d) // avance l'horloge factice comme le ferait un vrai sleep
		return nil
	}

	// 1er appel : jamais d'attente (r.next initialise a `now`).
	if err := r.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() #1 erreur inattendue : %v", err)
	}
	if len(sleptFor) != 0 {
		t.Fatalf("Wait() #1 n'aurait pas du attendre, sleptFor=%v", sleptFor)
	}

	// 2e appel immediat (clock inchangee) : doit attendre exactement 1s.
	if err := r.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() #2 erreur inattendue : %v", err)
	}
	if len(sleptFor) != 1 || sleptFor[0] != time.Second {
		t.Fatalf("Wait() #2 attente = %v, voulu [1s]", sleptFor)
	}

	// 3e appel : la clock a deja avance de l'intervalle complet (via le sleep
	// precedent) -> pas de nouvelle attente needed tout de suite, mais un
	// appel immediat sans avancer manuellement l'horloge doit re-attendre.
	if err := r.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() #3 erreur inattendue : %v", err)
	}
	if len(sleptFor) != 2 || sleptFor[1] != time.Second {
		t.Fatalf("Wait() #3 attente = %v, voulu 2 attentes de 1s", sleptFor)
	}
}

// TestRateLimiter_noBurstAfterIdlePeriod verifie que `next` est recale sur
// `now` apres une longue periode d'inactivite (pas de rafale de rattrapage),
// comme documente dans ratelimit.go.
func TestRateLimiter_noBurstAfterIdlePeriod(t *testing.T) {
	r := newRateLimiter(60) // interval = 1s
	clock := time.Unix(2000, 0)
	r.nowFn = func() time.Time { return clock }
	r.sleepFn = func(ctx context.Context, d time.Duration) error {
		clock = clock.Add(d)
		return nil
	}

	if err := r.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() #1 erreur inattendue : %v", err)
	}

	// Longue periode d'inactivite (1 heure) avant le 2e appel : ne doit
	// declencher AUCUNE attente (r.next est dans le passe, recale sur now).
	clock = clock.Add(time.Hour)
	if err := r.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() #2 erreur inattendue : %v", err)
	}
}

// TestRateLimiter_contextCanceledDuringWait verifie qu'une annulation de
// contexte pendant l'attente remonte ctx.Err() sans jamais dormir reellement.
func TestRateLimiter_contextCanceledDuringWait(t *testing.T) {
	r := newRateLimiter(60)
	clock := time.Unix(3000, 0)
	r.nowFn = func() time.Time { return clock }
	r.sleepFn = func(ctx context.Context, d time.Duration) error {
		return ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// 1er appel : jamais d'attente (r.next=now).
	if err := r.Wait(ctx); err != nil {
		t.Fatalf("Wait() #1 erreur inattendue : %v", err)
	}
	// 2e appel : declenche l'attente, sleepFn factice retourne ctx.Err()
	// (contexte deja annule) au lieu de dormir.
	err := r.Wait(ctx)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait() apres annulation de contexte = %v, voulu context.Canceled", err)
	}
}

// ============================================================
// contextSleep — fonction reellement utilisee en production (newRateLimiter).
// Testee avec des durees infimes pour ne jamais depasser ~quelques ms.
// ============================================================

func TestContextSleep_nonPositiveDuration_returnsImmediately(t *testing.T) {
	if err := contextSleep(context.Background(), 0); err != nil {
		t.Fatalf("contextSleep(0) erreur inattendue : %v", err)
	}
	if err := contextSleep(context.Background(), -time.Second); err != nil {
		t.Fatalf("contextSleep(negatif) erreur inattendue : %v", err)
	}
}

func TestContextSleep_completesNormally(t *testing.T) {
	if err := contextSleep(context.Background(), 5*time.Millisecond); err != nil {
		t.Fatalf("contextSleep() erreur inattendue : %v", err)
	}
}

func TestContextSleep_canceledBeforeExpiry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := contextSleep(ctx, time.Minute) // duree longue mais annulation immediate
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("contextSleep() apres annulation = %v, voulu context.Canceled", err)
	}
}
