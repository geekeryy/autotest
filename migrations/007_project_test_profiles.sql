-- +goose Up
CREATE TABLE IF NOT EXISTS project_test_profiles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    profile_data JSONB NOT NULL DEFAULT '{}',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_project_test_profiles_project ON project_test_profiles(project_id);
