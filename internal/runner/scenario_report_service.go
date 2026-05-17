package runner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"autotest/internal/report"
	"autotest/internal/scenario"

	"github.com/google/uuid"
)

func (s *Service) ListProjectRuns(ctx context.Context, filter report.ListRunsFilter) ([]report.RunListEntry, int, int, int, error) {
	if filter.ProjectID == uuid.Nil {
		return nil, 0, 0, 0, errors.New("projectId is required")
	}
	total, err := s.reports.CountRuns(ctx, filter)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	items, err := s.reports.ListRuns(ctx, filter)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	entries, err := report.EnrichRunListEntries(ctx, s.reports, items)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	return entries, total, limit, filter.Offset, nil
}

func (s *Service) ListScenarioRuns(ctx context.Context, scenarioRepo scenarioRepository, scenarioID uuid.UUID, filter report.ListRunsFilter) (*report.ListRunsPage, error) {
	sc, err := scenarioRepo.Get(ctx, scenarioID)
	if err != nil {
		return nil, err
	}
	if filter.ProjectID != uuid.Nil && sc.ProjectID != filter.ProjectID {
		return nil, scenario.ErrScenarioNotFound
	}
	filter.ProjectID = sc.ProjectID
	filter.ScenarioID = &scenarioID

	total, err := s.reports.CountRuns(ctx, filter)
	if err != nil {
		return nil, err
	}
	items, err := s.reports.ListRuns(ctx, filter)
	if err != nil {
		return nil, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	return &report.ListRunsPage{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: filter.Offset,
	}, nil
}

func (s *Service) GetScenarioRunReport(ctx context.Context, runID uuid.UUID) (*report.ScenarioRunReport, error) {
	rep, err := s.reports.GetScenarioRunReport(ctx, runID)
	if err != nil {
		return nil, err
	}
	return rep, nil
}

func (s *Service) ScenarioRunStats(ctx context.Context, scenarioRepo scenarioRepository, scenarioID uuid.UUID, projectID uuid.UUID, days int) (*report.ScenarioRunStats, error) {
	sc, err := scenarioRepo.Get(ctx, scenarioID)
	if err != nil {
		return nil, err
	}
	if projectID != uuid.Nil && sc.ProjectID != projectID {
		return nil, scenario.ErrScenarioNotFound
	}
	return s.reports.ScenarioRunStats(ctx, sc.ProjectID, scenarioID, days)
}

func (s *Service) ExportScenarioRun(ctx context.Context, runID uuid.UUID, format string) ([]byte, string, string, error) {
	rep, err := s.reports.GetScenarioRunReport(ctx, runID)
	if err != nil {
		return nil, "", "", err
	}
	switch format {
	case "html":
		data, err := report.ExportScenarioRunHTML(rep)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("scenario-run-%s.html", runID)
		return data, "text/html; charset=utf-8", filename, nil
	case "json", "":
		data, err := report.ExportJSON(rep)
		if err != nil {
			return nil, "", "", err
		}
		filename := fmt.Sprintf("scenario-run-%s.json", runID)
		return data, "application/json; charset=utf-8", filename, nil
	default:
		return nil, "", "", fmt.Errorf("unsupported format %q", format)
	}
}

func parseRunListFilter(r interface {
	URLQueryGet(string) string
}, projectID uuid.UUID) (report.ListRunsFilter, error) {
	filter := report.ListRunsFilter{ProjectID: projectID}
	if status := r.URLQueryGet("status"); status != "" {
		st := report.RunStatus(status)
		filter.Status = &st
	}
	if scenarioID := r.URLQueryGet("scenarioId"); scenarioID != "" {
		id, err := uuid.Parse(scenarioID)
		if err != nil {
			return filter, fmt.Errorf("invalid scenarioId")
		}
		filter.ScenarioID = &id
	}
	if from := r.URLQueryGet("from"); from != "" {
		t, err := parseFilterTime(from)
		if err != nil {
			return filter, err
		}
		filter.From = &t
	}
	if to := r.URLQueryGet("to"); to != "" {
		t, err := parseFilterTime(to)
		if err != nil {
			return filter, err
		}
		filter.To = &t
	}
	if pageSize := r.URLQueryGet("pageSize"); pageSize != "" {
		var n int
		if _, err := fmt.Sscanf(pageSize, "%d", &n); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if page := r.URLQueryGet("page"); page != "" {
		var p int
		if _, err := fmt.Sscanf(page, "%d", &p); err == nil && p > 0 {
			if filter.Limit <= 0 {
				filter.Limit = 20
			}
			filter.Offset = (p - 1) * filter.Limit
		}
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	return filter, nil
}

func parseFilterTime(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, nil
	}
	return time.Time{}, errors.New("invalid time, use RFC3339 or YYYY-MM-DD")
}
