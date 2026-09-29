-- Core domain schema: organizations and the people in them, programs and who they are shared
-- with, the tasks inside a program, the dependency edges between tasks, and the audit trail.

-- +goose Up

create table organizations (
    id         uuid primary key default gen_random_uuid(),
    name       text        not null unique,
    created_at timestamptz not null default now()
);

create table users (
    id           uuid primary key default gen_random_uuid(),
    email        text not null unique,
    display_name text not null
);

create table memberships (
    user_id uuid not null references users (id) on delete cascade,
    org_id  uuid not null references organizations (id) on delete cascade,
    role    text not null check (role in ('owner', 'admin', 'member', 'viewer')),
    primary key (user_id, org_id)
);

-- An organization that still owns a program cannot be deleted (on delete restrict), but an
-- organization that only appears on the sharing side goes away cleanly (on delete cascade in
-- program_shares below).
create table programs (
    id         uuid primary key default gen_random_uuid(),
    org_id     uuid        not null references organizations (id) on delete restrict,
    name       text        not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz
);

-- Program names are unique among the still-active programs of one organization, so a name freed
-- up by a soft delete can be reused. A partial unique index says exactly that; a table-level
-- unique constraint could not.
create unique index programs_active_name_per_org_idx
    on programs (org_id, name)
    where deleted_at is null;

create table program_shares (
    program_id uuid not null references programs (id) on delete cascade,
    org_id     uuid not null references organizations (id) on delete cascade,
    role       text not null check (role in ('editor', 'viewer')),
    primary key (program_id, org_id)
);

create table tasks (
    id               uuid primary key default gen_random_uuid(),
    program_id       uuid        not null references programs (id) on delete cascade,
    org_id           uuid        not null references organizations (id) on delete restrict,
    title            text        not null check (char_length(title) between 1 and 200),
    status           text        not null default 'todo'
                     check (status in ('todo', 'doing', 'blocked', 'done')),
    estimate_minutes integer     check (estimate_minutes >= 0),
    assignee_id      uuid        references users (id) on delete set null,
    due_on           date,
    internal_note    text,
    created_at       timestamptz not null default now(),
    updated_at       timestamptz not null default now(),
    deleted_at       timestamptz
);

create unique index tasks_active_title_per_program_idx
    on tasks (program_id, title)
    where deleted_at is null;

-- The board query filters one program to one status, skips soft-deleted rows, and orders by due
-- date. Leading the index with (program_id, status) and trailing it with due_on lets Postgres
-- seek to the matching slice and read it already ordered, so a LIMIT stops early.
create index tasks_board_idx
    on tasks (program_id, status, due_on)
    where deleted_at is null;

create index tasks_assignee_idx on tasks (assignee_id) where assignee_id is not null;

create table task_dependencies (
    predecessor_id uuid not null references tasks (id) on delete cascade,
    successor_id   uuid not null references tasks (id) on delete cascade,
    primary key (predecessor_id, successor_id),
    constraint task_dependencies_no_self_edge check (predecessor_id <> successor_id)
);

create index task_dependencies_successor_idx on task_dependencies (successor_id);

create table task_events (
    id          bigint generated always as identity primary key,
    task_id     uuid        not null references tasks (id) on delete cascade,
    actor_id    uuid        references users (id) on delete set null,
    kind        text        not null,
    from_status text,
    to_status   text,
    created_at  timestamptz not null default now()
);

create index task_events_task_idx on task_events (task_id, id);

-- updated_at maintains itself, so no caller can forget it and quietly produce a wrong trail.
-- +goose StatementBegin
create function set_updated_at() returns trigger
language plpgsql
as $$
begin
    new.updated_at := now();
    return new;
end;
$$;
-- +goose StatementEnd

create trigger programs_set_updated_at
    before update on programs
    for each row execute function set_updated_at();

create trigger tasks_set_updated_at
    before update on tasks
    for each row execute function set_updated_at();

-- +goose Down

drop trigger if exists tasks_set_updated_at on tasks;
drop trigger if exists programs_set_updated_at on programs;
drop function if exists set_updated_at();
drop table if exists task_events;
drop table if exists task_dependencies;
drop table if exists tasks;
drop table if exists program_shares;
drop table if exists programs;
drop table if exists memberships;
drop table if exists users;
drop table if exists organizations;
