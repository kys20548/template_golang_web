CREATE TABLE operation_logs (
    id bigserial PRIMARY KEY,
    user_id bigint,
    username varchar NOT NULL DEFAULT '',
    method varchar NOT NULL,
    path varchar NOT NULL,
    request_body text NOT NULL DEFAULT '',
    status_code int NOT NULL,
    request_id varchar NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT (now())
);

CREATE INDEX idx_operation_logs_user_id ON operation_logs (user_id);
CREATE INDEX idx_operation_logs_created_at ON operation_logs (created_at);
