package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"

	"github.com/Amirreza-Zeraati/vaultline/internal/config"
	"github.com/Amirreza-Zeraati/vaultline/internal/redis"
)

func newRateLimitRouter(t *testing.T, cfg config.RateLimit) (*gin.Engine, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	port, err := strconv.Atoi(mr.Port())
	if err != nil {
		t.Fatalf("parsing miniredis port: %v", err)
	}
	rdb := redis.New(config.Redis{Host: mr.Host(), Port: port})
	t.Cleanup(func() { _ = rdb.Close() })

	r := gin.New()
	r.Use(RateLimit(rdb, cfg))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r, mr
}

func hit(r *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:4567"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRateLimit_AllowsUpToLimitThenBlocks(t *testing.T) {
	r, _ := newRateLimitRouter(t, config.RateLimit{Enabled: true, Requests: 3, Window: time.Minute})

	for i := 1; i <= 3; i++ {
		if w := hit(r); w.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, w.Code)
		}
	}

	w := hit(r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request: status = %d, want 429", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After = %q, want 60", got)
	}
	if got := w.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Errorf("X-RateLimit-Remaining = %q, want 0", got)
	}
}

// Regression test: the limiter previously called a plain EXPIRE on every
// request, which pushed the TTL out continuously. A client that kept sending
// traffic would never get a fresh window and stayed blocked forever. With
// EXPIRE NX the TTL is set once, so the window really does roll over.
func TestRateLimit_WindowRollsOverUnderContinuousTraffic(t *testing.T) {
	window := time.Minute
	r, mr := newRateLimitRouter(t, config.RateLimit{Enabled: true, Requests: 2, Window: window})

	// Burn the quota, then keep hammering — each of these must NOT extend the
	// window.
	hit(r)
	hit(r)
	for i := 0; i < 5; i++ {
		mr.FastForward(10 * time.Second)
		if w := hit(r); w.Code != http.StatusTooManyRequests {
			t.Fatalf("expected to still be limited, got %d", w.Code)
		}
	}

	// Total elapsed is now past the original window, so the counter should have
	// expired and the client should be served again.
	mr.FastForward(15 * time.Second)
	if w := hit(r); w.Code != http.StatusOK {
		t.Fatalf("window did not roll over: status = %d, want 200", w.Code)
	}
}

func TestRateLimit_DisabledPassesEverythingThrough(t *testing.T) {
	r, _ := newRateLimitRouter(t, config.RateLimit{Enabled: false, Requests: 1, Window: time.Minute})

	for i := 0; i < 10; i++ {
		if w := hit(r); w.Code != http.StatusOK {
			t.Fatalf("request %d blocked while limiting is disabled", i)
		}
	}
}

// Redis being down must not take the API down with it.
func TestRateLimit_FailsOpenWhenRedisIsUnavailable(t *testing.T) {
	r, mr := newRateLimitRouter(t, config.RateLimit{Enabled: true, Requests: 1, Window: time.Minute})
	mr.Close()

	for i := 0; i < 3; i++ {
		if w := hit(r); w.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200 (limiter should fail open)", i, w.Code)
		}
	}
}
