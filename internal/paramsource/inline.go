package paramsource

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type InlineReference struct {
	Expression   string
	SourceKey    string
	Column       string
	FilterColumn string
	FilterValue  string
}

type inlineExecutorFunc func(context.Context, SourceWithDataSource, map[string]string) ([]map[string]string, ExecutionSnapshot, error)

func (s *Service) ExecuteInlineReferences(ctx context.Context, projectID, serviceID uuid.UUID, refs []InlineReference, vars map[string]string) (map[string]string, []ExecutionSnapshot, error) {
	if len(refs) == 0 {
		return map[string]string{}, nil, nil
	}
	sources, err := s.repo.ListExecutableSources(ctx, projectID, serviceID)
	if err != nil {
		return nil, nil, err
	}
	return executeInlineReferences(ctx, refs, vars, sources, s.executor.ExecuteRows)
}

func executeInlineReferences(ctx context.Context, refs []InlineReference, vars map[string]string, sources []SourceWithDataSource, execute inlineExecutorFunc) (map[string]string, []ExecutionSnapshot, error) {
	results := map[string]string{}
	snapshots := []ExecutionSnapshot{}
	rowsBySourceID := map[uuid.UUID][]map[string]string{}
	snapshotBySourceID := map[uuid.UUID]ExecutionSnapshot{}
	snapshotIndexBySourceID := map[uuid.UUID]int{}

	for _, ref := range dedupeInlineReferences(refs) {
		source, err := resolveInlineSource(ref.SourceKey, sources)
		if err != nil {
			return results, snapshots, err
		}
		rows, ok := rowsBySourceID[source.Source.ID]
		if !ok {
			var snapshot ExecutionSnapshot
			rows, snapshot, err = execute(ctx, source, vars)
			snapshots = append(snapshots, snapshot)
			if err != nil {
				return results, snapshots, err
			}
			rowsBySourceID[source.Source.ID] = rows
			snapshotBySourceID[source.Source.ID] = snapshot
			snapshotIndexBySourceID[source.Source.ID] = len(snapshots) - 1
		}

		value, err := resolveInlineValue(ref, rows)
		if err != nil {
			if snapshot, ok := snapshotBySourceID[source.Source.ID]; ok {
				snapshot.Error = err.Error()
				snapshotBySourceID[source.Source.ID] = snapshot
				if idx, ok := snapshotIndexBySourceID[source.Source.ID]; ok {
					snapshots[idx] = snapshot
				}
			}
			return results, snapshots, err
		}
		results[ref.Expression] = value
	}
	return results, snapshots, nil
}

func dedupeInlineReferences(refs []InlineReference) []InlineReference {
	seen := map[string]struct{}{}
	out := make([]InlineReference, 0, len(refs))
	for _, ref := range refs {
		if ref.Expression == "" {
			continue
		}
		if _, ok := seen[ref.Expression]; ok {
			continue
		}
		seen[ref.Expression] = struct{}{}
		out = append(out, ref)
	}
	return out
}

func resolveInlineSource(sourceKey string, sources []SourceWithDataSource) (SourceWithDataSource, error) {
	var matches []SourceWithDataSource
	for _, source := range sources {
		if inlineSourceMatches(source.Source, sourceKey) {
			matches = append(matches, source)
		}
	}
	if len(matches) == 0 {
		return SourceWithDataSource{}, fmt.Errorf("sql parameter source %q not found for inline reference", sourceKey)
	}
	if len(matches) > 1 {
		return SourceWithDataSource{}, fmt.Errorf("sql parameter source %q is ambiguous for inline reference", sourceKey)
	}
	return matches[0], nil
}

func inlineSourceMatches(source SQLParameterSource, sourceKey string) bool {
	sourceKey = strings.TrimSpace(sourceKey)
	if sourceKey == "" {
		return false
	}
	for _, candidate := range []string{source.Key, source.Name, source.ID.String()} {
		if strings.TrimSpace(candidate) == sourceKey {
			return true
		}
	}
	return false
}

func resolveInlineValue(ref InlineReference, rows []map[string]string) (string, error) {
	row, err := selectInlineRow(ref, rows)
	if err != nil {
		return "", err
	}
	value, ok := rowValue(row, ref.Column)
	if !ok {
		return "", fmt.Errorf("sql inline reference %q column %q not found", ref.Expression, ref.Column)
	}
	return value, nil
}

func selectInlineRow(ref InlineReference, rows []map[string]string) (map[string]string, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("sql inline reference %q returned no rows", ref.Expression)
	}
	if ref.FilterColumn == "" {
		return rows[0], nil
	}
	for _, row := range rows {
		value, ok := rowValue(row, ref.FilterColumn)
		if ok && value == ref.FilterValue {
			return row, nil
		}
	}
	return nil, fmt.Errorf("sql inline reference %q found no row where %s=%s", ref.Expression, ref.FilterColumn, ref.FilterValue)
}

func rowValue(row map[string]string, column string) (string, bool) {
	if value, ok := row[column]; ok {
		return value, true
	}
	lower := strings.ToLower(column)
	for key, value := range row {
		if strings.ToLower(key) == lower {
			return value, true
		}
	}
	return "", false
}
