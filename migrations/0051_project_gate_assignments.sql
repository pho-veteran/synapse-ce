-- +goose Up
CREATE INDEX projects_assigned_gate_idx ON projects (tenant_id, gate_id) WHERE gate_id <> '';

-- +goose Down
DROP INDEX projects_assigned_gate_idx;
