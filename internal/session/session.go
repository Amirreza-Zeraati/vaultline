// Package session implements server-side sessions stored in Redis. A random,
// opaque session ID is handed to the client in a cookie; all session data
// lives server-side in Redis and can be revoked instantly (unlike a JWT).
package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Amirreza-Zeraati/vaultline/internal/redis"
)

// ErrNotFound is returned when a session ID doesn't exist or has expired.
var ErrNotFound = errors.New("session not found")

const keyPrefix = "session:"

// Session is the data stored server-side for a logged-in user. Role is cached
// here so the auth middleware needs zero DB hits per request.
type Session struct {
	UserID    uuid.UUID `json:"user_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Store is the session persistence contract.
type Store interface {
	Create(ctx context.Context, s Session) (id string, err error)
	Get(ctx context.Context, id string) (Session, error)
	Delete(ctx context.Context, id string) error
	Refresh(ctx context.Context, id string) error
}

// redisStore is the Redis-backed implementation.
type redisStore struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewRedisStore builds a session store with the given TTL.
func NewRedisStore(rdb *redis.Client, ttl time.Duration) Store {
	return &redisStore{rdb: rdb, ttl: ttl}
}

func (s *redisStore) Create(ctx context.Context, sess Session) (string, error) {
	id, err := generateID()
	if err != nil {
		return "", err
	}
	sess.CreatedAt = time.Now().UTC()

	data, err := json.Marshal(sess)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}
	if err := s.rdb.Set(ctx, keyPrefix+id, data, s.ttl).Err(); err != nil {
		return "", fmt.Errorf("store session: %w", err)
	}
	return id, nil
}

func (s *redisStore) Get(ctx context.Context, id string) (Session, error) {
	data, err := s.rdb.Get(ctx, keyPrefix+id).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return Session{}, ErrNotFound
		}
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return Session{}, fmt.Errorf("unmarshal session: %w", err)
	}
	return sess, nil
}

func (s *redisStore) Delete(ctx context.Context, id string) error {
	return s.rdb.Del(ctx, keyPrefix+id).Err()
}

// Refresh extends the session's TTL (sliding expiration).
func (s *redisStore) Refresh(ctx context.Context, id string) error {
	ok, err := s.rdb.Expire(ctx, keyPrefix+id, s.ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// generateID returns 32 bytes of cryptographically-random data, base64url
// encoded — an unguessable, opaque session identifier.
func generateID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
