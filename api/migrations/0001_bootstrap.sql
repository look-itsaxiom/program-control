-- 0001_bootstrap.sql is provided as part of the skeleton. It exists only to prove the migration
-- tool works end to end on a fresh database; it makes no assumptions about your schema.
--
-- goose reserves version 0 as "nothing applied yet", so this provided migration is 0001 rather
-- than 0000. Your own migrations start at 0002. Use `go run ./cmd/migrate create <name>` from
-- api/ to scaffold the next one.

-- +goose Up
create extension if not exists pgcrypto;

-- +goose Down
drop extension if exists pgcrypto;
