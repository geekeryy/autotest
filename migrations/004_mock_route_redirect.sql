-- +goose Up
-- Mock 规则支持 HTTP 重定向响应，用于模拟 SSO/OAuth 授权端点。

alter table mock_routes
    add column if not exists response_mode text not null default 'body'
        check (response_mode in ('body', 'redirect'));

alter table mock_routes
    add column if not exists redirect_location text not null default '';

-- +goose Down
alter table mock_routes drop column if exists redirect_location;
alter table mock_routes drop column if exists response_mode;
