package aitools

// caller.go provides a per-request, in-context handshake between the
// chat entry point (which already knows the authenticated user and the
// session's project) and individual tool implementations.
//
// Why a separate file/concept: tool functions take only `context.Context`
// and the model-supplied arguments. The model can in principle put any
// `projectId` UUID into a tool call, and we must not trust it blindly.
// By stamping a CallerContext onto the request `context.Context` once at
// the SSE handler boundary, every tool can validate "the projectId I'm
// being asked to mutate matches the session this request came from"
// without each tool re-implementing the check.
//
// This file deliberately stays free of `internal/auth` so that the
// `aitools` package (used by lower-level packages like `aiprovider`)
// keeps its tiny dependency footprint.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// CallerContext carries the minimal identity needed by tools. UserID is
// recorded so future audit logging can attribute mutations; ProjectID is
// the only field tools actually enforce against today.
type CallerContext struct {
	UserID    uuid.UUID
	Username  string
	ProjectID uuid.UUID
}

type callerCtxKey struct{}

// WithCaller stamps the caller identity onto ctx. Call this once at the
// SSE handler boundary; tools will then read it via CallerFromContext.
func WithCaller(ctx context.Context, cc CallerContext) context.Context {
	return context.WithValue(ctx, callerCtxKey{}, cc)
}

// CallerFromContext returns the caller stamped earlier in the request.
// The second return is false when the chain forgot to stamp the caller,
// which is treated as a programming bug at the validation helpers below.
func CallerFromContext(ctx context.Context) (CallerContext, bool) {
	cc, ok := ctx.Value(callerCtxKey{}).(CallerContext)
	return cc, ok
}

// RequireProjectAccess verifies a tool that is about to operate on
// projectID was invoked from a session bound to that very project. Cross
// project attempts surface a Chinese error so the model can apologise to
// the user instead of attempting the call silently.
//
// Returns a non-nil error in any of:
//   - ctx has no caller stamped (entry point forgot WithCaller);
//   - the session is not yet bound to a project (UI bug);
//   - the model passed a different projectID than the session.
func RequireProjectAccess(ctx context.Context, projectID uuid.UUID) error {
	cc, ok := CallerFromContext(ctx)
	if !ok {
		return errors.New("内部错误：当前请求未携带会话身份，请刷新页面后重试")
	}
	if cc.ProjectID == uuid.Nil {
		return errors.New("当前 AI 会话未绑定项目，请在前端重新选择项目后继续")
	}
	if projectID == uuid.Nil {
		return errors.New("projectId 不能为空")
	}
	if projectID != cc.ProjectID {
		return fmt.Errorf(
			"权限隔离：本会话仅允许操作当前项目（%s），不允许调用项目 %s 的资源",
			cc.ProjectID, projectID,
		)
	}
	return nil
}

// ResolveProjectID is the convenience helper most tools want: if the
// model omitted projectId in args, fall back to the session's project;
// if it supplied one, validate it matches. Either way the returned UUID
// is safe to pass into a service call.
func ResolveProjectID(ctx context.Context, arg string) (uuid.UUID, error) {
	cc, ok := CallerFromContext(ctx)
	if !ok {
		return uuid.Nil, errors.New("内部错误：当前请求未携带会话身份，请刷新页面后重试")
	}
	if cc.ProjectID == uuid.Nil {
		return uuid.Nil, errors.New("当前 AI 会话未绑定项目，请在前端重新选择项目后继续")
	}
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return cc.ProjectID, nil
	}
	parsed, err := uuid.Parse(arg)
	if err != nil {
		return uuid.Nil, fmt.Errorf("projectId 不是合法 UUID: %w", err)
	}
	if parsed != cc.ProjectID {
		return uuid.Nil, fmt.Errorf(
			"权限隔离：本会话仅允许操作当前项目（%s），不允许调用项目 %s 的资源",
			cc.ProjectID, parsed,
		)
	}
	return parsed, nil
}
