package auth

import (
	"all-me-backend/internal/session"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func newTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"victim-token","scope":"Files.Read"}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func newTestApp(t *testing.T) (*echo.Echo, *Service) {
	t.Helper()
	t.Setenv("FRONTEND_URL", "https://allme.example")
	service := createTestService(newTokenServer(t).URL)
	e := echo.New()
	e.Use(SessionMiddleware(service))
	NewHandler(service).RegisterRoutes(e)
	return e, service
}

func do(e *echo.Echo, method, target, cookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: cookie})
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.CookieName {
			return c
		}
	}
	return nil
}

// login starts a login like a browser without a session cookie and returns the cookie and OAuth state.
func login(t *testing.T, e *echo.Echo) (cookie, state string) {
	t.Helper()
	rec := do(e, http.MethodGet, "/auth/onedrive/login", "")
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("login: expected redirect, got %d", rec.Code)
	}
	c := sessionCookie(t, rec)
	if c == nil || c.Value == "" {
		t.Fatal("login: expected a session cookie")
	}
	location, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("login: bad redirect: %v", err)
	}
	return c.Value, location.Query().Get("state")
}

func TestLogin_IssuesServerSessionCookie(t *testing.T) {
	e, service := newTestApp(t)

	rec := do(e, http.MethodGet, "/auth/onedrive/login?session_id=attacker-chosen", "")
	c := sessionCookie(t, rec)
	if c == nil {
		t.Fatal("expected a session cookie")
	}
	if c.Value == "attacker-chosen" || len(c.Value) != 64 {
		t.Errorf("expected a random server-generated session ID, got %q", c.Value)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/api" || c.MaxAge != 0 {
		t.Errorf("unexpected cookie attributes: %+v", c)
	}
	if !service.ResolveSession(c.Value) {
		t.Error("expected the cookie's session to exist")
	}
	if service.ResolveSession("attacker-chosen") {
		t.Error("a session_id query parameter must never create a session")
	}
}

func TestCallback_SameBrowser_StoresTokenAndRotatesSession(t *testing.T) {
	e, service := newTestApp(t)
	cookie, state := login(t, e)

	rec := do(e, http.MethodGet, "/auth/onedrive/callback?code=c&state="+state, cookie)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "success=true") {
		t.Fatalf("expected success redirect, got %q", loc)
	}
	rotated := sessionCookie(t, rec)
	if rotated == nil || rotated.Value == "" || rotated.Value == cookie {
		t.Fatalf("expected a new session cookie after login, got %+v", rotated)
	}
	if service.ResolveSession(cookie) {
		t.Error("the pre-login session ID must not stay valid")
	}
	if token, err := service.GetSessionToken(rotated.Value, "onedrive"); err != nil || token.AccessToken != "victim-token" {
		t.Errorf("expected the token under the new session, got %v, %v", token, err)
	}
}

func TestCallback_OtherBrowser_IsRejected(t *testing.T) {
	e, service := newTestApp(t)

	// The attacker starts a login and sends the provider's auth URL (with their state) to the victim.
	attackerCookie, attackerState := login(t, e)
	// The victim's browser has its own session (or none at all).
	victimCookie, _ := login(t, e)

	for name, cookie := range map[string]string{"victim session": victimCookie, "no session": ""} {
		rec := do(e, http.MethodGet, "/auth/onedrive/callback?code=c&state="+attackerState, cookie)
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "error=auth_failed") {
			t.Errorf("%s: expected auth_failed redirect, got %q", name, loc)
		}
		if sessionCookie(t, rec) != nil {
			t.Errorf("%s: no session cookie may be issued on a rejected callback", name)
		}
	}

	if _, err := service.GetSessionToken(attackerCookie, "onedrive"); err == nil {
		t.Error("the attacker's session must not receive the victim's token")
	}
}

func TestValidateSession_IgnoresSessionIDQueryParameter(t *testing.T) {
	e, _ := newTestApp(t)
	cookie, state := login(t, e)
	rotated := sessionCookie(t, do(e, http.MethodGet, "/auth/onedrive/callback?code=c&state="+state, cookie)).Value

	withoutCookie := do(e, http.MethodGet, "/auth/validate-session?provider=onedrive&session_id="+rotated, "")
	if !strings.Contains(withoutCookie.Body.String(), `"valid":false`) {
		t.Errorf("a session_id query parameter must not authenticate, got %s", withoutCookie.Body.String())
	}

	withCookie := do(e, http.MethodGet, "/auth/validate-session?provider=onedrive", rotated)
	if !strings.Contains(withCookie.Body.String(), `"valid":true`) {
		t.Errorf("expected the cookie session to be valid, got %s", withCookie.Body.String())
	}
}

func TestSignOut_LastProviderEndsSession(t *testing.T) {
	e, service := newTestApp(t)
	cookie, state := login(t, e)
	rotated := sessionCookie(t, do(e, http.MethodGet, "/auth/onedrive/callback?code=c&state="+state, cookie)).Value

	rec := do(e, http.MethodPost, "/auth/signout?provider=onedrive", rotated)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if c := sessionCookie(t, rec); c == nil || c.MaxAge >= 0 {
		t.Errorf("expected the session cookie to be cleared, got %+v", c)
	}
	if service.ResolveSession(rotated) {
		t.Error("expected the session to be deleted")
	}
}
