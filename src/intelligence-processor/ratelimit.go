package intelligenceprocessor

// ratelimit.go — token bucket / pacer simple pour on_campaign_run.go,
// derive de campaign.campaigns.rate_limit (migration 0016 : entier,
// 0 = illimite, N > 0 = messages/minute max pour ce run).
//
// Fonction/struct PURE et testable SANS horloge reelle (regle transverse n°9)
// : nowFn et sleepFn sont injectables. En production, newRateLimiter cable
// time.Now et un sleepFn conscient du contexte (contextSleep) ; les tests
// injectent une horloge et un sleepFn factices, deterministes, qui ne
// dorment jamais reellement.
import (
	"context"
	"time"
)

// rateLimiter espace les appels a Wait d'au moins `interval`. interval<=0
// signifie "illimite" (Wait ne bloque jamais).
type rateLimiter struct {
	interval time.Duration
	nowFn    func() time.Time
	sleepFn  func(ctx context.Context, d time.Duration) error
	next     time.Time
}

// newRateLimiter construit un rateLimiter a partir d'un rate_limit exprime en
// messages/minute (campaign.campaigns.rate_limit). ratePerMinute<=0 =
// illimite (comportement documente par la migration 0016 : "0 = illimite").
func newRateLimiter(ratePerMinute int) *rateLimiter {
	r := &rateLimiter{nowFn: time.Now, sleepFn: contextSleep}
	if ratePerMinute > 0 {
		r.interval = time.Minute / time.Duration(ratePerMinute)
	}
	return r
}

// contextSleep attend d, sauf annulation anticipee de ctx (retourne alors
// ctx.Err()). d<=0 retourne immediatement sans creer de timer.
func contextSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Wait bloque (via sleepFn) jusqu'a ce que le prochain envoi soit autorise
// (au moins `interval` depuis le precedent appel a Wait), ou retourne
// ctx.Err() si ctx est annule avant. Illimite (interval<=0) : ne bloque
// jamais, retourne immediatement nil.
func (r *rateLimiter) Wait(ctx context.Context) error {
	if r.interval <= 0 {
		return nil
	}

	now := r.nowFn()
	if r.next.IsZero() {
		r.next = now
	}

	if now.Before(r.next) {
		if err := r.sleepFn(ctx, r.next.Sub(now)); err != nil {
			return err
		}
		now = r.nowFn()
	}

	// Ne jamais laisser `next` derailler dans le passe (ex. apres une longue
	// pause d'inactivite entre deux Wait) : on le recale sur `now` avant
	// d'ajouter l'intervalle, pour ne pas autoriser une rafale de rattrapage.
	if r.next.Before(now) {
		r.next = now
	}
	r.next = r.next.Add(r.interval)
	return nil
}
