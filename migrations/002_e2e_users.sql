-- E2E 测试用表与种子数据；与主业务 schema 分离，便于生产环境跳过本文件

-- +goose Up
create table if not exists e2e_users (
    id         uuid        primary key default gen_random_uuid(),
    name       text        not null,
    email      text        not null,
    role       text        not null default 'tester',
    active     boolean     not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

-- 预置初始用户，供 e2e 测试使用，固定 UUID 便于重复执行幂等
insert into e2e_users (id, name, email, role, active) values
    ('00000000-0000-0000-0000-000000000001', 'Alice', 'alice@example.test', 'admin',  true),
    ('00000000-0000-0000-0000-000000000002', 'Bob',   'bob@example.test',   'tester', true)
on conflict (id) do nothing;

-- +goose Down
drop table if exists e2e_users;
