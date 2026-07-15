-- +goose Up
-- Bearer token for the MCP endpoint (#92): LLM clients authenticate every
-- /mcp request with it — random, revocable, one per user, separate from
-- the calendar token so either can rotate alone.
ALTER TABLE users ADD COLUMN mcp_token text UNIQUE;

-- +goose Down
ALTER TABLE users DROP COLUMN mcp_token;
