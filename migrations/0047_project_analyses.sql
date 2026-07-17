-- +goose Up
CREATE TABLE project_analyses (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    payload    JSONB NOT NULL
);
CREATE INDEX idx_project_analyses_tenant_project_created
    ON project_analyses (tenant_id, project_id, created_at DESC, id COLLATE "C" DESC);

-- +goose Down
DROP TABLE project_analyses;
