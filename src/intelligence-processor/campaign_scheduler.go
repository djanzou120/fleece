package intelligenceprocessor

// campaign_scheduler.go — ticker qui demarre les campagnes 'scheduled' dont
// scheduled_at est echue.
//
// ANTI-DOUBLE-RUN (vigilance T-017, critere de QA M-022) : la selection ET le
// marquage sont UNE SEULE requete "UPDATE ... RETURNING" — JAMAIS un SELECT
// suivi d'un UPDATE separe. Deux replicas de ce worker (ou deux tickers qui
// se chevauchent) executant CE MEME UPDATE en concurrence ne peuvent jamais
// selectionner deux fois la meme campagne : PostgreSQL verrouille chaque
// ligne candidate au niveau de l'UPDATE, la deuxieme transaction concurrente
// ne voit alors plus la ligne (status='running' deja applique par la
// premiere) et RETURNING ne lui renvoie rien pour cette campagne. L'index
// (status, scheduled_at) de la migration 0016 sert exactement ce pattern de
// lecture/ecriture.
//
// CONTRAT "campaign.run" FIGE PAR CE WORKER (voir on_campaign_run.go, cote
// consommateur) : {"campaign_id":"...", "run_id":"..."} — run_id est
// campaign.campaign_runs.id (bigserial), encode en decimal string.
import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// errPublisherUnavailable est retournee par publishCampaignRun quand s.AMQP
// est nil (jamais en production — le composition root exige AMQP_DSN, voir
// main.go — mais possible en test).
var errPublisherUnavailable = errors.New("campaign_scheduler: AMQP non configure")

// scheduledCampaignRow est une campagne capturee par l'UPDATE atomique
// ci-dessous (colonnes issues de la migration 0016).
type scheduledCampaignRow struct {
	ID              string `db:"id"`
	WorkspaceID     string `db:"workspace_id"`
	MessageBody     string `db:"message_body"`
	ChannelStrategy string `db:"channel_strategy"`
	RateLimit       int    `db:"rate_limit"`
}

// RunCampaignScheduler boucle jusqu'a annulation de ctx, executant
// runSchedulerTick toutes les `interval`. Le premier tick n'est declenche
// qu'apres le premier ecoulement de `interval` (time.Ticker standard) : le
// composition root (cmd/intelligence-processor/main.go) ne demarre cette
// goroutine qu'apres Init() complet du Service, donc "le worker est pret"
// avant que le premier tick puisse jamais se produire.
func (s *Service) RunCampaignScheduler(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.Logger.Info("intelligence-processor: campaign_scheduler arret demande")
			return
		case <-ticker.C:
			s.runSchedulerTick(ctx)
		}
	}
}

// runSchedulerTick execute un cycle du scheduler : capture atomique des
// campagnes echues, puis demarrage d'un run par campagne capturee. Un echec
// de tick (panne DB) est loggue ERROR mais ne tue jamais le worker — le tick
// suivant retentera.
func (s *Service) runSchedulerTick(ctx context.Context) {
	var rows []scheduledCampaignRow
	err := s.DB.Select(ctx, &rows,
		`UPDATE campaign.campaigns SET status = 'running'
		   WHERE status = 'scheduled' AND scheduled_at <= now()
		 RETURNING id, workspace_id, message_body, channel_strategy, rate_limit`,
	)
	if err != nil {
		s.Logger.Error("intelligence-processor: campaign_scheduler tick", "err", err)
		return
	}

	for _, row := range rows {
		if err := s.startCampaignRun(ctx, row); err != nil {
			s.Logger.Error("intelligence-processor: demarrage du run", "campaign_id", row.ID, "err", err)
		}
	}
}

// startCampaignRun cree la ligne campaign.campaign_runs (total_count =
// nombre de destinataires 'pending' au lancement) puis publie "campaign.run".
func (s *Service) startCampaignRun(ctx context.Context, row scheduledCampaignRow) error {
	var count struct {
		Count int64 `db:"count"`
	}
	if err := s.DB.Get(ctx, &count,
		`SELECT COUNT(*) AS count FROM campaign.campaign_recipients WHERE campaign_id = $1 AND status = 'pending'`,
		row.ID,
	); err != nil {
		return err
	}

	var inserted struct {
		ID int64 `db:"id"`
	}
	if err := s.DB.Get(ctx, &inserted,
		`INSERT INTO campaign.campaign_runs (campaign_id, started_at, total_count)
		 VALUES ($1, now(), $2)
		 RETURNING id`,
		row.ID, count.Count,
	); err != nil {
		return err
	}

	s.Logger.Info("intelligence-processor: run de campagne demarre",
		"campaign_id", row.ID, "run_id", inserted.ID, "total_count", count.Count)

	if err := s.publishCampaignRun(ctx, row.ID, inserted.ID); err != nil {
		// Le run est deja persiste en base (status='running' + ligne
		// campaign_runs creee) MEME si la publication AMQP echoue : on ne
		// rollback pas cet etat (ce n'est pas une transaction unique — les
		// operations DB precedentes sont deja commitees individuellement).
		// Limite documentee : sans republication manuelle de "campaign.run"
		// pour ce run_id, ce run ne sera jamais traite par
		// on_campaign_run.go (voir rapport de tache M-021).
		s.Logger.Error("intelligence-processor: publication campaign.run echouee, run cree mais non traite",
			"campaign_id", row.ID, "run_id", inserted.ID, "err", err)
		return err
	}
	return nil
}

// publishCampaignRun publie l'evenement "campaign.run" sur l'exchange
// "fleece". Si s.AMQP est nil (tests, ou configuration degradee), retourne
// une erreur explicite sans jamais paniquer.
func (s *Service) publishCampaignRun(ctx context.Context, campaignID string, runID int64) error {
	if s.AMQP == nil {
		return errPublisherUnavailable
	}
	body, err := json.Marshal(campaignRunEvent{
		CampaignID: campaignID,
		RunID:      strconv.FormatInt(runID, 10),
	})
	if err != nil {
		return err
	}
	return s.AMQP.Publish(ctx, "fleece", "campaign.run", body)
}
