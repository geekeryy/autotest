package scenario

import (
	"context"
	"errors"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	store.Repository
}

func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// ── Scenario CRUD ─────────────────────────────────────────────────────────

func (r *Repository) Create(ctx context.Context, input CreateScenarioInput) (*Scenario, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		insert into test_scenarios (project_id, service_id, name, description)
		select p.id, s.id, $3, $4
		from projects p
		join services s on s.project_id = p.id and s.id = $2 and s.deleted_at is null
		where p.id = $1 and p.deleted_at is null
		returning id, project_id, service_id, name, description, created_at, updated_at
	`, input.ProjectID, input.ServiceID, input.Name, input.Description)
	return scanScenario(row)
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, input UpdateScenarioInput) (*Scenario, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		update test_scenarios
		set name = $2, description = $3, updated_at = now()
		where id = $1 and deleted_at is null
		returning id, project_id, service_id, name, description, created_at, updated_at
	`, id, input.Name, input.Description)
	sc, err := scanScenario(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrScenarioNotFound
		}
		return nil, err
	}
	return sc, nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update test_scenarios set deleted_at = now() where id = $1 and deleted_at is null
	`, id)
	if err != nil {
		return fmt.Errorf("delete scenario: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrScenarioNotFound
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Scenario, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select id, project_id, service_id, name, description, created_at, updated_at
		from test_scenarios
		where id = $1 and deleted_at is null
	`, id)
	sc, err := scanScenario(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrScenarioNotFound
		}
		return nil, err
	}
	return sc, nil
}

func (r *Repository) List(ctx context.Context, filter ListFilter) ([]Scenario, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select id, project_id, service_id, name, description, created_at, updated_at
		from test_scenarios
		where deleted_at is null
		  and ($1::uuid is null or project_id = $1)
		  and ($2::uuid is null or service_id = $2)
		order by created_at asc
	`, nullUUID(filter.ProjectID), nullUUID(filter.ServiceID))
	if err != nil {
		return nil, fmt.Errorf("list scenarios: %w", err)
	}
	defer rows.Close()
	var out []Scenario
	for rows.Next() {
		sc, err := scanScenario(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sc)
	}
	return out, rows.Err()
}

// ── Steps ─────────────────────────────────────────────────────────────────

func (r *Repository) UpsertStep(ctx context.Context, scenarioID uuid.UUID, input UpsertStepInput) (*Step, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	stepType := input.StepType
	if stepType == "" {
		stepType = StepTypeAPI
	}
	config := input.Config
	if len(config) == 0 {
		config = []byte(`{}`)
	}
	reqOverride := input.RequestOverride
	if len(reqOverride) == 0 {
		reqOverride = []byte(`{}`)
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin upsert step: %w", err)
	}
	defer tx.Rollback(ctx)

	// 软删除的步骤仍占用 (scenario_id, step_order) 的 unique 槽位，若不先清理，
	// 后续 INSERT ... ON CONFLICT 会更新到已软删除行上，但不会重置 deleted_at，
	// 表现为「保存成功但列表查不到、刷新页面后也看不到」。这里在事务内先把
	// 同 (scenario_id, step_order) 的软删除占位行物理清理掉，使后续 INSERT 走真正的
	// 新行路径（带新的 step_seq、新的 id），符合软删除的语义。
	if _, err := tx.Exec(ctx, `
		delete from test_scenario_steps
		where scenario_id = $1 and step_order = $2 and deleted_at is not null
	`, scenarioID, input.StepOrder); err != nil {
		return nil, fmt.Errorf("cleanup soft-deleted step placeholder: %w", err)
	}

	// step_seq is assigned only on INSERT (the subquery computes the next value).
	// On conflict (same scenario_id + step_order with deleted_at IS NULL), the existing
	// step_seq is preserved.
	row := tx.QueryRow(ctx, `
		insert into test_scenario_steps
			(scenario_id, test_case_id, step_seq, step_order, step_type, name, enabled,
			 config, request_override)
		values ($1, $2,
			coalesce((select max(step_seq) from test_scenario_steps where scenario_id = $1), 0) + 1,
			$3, $4, $5, $6, $7, $8)
		on conflict (scenario_id, step_order) do update
		set test_case_id     = excluded.test_case_id,
		    step_type        = excluded.step_type,
		    name             = excluded.name,
		    enabled          = excluded.enabled,
		    config           = excluded.config,
		    request_override = excluded.request_override,
		    updated_at       = now()
		returning id, scenario_id, test_case_id, step_seq, step_order, step_type, name, enabled,
		          config, request_override, created_at, updated_at
	`, scenarioID, nullUUID(input.TestCaseID), input.StepOrder, stepType, input.Name, enabled,
		config, reqOverride)

	step, err := scanStep(row)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit upsert step: %w", err)
	}
	return step, nil
}

// ReorderSteps assigns step_order 1..n to rows in the given order (by step id).
// Uses a two-phase update so (scenario_id, step_order) stays unique.
// step_seq is never modified here — it is a permanent identifier.
func (r *Repository) ReorderSteps(ctx context.Context, scenarioID uuid.UUID, orderedStepIDs []uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	if len(orderedStepIDs) == 0 {
		return nil
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reorder steps: %w", err)
	}
	defer tx.Rollback(ctx)

	var existing []uuid.UUID
	rows, err := tx.Query(ctx, `
		select id from test_scenario_steps
		where scenario_id = $1 and deleted_at is null
	`, scenarioID)
	if err != nil {
		return fmt.Errorf("list step ids: %w", err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan step id: %w", err)
		}
		existing = append(existing, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("list step ids: %w", err)
	}
	rows.Close()

	if len(existing) != len(orderedStepIDs) {
		return fmt.Errorf("stepIds length does not match scenario steps")
	}
	want := make(map[uuid.UUID]struct{}, len(existing))
	for _, id := range existing {
		want[id] = struct{}{}
	}
	for _, id := range orderedStepIDs {
		if _, ok := want[id]; !ok {
			return fmt.Errorf("invalid or duplicate step id in stepIds")
		}
		delete(want, id)
	}
	if len(want) != 0 {
		return fmt.Errorf("stepIds must list every step exactly once")
	}

	var minOrder int
	if err := tx.QueryRow(ctx, `
		select coalesce(min(step_order), 1)
		from test_scenario_steps
		where scenario_id = $1
	`, scenarioID).Scan(&minOrder); err != nil {
		return fmt.Errorf("load minimum step order: %w", err)
	}
	scratchBase := minOrder - len(existing) - 1
	if _, err := tx.Exec(ctx, `
		with ranked as (
			select id, row_number() over (order by id)::integer as rn
			from test_scenario_steps
			where scenario_id = $1 and deleted_at is null
		)
		update test_scenario_steps s
		set step_order = $2 + ranked.rn,
		    updated_at = now()
		from ranked
		where s.id = ranked.id
	`, scenarioID, scratchBase); err != nil {
		return fmt.Errorf("shift step orders: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		delete from test_scenario_steps
		where scenario_id = $1 and deleted_at is not null and step_order between 1 and $2
	`, scenarioID, len(orderedStepIDs)); err != nil {
		return fmt.Errorf("cleanup soft-deleted step order placeholders: %w", err)
	}

	for i, id := range orderedStepIDs {
		tag, err := tx.Exec(ctx, `
			update test_scenario_steps
			set step_order = $2, updated_at = now()
			where id = $1 and scenario_id = $3 and deleted_at is null
		`, id, i+1, scenarioID)
		if err != nil {
			return fmt.Errorf("set step order: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrStepNotFound
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reorder steps: %w", err)
	}
	return nil
}

func (r *Repository) DeleteStep(ctx context.Context, stepID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update test_scenario_steps set deleted_at = now() where id = $1 and deleted_at is null
	`, stepID)
	if err != nil {
		return fmt.Errorf("delete scenario step: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrStepNotFound
	}
	return nil
}

func (r *Repository) ListSteps(ctx context.Context, scenarioID uuid.UUID) ([]Step, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select id, scenario_id, test_case_id, step_seq, step_order, step_type, name, enabled,
		       config, request_override, created_at, updated_at
		from test_scenario_steps
		where scenario_id = $1 and deleted_at is null
		order by step_order asc
	`, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("list scenario steps: %w", err)
	}
	defer rows.Close()
	var out []Step
	for rows.Next() {
		step, err := scanStep(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *step)
	}
	return out, rows.Err()
}

// ── Scanners ──────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanScenario(row rowScanner) (*Scenario, error) {
	var sc Scenario
	if err := row.Scan(
		&sc.ID, &sc.ProjectID, &sc.ServiceID,
		&sc.Name, &sc.Description,
		&sc.CreatedAt, &sc.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan scenario: %w", err)
	}
	return &sc, nil
}

func scanStep(row rowScanner) (*Step, error) {
	var s Step
	var testCaseID *uuid.UUID
	var rawReqOverride []byte
	var rawConfig []byte
	if err := row.Scan(
		&s.ID, &s.ScenarioID, &testCaseID,
		&s.StepSeq, &s.StepOrder, &s.StepType, &s.Name, &s.Enabled,
		&rawConfig, &rawReqOverride,
		&s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan scenario step: %w", err)
	}
	if testCaseID != nil {
		s.TestCaseID = *testCaseID
	}
	if s.StepType == "" {
		s.StepType = StepTypeAPI
	}
	if len(rawConfig) > 0 && string(rawConfig) != "{}" {
		s.Config = rawConfig
	}
	if len(rawReqOverride) > 0 && string(rawReqOverride) != "{}" {
		s.RequestOverride = rawReqOverride
	}
	return &s, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────

func nullUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
