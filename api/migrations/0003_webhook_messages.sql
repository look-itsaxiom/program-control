-- Inbound webhook messages, recorded so the same message delivered twice changes a task once.
-- The primary key is the sender's own message id, which is what makes the second delivery a
-- conflict the database refuses rather than a race the application has to win.

-- +goose Up

create table webhook_messages (
    id          text primary key,
    received_at timestamptz not null default now()
);

create index webhook_messages_received_at_idx on webhook_messages (received_at);

-- +goose Down

drop table if exists webhook_messages;
