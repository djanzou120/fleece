package intelligenceprocessor

// analytics_refresh.go — ticker REFRESH MATERIALIZED VIEW analytics.kpi_daily.
//
// DETTE D-M03 (voir .ia/MIGRATION_PLAN.md) GEREE EXPLICITEMENT ICI : la MV
// analytics.kpi_daily est creee "WITH NO DATA" (migration 0017), et
// PostgreSQL EXIGE que la MV ait deja ete peuplee au moins une fois pour
// accepter l'option CONCURRENTLY (erreur retournee sinon : "materialized view
// ... has not been populated"). Strategie retenue :
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

// notPopulatedErrSubstring est le fragment (insensible a la casse) du message
// d'erreur Postgres signalant qu'une MV CONCURRENTLY-refreshable n'a jamais
// ete peuplee. Constaté par la documentation Postgres (REFRESH MATERIALIZED
// VIEW) : "cannot refresh materialized view ... concurrently" /
// "has not been populated".
const notPopulatedErrSubstring = "has not been populated"

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
	return err != nil && strings.Contains(strings.ToLower(err.Error()), notPopulatedErrSubstring)
}
