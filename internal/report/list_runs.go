package report

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ListRunsFilter filters scenario test runs for list/count queries.
type ListRunsFilter struct {
	ProjectID  uuid.UUID
	ScenarioID *uuid.UUID
	ServiceID  *uuid.UUID
	Status     *RunStatus
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}

// DailyRunTrend is one day's run status counts for a scenario.
type DailyRunTrend struct {
	Date    string `json:"date"`
	Passed  int    `json:"passed"`
	Failed  int    `json:"failed"`
	Error   int    `json:"error"`
	Running int    `json:"running"`
	Queued  int    `json:"queued"`
	Total   int    `json:"total"`
}

// FailedStepStat aggregates failures for a scenario step.
type FailedStepStat struct {
	StepID    uuid.UUID `json:"stepId"`
	StepSeq   *int      `json:"stepSeq,omitempty"`
	StepName  string    `json:"stepName,omitempty"`
	StepType  string    `json:"stepType,omitempty"`
	FailCount int       `json:"failCount"`
}

// ListRunsPage is a paginated list of scenario runs.
type ListRunsPage struct {
	Items  []Run `json:"items"`
	Total  int   `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

// RunListEntry is a run row for list APIs with optional step summary.
type RunListEntry struct {
	Run
	Summary *RunSummary `json:"summary,omitempty"`
}

// ScenarioRunStats holds trend and top failure stats for a scenario.
type ScenarioRunStats struct {
	Trend          []DailyRunTrend  `json:"trend"`
	TopFailedSteps []FailedStepStat `json:"topFailedSteps"`
	TotalRuns      int              `json:"totalRuns"`
	PassRate       float64          `json:"passRate"`
}

func (r *Repository) ListRuns(ctx context.Context, filter ListRunsFilter) ([]Run, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	where, args := scenarioRunsWhere(filter)
	args = append(args, filter.Limit, filter.Offset)
	query := `
		select id, name, project_id, service_id, suite_id, scenario_id, environment_id, status,
			variables, snapshot, started_at, finished_at, created_at
		from test_runs
		where ` + where + `
		order by created_at desc
		limit $` + fmt.Sprint(len(args)-1) + ` offset $` + fmt.Sprint(len(args))
	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list scenario runs: %w", err)
	}
	defer rows.Close()
	var runs []Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *run)
	}
	return runs, rows.Err()
}

func (r *Repository) CountRuns(ctx context.Context, filter ListRunsFilter) (int, error) {
	if r.DB == nil {
		return 0, fmt.Errorf("database unavailable")
	}
	where, args := scenarioRunsWhere(filter)
	query := `select count(*) from test_runs where ` + where
	var total int
	if err := r.DB.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count scenario runs: %w", err)
	}
	return total, nil
}

func scenarioRunsWhere(filter ListRunsFilter) (string, []any) {
	parts := []string{
		"deleted_at is null",
		"scenario_id is not null",
		"project_id = $1",
	}
	args := []any{filter.ProjectID}
	if filter.ScenarioID != nil {
		args = append(args, *filter.ScenarioID)
		parts = append(parts, fmt.Sprintf("scenario_id = $%d", len(args)))
	}
	if filter.ServiceID != nil {
		args = append(args, *filter.ServiceID)
		parts = append(parts, fmt.Sprintf("service_id = $%d", len(args)))
	}
	if filter.Status != nil {
		args = append(args, *filter.Status)
		parts = append(parts, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.From != nil {
		args = append(args, *filter.From)
		parts = append(parts, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		parts = append(parts, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	return strings.Join(parts, " and "), args
}

func (r *Repository) ScenarioRunStats(ctx context.Context, projectID, scenarioID uuid.UUID, days int) (*ScenarioRunStats, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if days <= 0 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -days)

	trendRows, err := r.DB.Query(ctx, `
		select date_trunc('day', coalesce(finished_at, created_at))::date as day,
			status, count(*)::int
		from test_runs
		where project_id = $1 and scenario_id = $2 and deleted_at is null
		  and scenario_id is not null
		  and coalesce(finished_at, created_at) >= $3
		group by 1, 2
		order by 1 asc
	`, projectID, scenarioID, since)
	if err != nil {
		return nil, fmt.Errorf("scenario run trend: %w", err)
	}
	defer trendRows.Close()

	byDay := map[string]*DailyRunTrend{}
	var totalRuns, passedRuns int
	for trendRows.Next() {
		var day time.Time
		var status RunStatus
		var count int
		if err := trendRows.Scan(&day, &status, &count); err != nil {
			return nil, err
		}
		key := day.Format("2006-01-02")
		entry, ok := byDay[key]
		if !ok {
			entry = &DailyRunTrend{Date: key}
			byDay[key] = entry
		}
		entry.Total += count
		totalRuns += count
		switch status {
		case RunPassed:
			entry.Passed += count
			passedRuns += count
		case RunFailed:
			entry.Failed += count
		case RunRunning:
			entry.Running += count
		case RunQueued:
			entry.Queued += count
		default:
			entry.Error += count
		}
	}
	if err := trendRows.Err(); err != nil {
		return nil, err
	}

	trend := make([]DailyRunTrend, 0, len(byDay))
	for d := since.Truncate(24 * time.Hour); !d.After(time.Now()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		if entry, ok := byDay[key]; ok {
			trend = append(trend, *entry)
		} else {
			trend = append(trend, DailyRunTrend{Date: key})
		}
	}

	stepIndex, _ := r.LoadScenarioStepIndex(ctx, scenarioID)

	failRows, err := r.DB.Query(ctx, `
		select rr.step_id, count(*)::int
		from test_run_results rr
		join test_runs tr on tr.id = rr.run_id and tr.deleted_at is null
		where tr.project_id = $1 and tr.scenario_id = $2
		  and tr.scenario_id is not null
		  and coalesce(tr.finished_at, tr.created_at) >= $3
		  and rr.deleted_at is null
		  and rr.step_id is not null
		  and rr.status in ('failed', 'error')
		group by rr.step_id
		order by count(*) desc
		limit 10
	`, projectID, scenarioID, since)
	if err != nil {
		return nil, fmt.Errorf("top failed steps: %w", err)
	}
	defer failRows.Close()

	topFailed := []FailedStepStat{}
	for failRows.Next() {
		var stepID uuid.UUID
		var count int
		if err := failRows.Scan(&stepID, &count); err != nil {
			return nil, err
		}
		stat := FailedStepStat{StepID: stepID, FailCount: count}
		if meta, ok := stepIndex[stepID]; ok {
			seq := meta.StepSeq
			stat.StepSeq = &seq
			stat.StepName = meta.Name
			stat.StepType = meta.StepType
		}
		topFailed = append(topFailed, stat)
	}
	if err := failRows.Err(); err != nil {
		return nil, err
	}

	passRate := 0.0
	if totalRuns > 0 {
		passRate = float64(passedRuns) / float64(totalRuns)
	}
	return &ScenarioRunStats{
		Trend:          trend,
		TopFailedSteps: topFailed,
		TotalRuns:      totalRuns,
		PassRate:       passRate,
	}, nil
}

// BatchRunStepSummaries loads per-run step status counts for the given run IDs.
func (r *Repository) BatchRunStepSummaries(ctx context.Context, runIDs []uuid.UUID) (map[uuid.UUID]RunSummary, error) {
	out := make(map[uuid.UUID]RunSummary)
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if len(runIDs) == 0 {
		return out, nil
	}
	rows, err := r.DB.Query(ctx, `
		select run_id, status, count(*)::int
		from test_run_results
		where run_id = any($1) and deleted_at is null and step_id is not null
		group by run_id, status
	`, runIDs)
	if err != nil {
		return nil, fmt.Errorf("batch run step summaries: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var runID uuid.UUID
		var status ResultStatus
		var count int
		if err := rows.Scan(&runID, &status, &count); err != nil {
			return nil, err
		}
		entry := out[runID]
		entry.TotalSteps += count
		switch status {
		case ResultPassed:
			entry.Passed += count
		case ResultFailed:
			entry.Failed += count
		case ResultError:
			entry.Error += count
		}
		out[runID] = entry
	}
	return out, rows.Err()
}

// EnrichRunListEntries attaches step summaries to list run rows.
func EnrichRunListEntries(ctx context.Context, repo *Repository, runs []Run) ([]RunListEntry, error) {
	ids := make([]uuid.UUID, 0, len(runs))
	for _, run := range runs {
		ids = append(ids, run.ID)
	}
	summaries, err := repo.BatchRunStepSummaries(ctx, ids)
	if err != nil {
		return nil, err
	}
	entries := make([]RunListEntry, 0, len(runs))
	for _, run := range runs {
		entry := RunListEntry{Run: run}
		if sum, ok := summaries[run.ID]; ok && sum.TotalSteps > 0 {
			s := sum
			entry.Summary = &s
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
