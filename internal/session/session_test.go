package session

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"

	"github.com/Amirreza-Zeraati/vaultline/internal/config"
	"github.com/Amirreza-Zeraati/vaultline/internal/redis"
)

// newTestStore spins up an in-process Redis (miniredis) so these tests need no
// running server and no Docker.
func newTestStore(t *testing.T, ttl time.Duration) (Store, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.New(config.Redis{
		Host: mr.Host(),
		Port: mustPort(t, mr.Port()),
	})
	t.Cleanup(func() { _ = rdb.Close() })

	return NewRedisStore(rdb, ttl), mr
}

func mustPort(t *testing.T, port string) int {
	t.Helper()
	p, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("parsing miniredis port %q: %v", port, err)
	}
	return p
}

func TestCreateAndGet(t *testing.T) {
	store, _ := newTestStore(t, time.Hour)
	ctx := context.Background()

	userID := uuid.New()
	id, err := store.Create(ctx, Session{UserID: userID, Role: "admin"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Fatal("Create returned an empty session id")
	}

	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UserID != userID {
		t.Errorf("user id = %v, want %v", got.UserID, userID)
	}
	if got.Role != "admin" {
		t.Errorf("role = %q, want admin", got.Role)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt was not set")
	}
}

// Session IDs must be unguessable and never repeat.
func TestCreateGeneratesUniqueOpaqueIDs(t *testing.T) {
	store, _ := newTestStore(t, time.Hour)
	ctx := context.Background()

	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		id, err := store.Create(ctx, Session{UserID: uuid.New(), Role: "user"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate session id generated: %q", id)
		}
		// 32 random bytes, base64url encoded, is 43 characters.
		if len(id) < 40 {
			t.Errorf("session id %q looks too short to be 32 random bytes", id)
		}
		seen[id] = struct{}{}
	}
}

func TestGetMissingReturnsErrNotFound(t *testing.T) {
	store, _ := newTestStore(t, time.Hour)

	_, err := store.Get(context.Background(), "no-such-session")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteRevokesImmediately(t *testing.T) {
	store, _ := newTestStore(t, time.Hour)
	ctx := context.Background()

	id, err := store.Create(ctx, Session{UserID: uuid.New(), Role: "user"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := store.Get(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("session still readable after delete: err = %v", err)
	}
}

func TestSessionExpiresAfterTTL(t *testing.T) {
	ttl := time.Minute
	store, mr := newTestStore(t, ttl)
	ctx := context.Background()

	id, err := store.Create(ctx, Session{UserID: uuid.New(), Role: "user"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Jump past the TTL without sleeping.
	mr.FastForward(ttl + time.Second)

	if _, err := store.Get(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected the session to have expired, err = %v", err)
	}
}

func TestRefreshExtendsTTL(t *testing.T) {
	ttl := time.Minute
	store, mr := newTestStore(t, ttl)
	ctx := context.Background()

	id, err := store.Create(ctx, Session{UserID: uuid.New(), Role: "user"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Advance most of the way, then refresh: the clock restarts.
	mr.FastForward(50 * time.Second)
	if err := store.Refresh(ctx, id); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	// Past the original expiry, but within the refreshed window.
	mr.FastForward(30 * time.Second)
	if _, err := store.Get(ctx, id); err != nil {
		t.Errorf("session should still be alive after refresh, got %v", err)
	}
}

func TestRefreshMissingReturnsErrNotFound(t *testing.T) {
	store, _ := newTestStore(t, time.Hour)

	err := store.Refresh(context.Background(), "no-such-session")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
