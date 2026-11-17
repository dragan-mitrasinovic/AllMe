// Package session carries the server-issued session ID between the cookie and the handlers.
// The session ID is never accepted from query parameters or request bodies.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

const (
	CookieName = "allme_session"
	contextKey = "allme.session_id"
)

// NewID returns a new random session ID (256 bits, hex encoded).
func NewID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// FromContext returns the validated session ID for this request, or "" if there is none.
func FromContext(c echo.Context) string {
	id, _ := c.Get(contextKey).(string)
	return id
}

// SetContext stores the validated session ID for this request.
func SetContext(c echo.Context, id string) {
	c.Set(contextKey, id)
}

// CookieValue returns the raw session cookie value, or "" if the cookie is absent.
func CookieValue(c echo.Context) string {
	cookie, err := c.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// SetCookie sends the session cookie. It has no Max-Age, so it ends when the browser closes;
// the server-side session expiry still applies.
func SetCookie(c echo.Context, id string) {
	c.SetCookie(newCookie(id, 0))
}

// ClearCookie removes the session cookie from the browser.
func ClearCookie(c echo.Context) {
	c.SetCookie(newCookie("", -1))
}

func newCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     cookiePath(),
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   cookieSecure(),
		// Lax: sent on the top-level redirect back from the OAuth provider, but not on cross-site POST/DELETE.
		SameSite: http.SameSiteLaxMode,
	}
}

// cookiePath is the path the browser sees. The backend is served under /api (nginx and the Angular dev
// proxy strip the prefix), so the cookie is scoped to it.
func cookiePath() string {
	if p := os.Getenv("SESSION_COOKIE_PATH"); p != "" {
		return p
	}
	return "/api"
}

// cookieSecure is true unless the frontend is explicitly served over plain HTTP (local development).
func cookieSecure() bool {
	return !strings.HasPrefix(os.Getenv("FRONTEND_URL"), "http://")
}

// Required rejects requests that have no valid session.
func Required(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if FromContext(c) == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Not signed in. Please connect your cloud storage again.",
			})
		}
		return next(c)
	}
}
