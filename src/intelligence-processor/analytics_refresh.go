package intelligenceprocessor

// analytics_refresh.go — ticker REFRESH MATERIALIZED VIEW analytics.kpi_daily.
//
// DETTE D-M03 (voir .ia/MIGRATION_PLAN.md) GEREE EXPLICITEMENT ICI : la MV
// analytics.kpi_daily est creee "WITH NO DATA" (migration 0017), et
// PostgreSQL EXIGE que la MV ait deja ete peuplee au moins une fois pour
// accepter l'option CONCURRENTLY. Strategie retenue :
//  1. Tenter REFRESH ... CONCURRENTLY (cas nominal steady-state : refresh
//     sans verrou, compatible avec des lectures simultanees).
//  2. Si l'erreur retournee par Postgres indique que la MV n'a jamais ete
//     peuplee (isNotPopulatedError), retomber sur un refresh NON-concurrent
//     (qui, lui, peuple la MV meme partant de "WITH NO DATA").
//
// Un booleen "premier passage" en memoire aurait ete plus simple a lire, mais
// plus FRAGILE : il redeviendrait faussement "true" a chaque redemarrage du
// worker, masquant une MV qui echouerait a se peupler pour une AUTRE raison
// recurrente (permissions, verrou, etc.) — l'inspection du message d'erreur
// Postgres reste correcte quel que soit l'etat memoire du worker, redemarrage
// ou non.
//
// Un echec de refresh (quel qu'il soit) ne tue JAMAIS le worker : log ERROR,
// nouvelle tentative au tick suivant.
import (
	"context"
	"strings"
	"time"
)

// notPopulatedErrSubstrings liste les fragments (compares en minuscules) des
// messages d'erreur Postgres signalant qu'une MV n'a jamais ete peuplee.
//
// CORRECTIF M-026 (BUG REEL, invisible en test unitaire) — IL Y A DEUX
// MESSAGES DISTINCTS, PAS UN SEUL. Avant ce correctif, la detection portait
// sur le SEUL fragment "has not been populated", qui est le message du
// **SELECT** sur une MV vide :
//
//	pq: materialized view "kpi_daily" has not been populated
//
// Or ce ticker n'execute jamais de SELECT : il execute un REFRESH, et Postgres
// repond alors avec une formulation TOUTE AUTRE ("is not populated", pas
// "has not been populated") :
//
//	pq: CONCURRENTLY cannot be used when the materialized view is not populated
//
// Le fragment recherche n'etant present dans AUCUN des deux (le premier
// intercale "been"), isNotPopulatedError renvoyait toujours false, le repli
// non-concurrent n'etait JAMAIS declenche, et la MV restait vide
// INDEFINIMENT : sur tout deploiement neuf, GET /analytics/kpis repondait 500
// en permanence, et ce ticker journalisait une ERROR a chaque intervalle sans
// jamais converger. Le test unitaire ne pouvait pas l'attraper : il mockait le
// message du SELECT pour exercer le chemin du REFRESH (voir
// analytics_refresh_test.go, corrige en meme temps).
//
// Les deux fragments ci-dessous ont ete **OBSERVES** contre un PostgreSQL 16
// reel pendant M-026 — ils ne sont pas repris de la documentation. Le second
// est conserve par prudence (formulation d'autres versions/chemins de code).
// Alternative plus robuste ecartee pour ne pas faire dependre ce worker de
// lib/pq : tester le SQLSTATE 55000 (object_not_in_prerequisite_state).
var notPopulatedErrSubstrings = []string{
	"is not populated",       // REFRESH ... CONCURRENTLY (le cas REEL de ce ticker)
	"has not been populated", // SELECT sur une MV vide (conserve par prudence)
}

// RunAnalyticsRefresh boucle jusqu'a annulation de ctx, executant
// refreshTick toutes les `interval`.
func (s *Service) RunAnalyticsRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.Logger.Info("intelligence-processor: analytics_refresh arret demande")
			return
		case <-ticker.C:
			s.refreshTick(ctx)
		}
	}
}

// refreshTick tente un REFRESH CONCURRENTLY, avec repli non-concurrent si la
// MV n'a jamais ete peuplee (D-M03, voir doc de tete de fichier).
func (s *Service) refreshTick(ctx context.Context) {
	if err := s.DB.Exec(ctx, `REFRESH MATERIALIZED VIEW CONCURRENTLY analytics.kpi_daily`); err != nil {
		if isNotPopulatedError(err) {
			s.Logger.Warn("intelligence-processor: kpi_daily jamais peuplee, refresh non-concurrent de secours (D-M03)",
				"err", err)
			if err2 := s.DB.Exec(ctx, `REFRESH MATERIALIZED VIEW analytics.kpi_daily`); err2 != nil {
				s.Logger.Error("intelligence-processor: refresh non-concurrent de secours echoue", "err", err2)
			} else {
				s.Logger.Info("intelligence-processor: kpi_daily peuplee (refresh non-concurrent)")
			}
			return
		}
		s.Logger.Error("intelligence-processor: refresh kpi_daily echoue", "err", err)
		return
	}
	s.Logger.Info("intelligence-processor: kpi_daily rafraichie (concurrently)")
}

// isNotPopulatedError est une fonction pure, testable isolement : detecte si
// err correspond a une MV jamais peuplee (cf. notPopulatedErrSubstring).
func isNotPopulatedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, sub := range notPopulatedErrSubstrings {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}
