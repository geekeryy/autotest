package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"autotest/internal/aitools"
	testcase "autotest/internal/case"
	"autotest/internal/project"
	"autotest/internal/scenario"

	"github.com/google/uuid"
)

// Tests in this file lock down the project-isolation guarantees we
// expose to AI tool calls: every tool that takes a projectId (directly,
// or transitively via caseId/scenarioId) must refuse to operate on a
// different project than the one bound to the chat session.

type fakeProjectService struct {
	calledWith uuid.UUID
}

func (f *fakeProjectService) ListServices(_ context.Context, projectID uuid.UUID) ([]project.Service, error) {
	f.calledWith = projectID
	return nil, nil
}
func (f *fakeProjectService) ListEnvironments(_ context.Context, _ uuid.UUID) ([]project.Environment, error) {
	return nil, nil
}
func (f *fakeProjectService) ListServiceEnvironments(_ context.Context, _, _ uuid.UUID) ([]project.Environment, error) {
	return nil, nil
}
func (f *fakeProjectService) CreateService(_ context.Context, _ uuid.UUID, _ project.CreateServiceInput) (*project.Service, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeProjectService) UpdateService(_ context.Context, _, _ uuid.UUID, _ project.UpdateServiceInput) (*project.Service, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeProjectService) DeleteService(_ context.Context, _, _ uuid.UUID) error {
	return errors.New("not implemented")
}
func (f *fakeProjectService) CreateServiceEnvironment(_ context.Context, _, _ uuid.UUID, _ project.CreateEnvironmentInput) (*project.Environment, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeProjectService) UpdateServiceEnvironment(_ context.Context, _, _, _ uuid.UUID, _ project.UpdateEnvironmentInput) (*project.Environment, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeProjectService) DeleteServiceEnvironment(_ context.Context, _, _, _ uuid.UUID) error {
	return errors.New("not implemented")
}
func (f *fakeProjectService) GetServiceEnvironment(_ context.Context, _, _, _ uuid.UUID) (*project.Environment, error) {
	return nil, errors.New("not implemented")
}

// listServicesTool ignores any args entirely — the schema no longer
// exposes projectId, and the implementation always reaches for the
// session project. Even a clever model that sneaks an extra projectId
// into args must not get a foothold here.
func TestListServices_AlwaysUsesSessionProject(t *testing.T) {
	t.Parallel()
	svc := &fakeProjectService{}
	tool := listServicesTool(Deps{Projects: svc})

	session := uuid.New()
	ctx := aitools.WithCaller(context.Background(), aitools.CallerContext{ProjectID: session})
	// Even if the model snuck a different projectId into args, the
	// implementation does not read it.
	args := mustJSON(t, map[string]any{"projectId": uuid.New().String()})

	if _, err := tool.Run(ctx, args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.calledWith != session {
		t.Errorf("ListServices should always be called with session project %s, got %s", session, svc.calledWith)
	}
}

// listServicesTool refuses to run without a caller stamped on ctx —
// guards against an SSE handler that forgets WithCaller.
func TestListServices_RejectsWhenNoCaller(t *testing.T) {
	t.Parallel()
	tool := listServicesTool(Deps{Projects: &fakeProjectService{}})
	_, err := tool.Run(context.Background(), mustJSON(t, map[string]any{}))
	if err == nil || !strings.Contains(err.Error(), "未携带会话身份") {
		t.Fatalf("expected missing-caller error, got %v", err)
	}
}

// list_cases must use the session's project even when the model omits
// projectId entirely.
func TestListCases_FallsBackToSessionProject(t *testing.T) {
	t.Parallel()
	called := uuid.Nil
	cases := &recordingCaseService{
		listFn: func(ctx context.Context, filter testcase.ListFilter) ([]testcase.TestCase, error) {
			called = filter.ProjectID
			return nil, nil
		},
	}
	tool := listCasesTool(Deps{Cases: cases})

	session := uuid.New()
	ctx := aitools.WithCaller(context.Background(), aitools.CallerContext{ProjectID: session})
	if _, err := tool.Run(ctx, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != session {
		t.Errorf("CaseService.List should be called with session project %s, got %s", session, called)
	}
}

// get_case fetches a TestCase that belongs to project A, but the session
// is bound to project B → the tool must reject the result.
func TestGetCase_RejectsCrossProjectFetch(t *testing.T) {
	t.Parallel()
	otherProject := uuid.New()
	caseID := uuid.New()
	cases := &recordingCaseService{
		getFn: func(_ context.Context, _ uuid.UUID) (*testcase.TestCase, error) {
			return &testcase.TestCase{ID: caseID, ProjectID: otherProject}, nil
		},
	}
	tool := getCaseTool(Deps{Cases: cases})

	session := uuid.New()
	ctx := aitools.WithCaller(context.Background(), aitools.CallerContext{ProjectID: session})
	args := mustJSON(t, map[string]any{"caseId": caseID.String()})

	_, err := tool.Run(ctx, args)
	if err == nil || !strings.Contains(err.Error(), "权限隔离") {
		t.Fatalf("expected cross-project rejection, got %v", err)
	}
}

// update_scenario_step is the worst case: only scenarioId is in the
// args, the test case (and project) are reached through Scenarios.Get.
// We must still refuse before touching ListSteps / UpsertStep.
func TestUpdateScenarioStep_RejectsCrossProjectScenario(t *testing.T) {
	t.Parallel()
	svc := newFakeScenarios()
	otherProject := uuid.New()
	sc, _ := svc.Create(context.Background(), scenario.CreateScenarioInput{
		ProjectID: otherProject,
		ServiceID: uuid.New(),
		Name:      "owned by another project",
	})

	tool := updateScenarioStepTool(Deps{Scenarios: svc})
	session := uuid.New()
	ctx := aitools.WithCaller(context.Background(), aitools.CallerContext{ProjectID: session})
	args := mustJSON(t, map[string]any{
		"scenarioId": sc.ID.String(),
		"stepOrder":  1,
		"name":       "should never run",
	})
	_, err := tool.Run(ctx, args)
	if err == nil || !strings.Contains(err.Error(), "权限隔离") {
		t.Fatalf("expected cross-project rejection, got %v", err)
	}
}

// recordingCaseService is a CaseService stub that lets tests plug in
// only the methods they need; everything else returns an explicit
// failure so unexpected calls show up loudly.
type recordingCaseService struct {
	listFn   func(ctx context.Context, filter testcase.ListFilter) ([]testcase.TestCase, error)
	getFn    func(ctx context.Context, id uuid.UUID) (*testcase.TestCase, error)
	patchFn  func(ctx context.Context, id uuid.UUID, input testcase.PatchInput) (*testcase.TestCase, error)
	createFn func(ctx context.Context, input testcase.CreateManualInput) (*testcase.TestCase, error)
}

func (r *recordingCaseService) Get(ctx context.Context, id uuid.UUID) (*testcase.TestCase, error) {
	if r.getFn != nil {
		return r.getFn(ctx, id)
	}
	return nil, errors.New("recordingCaseService.Get not configured")
}

func (r *recordingCaseService) List(ctx context.Context, filter testcase.ListFilter) ([]testcase.TestCase, error) {
	if r.listFn != nil {
		return r.listFn(ctx, filter)
	}
	return nil, errors.New("recordingCaseService.List not configured")
}

func (r *recordingCaseService) ListSaved(_ context.Context, _ uuid.UUID) ([]testcase.TestCase, error) {
	return nil, errors.New("recordingCaseService.ListSaved not configured")
}

func (r *recordingCaseService) CreateManual(ctx context.Context, input testcase.CreateManualInput) (*testcase.TestCase, error) {
	if r.createFn != nil {
		return r.createFn(ctx, input)
	}
	return nil, errors.New("recordingCaseService.CreateManual not configured")
}

func (r *recordingCaseService) CreateSaved(_ context.Context, _ uuid.UUID, _ testcase.CreateSavedInput) (*testcase.TestCase, error) {
	return nil, errors.New("recordingCaseService.CreateSaved not configured")
}

func (r *recordingCaseService) Patch(ctx context.Context, id uuid.UUID, input testcase.PatchInput) (*testcase.TestCase, error) {
	if r.patchFn != nil {
		return r.patchFn(ctx, id, input)
	}
	return nil, errors.New("recordingCaseService.Patch not configured")
}
