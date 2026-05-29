-- +goose Up
-- AI 助理平台级运行时配置（工具轮次、路由模式、场景闭环等）

create table if not exists ai_assistant_settings (
    id         int         primary key check (id = 1),
    config     jsonb       not null default '{}'::jsonb,
    updated_at timestamptz not null default now()
);

insert into ai_assistant_settings (id, config)
values (1, '{}'::jsonb)
on conflict (id) do nothing;

-- +goose Down
drop table if exists ai_assistant_settings;
