package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Amirreza-Zeraati/vaultline/internal/apperr"
	"github.com/Amirreza-Zeraati/vaultline/internal/models"
	"github.com/Amirreza-Zeraati/vaultline/internal/repository"
)

// fakeUserRepo is an in-memory stand-in for the real repository. Because
// AuthService depends on the repository.UserRepository *interface*, these tests
// need no database, no Docker, and run in milliseconds.
type fakeUserRepo struct {
	byEmail map[string]*models.User
	byID    map[uuid.UUID]*models.User

	// forceErr, when set, is returned by every method — used to check that
	// infrastructure failures surface as 500s rather than leaking.
	forceErr error

	createCalls int
}

func newFakeRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byEmail: make(map[string]*models.User),
		byID:    make(map[uuid.UUID]*models.User),
	}
}

func (f *fakeUserRepo) add(u *models.User) {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	f.byEmail[u.Email] = u
	f.byID[u.ID] = u
}

func (f *fakeUserRepo) Create(_ context.Context, user *models.User) error {
	if f.forceErr != nil {
		return f.forceErr
	}
	f.createCalls++
	f.add(user)
	return nil
}

func (f *fakeUserRepo) GetByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	if f.forceErr != nil {
		return nil, f.forceErr
	}
	u, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) GetByEmail(_ context.Context, email string) (*models.User, error) {
	if f.forceErr != nil {
		return nil, f.forceErr
	}
	u, ok := f.byEmail[email]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) Update(_ context.Context, _ *models.User) error { return f.forceErr }
func (f *fakeUserRepo) Delete(_ context.Context, _ uuid.UUID) error    { return f.forceErr }
func (f *fakeUserRepo) List(_ context.Context, _, _ int) ([]models.User, error) {
	return nil, f.forceErr
}

// mustHash builds a bcrypt hash for seeding fake users.
func mustHash(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing password: %v", err)
	}
	return string(h)
}

// assertCode fails unless err is an *apperr.Error with the expected code.
func assertCode(t *testing.T, err error, want apperr.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %q, got nil", want)
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperr.Error, got %T (%v)", err, err)
	}
	if appErr.Code != want {
		t.Fatalf("expected code %q, got %q", want, appErr.Code)
	}
}

func TestRegister_Success(t *testing.T) {
	repo := newFakeRepo()
	svc := NewAuthService(repo)

	user, err := svc.Register(context.Background(), "new@example.com", "supersecret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.Email != "new@example.com" {
		t.Errorf("email = %q, want new@example.com", user.Email)
	}
	if user.Role != "user" {
		t.Errorf("role = %q, want user", user.Role)
	}
	if repo.createCalls != 1 {
		t.Errorf("Create called %d times, want 1", repo.createCalls)
	}

	// The password must never be stored in plaintext.
	if user.PasswordHash == "supersecret" {
		t.Fatal("password stored in plaintext")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("supersecret")); err != nil {
		t.Errorf("stored hash does not verify against the original password: %v", err)
	}
}

func TestRegister_EmailAlreadyTaken(t *testing.T) {
	repo := newFakeRepo()
	repo.add(&models.User{Email: "taken@example.com", PasswordHash: mustHash(t, "whatever"), Role: "user"})
	svc := NewAuthService(repo)

	_, err := svc.Register(context.Background(), "taken@example.com", "supersecret")

	assertCode(t, err, apperr.CodeConflict)
	if !errors.Is(err, ErrEmailTaken) {
		t.Error("expected errors.Is(err, ErrEmailTaken) to be true")
	}
	if repo.createCalls != 0 {
		t.Errorf("Create called %d times on duplicate email, want 0", repo.createCalls)
	}
}

func TestRegister_RepositoryFailureIsInternal(t *testing.T) {
	repo := newFakeRepo()
	repo.forceErr = errors.New("connection refused")
	svc := NewAuthService(repo)

	_, err := svc.Register(context.Background(), "new@example.com", "supersecret")

	assertCode(t, err, apperr.CodeInternal)

	// The cause must be preserved for logs...
	if !errors.Is(err, repo.forceErr) {
		t.Error("underlying cause was not wrapped")
	}
	// ...but must not appear in the client-facing message.
	var appErr *apperr.Error
	errors.As(err, &appErr)
	if appErr.Message == "connection refused" {
		t.Error("internal detail leaked into the client message")
	}
}

func TestAuthenticate_Success(t *testing.T) {
	repo := newFakeRepo()
	repo.add(&models.User{Email: "user@example.com", PasswordHash: mustHash(t, "correct-password"), Role: "user"})
	svc := NewAuthService(repo)

	user, err := svc.Authenticate(context.Background(), "user@example.com", "correct-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "user@example.com" {
		t.Errorf("email = %q, want user@example.com", user.Email)
	}
}

func TestAuthenticate_WrongPasswordAndUnknownEmailAreIndistinguishable(t *testing.T) {
	repo := newFakeRepo()
	repo.add(&models.User{Email: "user@example.com", PasswordHash: mustHash(t, "correct-password"), Role: "user"})
	svc := NewAuthService(repo)

	_, wrongPass := svc.Authenticate(context.Background(), "user@example.com", "wrong-password")
	_, noSuchUser := svc.Authenticate(context.Background(), "nobody@example.com", "any-password")

	assertCode(t, wrongPass, apperr.CodeUnauthorized)
	assertCode(t, noSuchUser, apperr.CodeUnauthorized)

	// Identical messages: an attacker must not be able to enumerate accounts.
	if wrongPass.Error() != noSuchUser.Error() {
		t.Errorf("errors differ and leak whether the account exists:\n wrong password: %v\n unknown email:  %v",
			wrongPass, noSuchUser)
	}
}

func TestGetByID(t *testing.T) {
	repo := newFakeRepo()
	existing := &models.User{Email: "user@example.com", PasswordHash: mustHash(t, "pw"), Role: "admin"}
	repo.add(existing)
	svc := NewAuthService(repo)

	t.Run("found", func(t *testing.T) {
		user, err := svc.GetByID(context.Background(), existing.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Role != "admin" {
			t.Errorf("role = %q, want admin", user.Role)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.GetByID(context.Background(), uuid.New())
		assertCode(t, err, apperr.CodeNotFound)
	})
}
