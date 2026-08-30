package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Amirreza-Zeraati/vaultline/internal/config"
	"github.com/Amirreza-Zeraati/vaultline/internal/session"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

// fakeStore is an in-memory session.Store.
type fakeStore struct {
	sessions     map[string]session.Session
	err          error
	refreshCalls int
}

func newFakeStore() *fakeStore {
	return &fakeStore{sessions: make(map[string]session.Session)}
}

func (f *fakeStore) Create(_ context.Context, s session.Session) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	id := uuid.NewString()
	f.sessions[id] = s
	return id, nil
}

func (f *fakeStore) Get(_ context.Context, id string) (session.Session, error) {
	if f.err != nil {
		return session.Session{}, f.err
	}
	s, ok := f.sessions[id]
	if !ok {
		return session.Session{}, session.ErrNotFound
	}
	return s, nil
}

func (f *fakeStore) Delete(_ context.Context, id string) error {
	delete(f.sessions, id)
	return f.err
}

func (f *fakeStore) Refresh(_ context.Context, id string) error {
	f.refreshCalls++
	if _, ok := f.sessions[id]; !ok {
		return session.ErrNotFound
	}
	return f.err
}

func testSessionConfig() config.Session {
	return config.Session{
		CookieName:     "session_id",
		CookiePath:     "/",
		CookieSameSite: "lax",
	}
}

// buildRouter wires the Auth middleware in front of a probe handler that
// reports what landed in the request context.
func buildRouter(store session.Store, extra ...gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	chain := append([]gin.HandlerFunc{Auth(store, testSessionConfig())}, extra...)
	r.GET("/protected", append(chain, func(c *gin.Context) {
		id, _ := CurrentUserID(c)
		role, _ := CurrentRole(c)
		sid, _ := CurrentSessionID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": id.String(), "role": role, "session_id": sid})
	})...)
	return r
}

func doRequest(r *gin.Engine, cookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "session_id", Value: cookie})
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuth_NoCookieIsUnauthorized(t *testing.T) {
	w := doRequest(buildRouter(newFakeStore()), "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not the standard error envelope: %v", err)
	}
	if body.Error.Code != "unauthorized" {
		t.Errorf("error code = %q, want unauthorized", body.Error.Code)
	}
}

func TestAuth_UnknownSessionIsUnauthorizedAndClearsCookie(t *testing.T) {
	w := doRequest(buildRouter(newFakeStore()), "does-not-exist")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}

	// The stale cookie should be expired on the client.
	var cleared bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "session_id" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("expected the stale session cookie to be cleared")
	}
}

func TestAuth_ValidSessionPopulatesContextAndRefreshesTTL(t *testing.T) {
	store := newFakeStore()
	userID := uuid.New()
	sid, err := store.Create(context.Background(), session.Session{UserID: userID, Role: "admin"})
	if err != nil {
		t.Fatalf("seeding session: %v", err)
	}

	w := doRequest(buildRouter(store), sid)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	var body struct {
		UserID    string `json:"user_id"`
		Role      string `json:"role"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if body.UserID != userID.String() {
		t.Errorf("user_id = %q, want %q", body.UserID, userID)
	}
	if body.Role != "admin" {
		t.Errorf("role = %q, want admin", body.Role)
	}
	if body.SessionID != sid {
		t.Errorf("session_id = %q, want %q", body.SessionID, sid)
	}
	if store.refreshCalls != 1 {
		t.Errorf("Refresh called %d times, want 1 (sliding expiration)", store.refreshCalls)
	}
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name       string
		userRole   string
		allowed    []string
		wantStatus int
	}{
		{"matching role passes", "admin", []string{"admin"}, http.StatusOK},
		{"one of several roles passes", "editor", []string{"admin", "editor"}, http.StatusOK},
		{"wrong role is forbidden", "user", []string{"admin"}, http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			sid, err := store.Create(context.Background(), session.Session{UserID: uuid.New(), Role: tc.userRole})
			if err != nil {
				t.Fatalf("seeding session: %v", err)
			}

			r := buildRouter(store, RequireRole(tc.allowed...))
			w := doRequest(r, sid)

			if w.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}
}

// RequireRole must not authorize a request that never went through Auth.
func TestRequireRole_WithoutAuthIsUnauthorized(t *testing.T) {
	r := gin.New()
	r.GET("/admin", RequireRole("admin"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
