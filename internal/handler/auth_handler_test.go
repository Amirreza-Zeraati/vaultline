package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Amirreza-Zeraati/vaultline/internal/apperr"
	"github.com/Amirreza-Zeraati/vaultline/internal/config"
	"github.com/Amirreza-Zeraati/vaultline/internal/models"
	"github.com/Amirreza-Zeraati/vaultline/internal/session"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

// fakeAuthService implements the handler's AuthService interface.
type fakeAuthService struct {
	user *models.User
	err  error
}

func (f *fakeAuthService) Register(_ context.Context, email, _ string) (*models.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	u := f.user
	if u == nil {
		u = &models.User{Email: email, Role: "user"}
		u.ID = uuid.New()
	}
	return u, nil
}

func (f *fakeAuthService) Authenticate(_ context.Context, email, _ string) (*models.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	u := f.user
	if u == nil {
		u = &models.User{Email: email, Role: "user"}
		u.ID = uuid.New()
	}
	return u, nil
}

func (f *fakeAuthService) GetByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	u := f.user
	if u == nil {
		u = &models.User{Email: "user@example.com", Role: "user"}
		u.ID = id
	}
	return u, nil
}

// fakeSessionStore is a minimal in-memory session.Store.
type fakeSessionStore struct {
	created map[string]session.Session
	err     error
}

func newFakeSessionStore() *fakeSessionStore {
	return &fakeSessionStore{created: make(map[string]session.Session)}
}

func (f *fakeSessionStore) Create(_ context.Context, s session.Session) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	id := "test-session-id"
	f.created[id] = s
	return id, nil
}
func (f *fakeSessionStore) Get(_ context.Context, id string) (session.Session, error) {
	s, ok := f.created[id]
	if !ok {
		return session.Session{}, session.ErrNotFound
	}
	return s, nil
}
func (f *fakeSessionStore) Delete(_ context.Context, id string) error {
	delete(f.created, id)
	return nil
}
func (f *fakeSessionStore) Refresh(_ context.Context, _ string) error { return nil }

func testConfig() config.Session {
	return config.Session{
		CookieName:     "session_id",
		CookiePath:     "/",
		CookieSameSite: "lax",
		TTL:            24 * time.Hour,
	}
}

func postJSON(h gin.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	r := gin.New()
	r.POST(path, h)

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// decodeError pulls the standard error envelope out of a response.
func decodeError(t *testing.T, w *httptest.ResponseRecorder) (code, message string, fields map[string]string) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string            `json:"code"`
			Message string            `json:"message"`
			Fields  map[string]string `json:"fields"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not the standard error envelope: %v (body: %s)", err, w.Body.String())
	}
	return body.Error.Code, body.Error.Message, body.Error.Fields
}

func TestRegister_Success(t *testing.T) {
	h := NewAuthHandler(&fakeAuthService{}, newFakeSessionStore(), testConfig())

	w := postJSON(h.Register, "/register", `{"email":"new@example.com","password":"supersecret"}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", w.Code, w.Body.String())
	}

	var body struct {
		Data struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if body.Data.Email != "new@example.com" {
		t.Errorf("email = %q, want new@example.com", body.Data.Email)
	}

	// The response must never expose the password hash.
	if bytes.Contains(w.Body.Bytes(), []byte("password_hash")) {
		t.Error("response leaked the password hash")
	}
}

func TestRegister_ValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
		wantField  string
	}{
		{"missing email", `{"password":"supersecret"}`, http.StatusUnprocessableEntity, "validation_failed", "Email"},
		{"malformed email", `{"email":"nope","password":"supersecret"}`, http.StatusUnprocessableEntity, "validation_failed", "Email"},
		{"short password", `{"email":"a@example.com","password":"short"}`, http.StatusUnprocessableEntity, "validation_failed", "Password"},
		{"malformed json", `{"email":`, http.StatusBadRequest, "invalid_input", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewAuthHandler(&fakeAuthService{}, newFakeSessionStore(), testConfig())
			w := postJSON(h.Register, "/register", tc.body)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			code, _, fields := decodeError(t, w)
			if code != tc.wantCode {
				t.Errorf("error code = %q, want %q", code, tc.wantCode)
			}
			if tc.wantField != "" {
				if _, ok := fields[tc.wantField]; !ok {
					t.Errorf("expected a message for field %q, got %v", tc.wantField, fields)
				}
			}
		})
	}
}

// A domain error from the service must drive the status code, with no mapping
// logic in the handler.
func TestRegister_ServiceErrorDrivesStatus(t *testing.T) {
	h := NewAuthHandler(
		&fakeAuthService{err: apperr.Conflict("email already registered")},
		newFakeSessionStore(),
		testConfig(),
	)

	w := postJSON(h.Register, "/register", `{"email":"taken@example.com","password":"supersecret"}`)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
	code, message, _ := decodeError(t, w)
	if code != "conflict" {
		t.Errorf("error code = %q, want conflict", code)
	}
	if message != "email already registered" {
		t.Errorf("message = %q", message)
	}
}

// An unexpected (non-apperr) error must become a generic 500 that leaks nothing.
func TestRegister_UnknownErrorBecomesSafe500(t *testing.T) {
	h := NewAuthHandler(
		&fakeAuthService{err: errors.New(`pq: password authentication failed for user "admin"`)},
		newFakeSessionStore(),
		testConfig(),
	)

	w := postJSON(h.Register, "/register", `{"email":"new@example.com","password":"supersecret"}`)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("password authentication failed")) {
		t.Error("internal error detail leaked to the client")
	}
}

func TestLogin_SetsHttpOnlySessionCookie(t *testing.T) {
	store := newFakeSessionStore()
	h := NewAuthHandler(&fakeAuthService{}, store, testConfig())

	w := postJSON(h.Login, "/login", `{"email":"user@example.com","password":"supersecret"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	var found *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "session_id" {
			found = c
		}
	}
	if found == nil {
		t.Fatal("no session cookie was set")
	}
	if !found.HttpOnly {
		t.Error("session cookie must be HttpOnly so JavaScript cannot read it")
	}
	if found.Value == "" {
		t.Error("session cookie has no value")
	}
	if len(store.created) != 1 {
		t.Errorf("%d sessions created, want 1", len(store.created))
	}
}

func TestLogin_InvalidCredentialsIs401(t *testing.T) {
	h := NewAuthHandler(
		&fakeAuthService{err: apperr.Unauthorized("invalid email or password")},
		newFakeSessionStore(),
		testConfig(),
	)

	w := postJSON(h.Login, "/login", `{"email":"user@example.com","password":"wrong"}`)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	code, _, _ := decodeError(t, w)
	if code != "unauthorized" {
		t.Errorf("error code = %q, want unauthorized", code)
	}
}
