package mockrecord

import (
	"context"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository persists recorded responses.
type Repository struct {
	store.Repository
}

// NewRepository creates a Repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// InsertRecording inserts a new recorded response.
func (r *Repository) InsertRecording(ctx context.Context, rec *RecordedResponse) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	_, err := r.DB.Exec(ctx, `
		INSERT INTO mock_recordings (
			id, project_id, mock_server_id, mock_route_id,
			method, path, query_hash, request_body,
			status_code, headers, body, recorded_at, hit_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, rec.ID, rec.ProjectID, rec.MockServerID, rec.MockRouteID,
		rec.Method, rec.Path, rec.QueryHash, rec.RequestBody,
		rec.StatusCode, rec.Headers, rec.Body, rec.RecordedAt, rec.HitCount)
	if err != nil {
		return fmt.Errorf("insert recording: %w", err)
	}
	return nil
}

// FindExactMatch 查找精确匹配的录制数据（Route + Method + Path + QueryHash）。
func (r *Repository) FindExactMatch(ctx context.Context, serverID, routeID uuid.UUID, method, path, queryHash string) (*RecordedResponse, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		SELECT id, project_id, mock_server_id, mock_route_id,
			method, path, query_hash, request_body,
			status_code, headers, body, recorded_at, hit_count
		FROM mock_recordings
		WHERE mock_server_id = $1 AND mock_route_id = $2
		  AND method = $3 AND path = $4 AND query_hash = $5
		ORDER BY recorded_at DESC
		LIMIT 1
	`, serverID, routeID, method, path, queryHash)
	return scanRecording(row)
}

// FindFuzzyMatch 查找模糊匹配的录制数据（Route + Method + Path，忽略 query）。
func (r *Repository) FindFuzzyMatch(ctx context.Context, serverID, routeID uuid.UUID, method, path string) (*RecordedResponse, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		SELECT id, project_id, mock_server_id, mock_route_id,
			method, path, query_hash, request_body,
			status_code, headers, body, recorded_at, hit_count
		FROM mock_recordings
		WHERE mock_server_id = $1 AND mock_route_id = $2
		  AND method = $3 AND path = $4
		ORDER BY recorded_at DESC
		LIMIT 1
	`, serverID, routeID, method, path)
	return scanRecording(row)
}

// ListByRoute 列出路由的所有录制数据。
func (r *Repository) ListByRoute(ctx context.Context, serverID, routeID uuid.UUID) ([]RecordedResponse, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		SELECT id, project_id, mock_server_id, mock_route_id,
			method, path, query_hash, request_body,
			status_code, headers, body, recorded_at, hit_count
		FROM mock_recordings
		WHERE mock_server_id = $1 AND mock_route_id = $2
		ORDER BY recorded_at DESC
	`, serverID, routeID)
	if err != nil {
		return nil, fmt.Errorf("list recordings: %w", err)
	}
	defer rows.Close()
	return scanRecordings(rows)
}

// DeleteByID 删除一条录制数据。
func (r *Repository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `DELETE FROM mock_recordings WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete recording: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("recording not found")
	}
	return nil
}

// ClearByRoute 清空路由的所有录制数据。
func (r *Repository) ClearByRoute(ctx context.Context, serverID, routeID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	_, err := r.DB.Exec(ctx, `
		DELETE FROM mock_recordings
		WHERE mock_server_id = $1 AND mock_route_id = $2
	`, serverID, routeID)
	if err != nil {
		return fmt.Errorf("clear recordings: %w", err)
	}
	return nil
}

// IncrementHitCount 递增命中次数。
func (r *Repository) IncrementHitCount(ctx context.Context, id uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	_, err := r.DB.Exec(ctx, `
		UPDATE mock_recordings SET hit_count = hit_count + 1 WHERE id = $1
	`, id)
	return err
}

func scanRecording(row pgx.Row) (*RecordedResponse, error) {
	var rec RecordedResponse
	err := row.Scan(
		&rec.ID, &rec.ProjectID, &rec.MockServerID, &rec.MockRouteID,
		&rec.Method, &rec.Path, &rec.QueryHash, &rec.RequestBody,
		&rec.StatusCode, &rec.Headers, &rec.Body, &rec.RecordedAt, &rec.HitCount,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

func scanRecordings(rows pgx.Rows) ([]RecordedResponse, error) {
	var recordings []RecordedResponse
	for rows.Next() {
		var rec RecordedResponse
		if err := rows.Scan(
			&rec.ID, &rec.ProjectID, &rec.MockServerID, &rec.MockRouteID,
			&rec.Method, &rec.Path, &rec.QueryHash, &rec.RequestBody,
			&rec.StatusCode, &rec.Headers, &rec.Body, &rec.RecordedAt, &rec.HitCount,
		); err != nil {
			return nil, fmt.Errorf("scan recording: %w", err)
		}
		recordings = append(recordings, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if recordings == nil {
		return []RecordedResponse{}, nil
	}
	return recordings, nil
}
