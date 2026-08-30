package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Amirreza-Zeraati/vaultline/internal/metrics"
)

// scrape renders the current metrics exposition text.
func scrape(t *testing.T, m *metrics.Metrics) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, req)
	return w.Body.String()
}

// The route label must be the registered pattern, not the concrete URL —
// otherwise every distinct ID creates a new Prometheus time series.
func TestMetrics_UsesRoutePatternNotRawPath(t *testing.T) {
	m := metrics.New()

	r := gin.New()
	r.Use(Metrics(m))
	r.GET("/users/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, id := range []string{"1", "2", "3"} {
		req := httptest.NewRequest(http.MethodGet, "/users/"+id, nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
	}

	body := scrape(t, m)

	if !strings.Contains(body, `route="/users/:id"`) {
		t.Errorf("expected the route pattern label, got:\n%s", body)
	}
	if strings.Contains(body, `route="/users/1"`) {
		t.Error("concrete path used as a label — this explodes metric cardinality")
	}
}

func TestMetrics_RecordsStatusAndUnmatchedRoutes(t *testing.T) {
	m := metrics.New()

	r := gin.New()
	r.Use(Metrics(m))
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })

	for _, path := range []string{"/ok", "/boom", "/does-not-exist"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
	}

	body := scrape(t, m)

	for _, want := range []string{
		`status="200"`,
		`status="500"`,
		`route="unmatched"`,
		"http_request_duration_seconds",
		"http_requests_in_flight",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in the metrics output", want)
		}
	}
}

func TestSetDependencyUp(t *testing.T) {
	m := metrics.New()
	m.SetDependencyUp("postgres", true)
	m.SetDependencyUp("redis", false)

	body := scrape(t, m)

	if !strings.Contains(body, `dependency_up{dependency="postgres"} 1`) {
		t.Errorf("postgres gauge not set to 1:\n%s", body)
	}
	if !strings.Contains(body, `dependency_up{dependency="redis"} 0`) {
		t.Errorf("redis gauge not set to 0:\n%s", body)
	}
}
