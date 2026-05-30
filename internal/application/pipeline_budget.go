package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/farxc/envelopa-transparencia/internal/domain/model"
	"github.com/farxc/envelopa-transparencia/internal/domain/service"
	"github.com/farxc/envelopa-transparencia/internal/infrastructure/filesystem"
	"github.com/farxc/envelopa-transparencia/internal/infrastructure/logger"
	"github.com/farxc/envelopa-transparencia/internal/infrastructure/store"
	"github.com/lib/pq"
)

// BudgetPipeline implements Pipeline[model.BudgetJob].
// It handles annual budget allocation (LOA) data from the orcamento-despesa endpoint.
type BudgetPipeline struct {
	client    service.TransparencyPortalClient
	loader    service.Loader
	appLogger *logger.Logger
}

func NewBudgetPipeline(
	client service.TransparencyPortalClient,
	loader service.Loader,
	appLogger *logger.Logger,
) *BudgetPipeline {
	return &BudgetPipeline{
		client:    client,
		loader:    loader,
		appLogger: appLogger,
	}
}

func (p *BudgetPipeline) Execute(ctx context.Context, job model.BudgetJob) error {
	// 1. Download if not already present
	zipPath := "tmp/zips/budget/" + job.Year + "_OrcamentoDespesa.zip"
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		download := p.client.FetchBudget(job.Year)
		if !download.Success {
			return fmt.Errorf("download failed for year %s", job.Year)
		}
	}

	// 2. Unzip
	outputDir := "tmp/data/budget_" + job.Year
	extraction := filesystem.UnzipFile(zipPath, outputDir, p.appLogger)
	if !extraction.Success {
		return fmt.Errorf("extraction failed for year %s", job.Year)
	}

	// 3. Build extraction config
	codeStrings := make([]string, len(job.Codes))
	for i, c := range job.Codes {
		codeStrings[i] = fmt.Sprintf("%d", c)
	}

	csvFile := filepath.Join(extraction.OutputDir, job.Year+service.OrcamentoDespesaCSVSuffix)

	cfg := service.BudgetExtractionConfig{
		Codes: codeStrings,
		File:  csvFile,
		Year:  job.Year,
	}

	// 4. Extract
	payload, err := p.client.ExtractBudget(cfg)
	if err != nil {
		return err
	}

	// 5. Load
	return p.loader.LoadExpenseBudget(ctx, payload)
}

func (p *BudgetPipeline) BuildHistoryRecord(job model.BudgetJob) *model.IngestionHistory {
	yearInt, _ := strconv.Atoi(job.Year)
	refDate := time.Date(yearInt, 1, 1, 0, 0, 0, 0, time.UTC)

	return &model.IngestionHistory{
		ReferenceDate:  refDate,
		TriggerType:    job.Trigger,
		ScopeType:      store.ScopeTypeManagingUnit,
		SourceFile:     fmt.Sprintf("%s_OrcamentoDespesa.zip", job.Year),
		ProcessedCodes: pq.Int64Array(job.Codes),
	}
}

func (p *BudgetPipeline) ShouldSkip(err error, job model.BudgetJob) bool {
	return err.Error() == "dataframe is empty"
}

func (p *BudgetPipeline) StatusKey(job model.BudgetJob) string {
	return job.Year
}

func (p *BudgetPipeline) HistoryKey(h model.IngestionHistory) string {
	return h.ReferenceDate.Format("2006")
}

func (p *BudgetPipeline) HistoryRange(startDate, endDate time.Time) (time.Time, time.Time) {
	start := time.Date(startDate.Year(), 1, 1, 0, 0, 0, 0, startDate.Location())
	end := time.Date(endDate.Year(), 1, 1, 0, 0, 0, 0, endDate.Location())
	return start, end
}

func (p *BudgetPipeline) Kind() string { return "budget" }
