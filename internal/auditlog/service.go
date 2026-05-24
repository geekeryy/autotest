package auditlog

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// Service 封装审计日志写入与查询。
type Service struct {
	repo *Repository
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// RecordLogin 记录登录成功或失败。
func (s *Service) RecordLogin(ctx context.Context, ev LoginEvent) error {
	detail := map[string]string{}
	if ev.FailureReason != "" {
		detail["reason"] = ev.FailureReason
	}
	if !ev.Success && ev.AttemptedPassword != "" {
		detail["attemptedPassword"] = ev.AttemptedPassword
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = s.repo.Create(ctx, createInput{
		Action:        ActionAuthLogin,
		Success:       ev.Success,
		ActorUserID:   ev.ActorUserID,
		ActorUsername: ev.ActorUsername,
		Resource:      ev.Method,
		ClientIP:      ev.ClientIP,
		UserAgent:     ev.UserAgent,
		Detail:        raw,
	})
	return err
}

// RecordLoginAttempt 实现 auth.LoginAuditor，供 auth 包注入。
func (s *Service) RecordLoginAttempt(ctx context.Context, success bool, method string, actorUserID *uuid.UUID, actorUsername, clientIP, userAgent, failureReason, attemptedPassword string) error {
	return s.RecordLogin(ctx, LoginEvent{
		Success:           success,
		Method:            method,
		ActorUserID:       actorUserID,
		ActorUsername:     actorUsername,
		ClientIP:          clientIP,
		UserAgent:         userAgent,
		FailureReason:     failureReason,
		AttemptedPassword: attemptedPassword,
	})
}

// List 分页查询审计日志。
func (s *Service) List(ctx context.Context, filter ListFilter) (*ListResponse, error) {
	items, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	return &ListResponse{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}
