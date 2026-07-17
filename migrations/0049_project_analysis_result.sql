-- +goose Up
ALTER TABLE project_analyses ADD COLUMN result JSONB;

-- +goose Down
ALTER TABLE project_analyses DROP COLUMN result;
