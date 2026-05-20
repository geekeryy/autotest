package spec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"autotest/internal/auth"

	"github.com/google/uuid"
)

type mockImportNotifier struct {
	calls int
	last  struct {
		userID    uuid.UUID
		projectID uuid.UUID
		serviceID uuid.UUID
		summary   *ImportSummary
	}
	err error
}

func (m *mockImportNotifier) CreateSpecImportNotification(_ context.Context, userID, projectID, serviceID uuid.UUID, summary *ImportSummary) error {
	m.calls++
	m.last.userID = userID
	m.last.projectID = projectID
	m.last.serviceID = serviceID
	m.last.summary = summary
	return m.err
}

func TestNotifyImportIfAPIKey(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	projectID := uuid.New()
	serviceID := uuid.New()
	summary := &ImportSummary{SpecID: uuid.New(), ApiCount: 1}

	cases := []struct {
		name      string
		principal *auth.Principal
		notifier  ImportNotifier
		wantCalls int
	}{
		{
			name:      "nil notifier skipped",
			principal: &auth.Principal{UserID: userID, Source: auth.SourceAPIKey},
			notifier:  nil,
			wantCalls: 0,
		},
		{
			name:      "jwt source skipped",
			principal: &auth.Principal{UserID: userID, Source: auth.SourceJWT},
			notifier:  &mockImportNotifier{},
			wantCalls: 0,
		},
		{
			name:      "missing principal skipped",
			principal: nil,
			notifier:  &mockImportNotifier{},
			wantCalls: 0,
		},
		{
			name:      "api key source notifies",
			principal: &auth.Principal{UserID: userID, Source: auth.SourceAPIKey},
			notifier:  &mockImportNotifier{},
			wantCalls: 1,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := &Handler{notifier: tc.notifier}
			req := httptest.NewRequest(http.MethodPost, "/import", nil)
			if tc.principal != nil {
				req = req.WithContext(auth.WithPrincipal(req.Context(), tc.principal))
			}
			h.notifyImportIfAPIKey(req, projectID, serviceID, summary)
			mock, _ := tc.notifier.(*mockImportNotifier)
			if mock == nil {
				return
			}
			if mock.calls != tc.wantCalls {
				t.Fatalf("calls = %d, want %d", mock.calls, tc.wantCalls)
			}
			if tc.wantCalls == 0 {
				return
			}
			if mock.last.userID != userID || mock.last.projectID != projectID || mock.last.serviceID != serviceID {
				t.Fatalf("last ids mismatch: %+v", mock.last)
			}
			if mock.last.summary != summary {
				t.Fatalf("summary pointer mismatch")
			}
		})
	}
}
