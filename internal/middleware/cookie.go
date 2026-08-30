package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Amirreza-Zeraati/vaultline/internal/config"
)

// SetSessionCookie writes the session ID as an HttpOnly cookie. HttpOnly keeps
// it out of reach of JavaScript (XSS protection); Secure + SameSite come from
// config. maxAgeSeconds should match the session TTL.
func SetSessionCookie(c *gin.Context, cfg config.Session, sid string, maxAgeSeconds int) {
	c.SetSameSite(sameSite(cfg.CookieSameSite))
	c.SetCookie(cfg.CookieName, sid, maxAgeSeconds, cfg.CookiePath, cfg.CookieDomain, cfg.CookieSecure, true)
}

// ClearSessionCookie expires the cookie on the client.
func ClearSessionCookie(c *gin.Context, cfg config.Session) {
	c.SetSameSite(sameSite(cfg.CookieSameSite))
	c.SetCookie(cfg.CookieName, "", -1, cfg.CookiePath, cfg.CookieDomain, cfg.CookieSecure, true)
}

func sameSite(v string) http.SameSite {
	switch strings.ToLower(v) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
