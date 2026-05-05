package scriptlibrary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// pgx Row / Rows iterator both expose Scan.
type rowScanner interface {
	Scan(dest ...any) error
}

type Repository struct {
	store.Repository
}

func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

func (r *Repository) Create(ctx context.Context, input CreateInput) (*Template, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	scopesJSON, err := json.Marshal(normalizeScopes(input.Scopes))
	if err != nil {
		return nil, err
	}
	row := r.DB.QueryRow(ctx, `
		insert into script_library_templates (project_id, name, category, description, code, scopes)
		select p.id, $2, $3, $4, $5, $6::jsonb
		from projects p
		where p.id = $1 and p.deleted_at is null
		returning id, project_id, is_builtin, builtin_key, sort_order, name, category, description, code, scopes, created_at, updated_at
	`, input.ProjectID, input.Name, input.Category, nullStr(input.Description), input.Code, scopesJSON)
	t, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("project not found or inaccessible")
		}
		return nil, err
	}
	return t, nil
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, input UpdateInput) (*Template, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	scopesJSON, err := json.Marshal(normalizeScopes(input.Scopes))
	if err != nil {
		return nil, err
	}
	row := r.DB.QueryRow(ctx, `
		update script_library_templates t
		set name = $2,
		    category = $3,
		    description = $4,
		    code = $5,
		    scopes = $6::jsonb,
		    updated_at = now()
		where t.id = $1
		  and (
		      (t.is_builtin and t.project_id is null)
		      or exists (
		          select 1 from projects p
		          where p.id = t.project_id and p.deleted_at is null
		      )
		  )
		returning t.id, t.project_id, t.is_builtin, t.builtin_key, t.sort_order, t.name, t.category, t.description, t.code, t.scopes, t.created_at, t.updated_at
	`, id, input.Name, input.Category, nullStr(input.Description), input.Code, scopesJSON)
	t, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}
	return t, nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		delete from script_library_templates t
		using projects p
		where t.id = $1
		  and t.is_builtin = false
		  and t.project_id = p.id
		  and p.deleted_at is null
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTemplateNotFound
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Template, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select t.id, t.project_id, t.is_builtin, t.builtin_key, t.sort_order, t.name, t.category, t.description, t.code, t.scopes, t.created_at, t.updated_at
		from script_library_templates t
		left join projects p on p.id = t.project_id
		where t.id = $1
		  and (
		      (t.is_builtin and t.project_id is null)
		      or (p.id is not null and p.deleted_at is null)
		  )
	`, id)
	t, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}
	return t, nil
}

func (r *Repository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]Template, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select t.id, t.project_id, t.is_builtin, t.builtin_key, t.sort_order, t.name, t.category, t.description, t.code, t.scopes, t.created_at, t.updated_at
		from script_library_templates t
		left join projects p on p.id = t.project_id
		where (t.is_builtin and t.project_id is null)
		   or (t.project_id = $1 and p.id is not null and p.deleted_at is null)
		order by case when t.is_builtin then 0 else 1 end, t.sort_order asc, t.category asc, t.name asc, t.created_at asc
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list script templates: %w", err)
	}
	defer rows.Close()
	var out []Template
	for rows.Next() {
		tpl, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *tpl)
	}
	return out, rows.Err()
}

func scanTemplate(row rowScanner) (*Template, error) {
	var t Template
	var desc *string
	var scopesRaw []byte
	var projectID *uuid.UUID
	var builtinKey *string
	if err := row.Scan(
		&t.ID, &projectID, &t.IsBuiltin, &builtinKey, &t.SortOrder,
		&t.Name, &t.Category, &desc, &t.Code, &scopesRaw, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	t.ProjectID = projectID
	if builtinKey != nil {
		t.BuiltinKey = *builtinKey
	}
	if desc != nil {
		t.Description = *desc
	}
	if len(scopesRaw) > 0 {
		if err := json.Unmarshal(scopesRaw, &t.Scopes); err != nil {
			return nil, err
		}
	}
	return &t, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
