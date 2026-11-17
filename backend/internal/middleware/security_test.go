package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestSameOriginWrites(t *testing.T) {
	t.Setenv("FRONTEND_URL", "https://allme.example")
	e := echo.New()
	e.Use(SameOriginWrites())
	ok := func(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
	e.POST("/x", ok)
	e.GET("/x", ok)

	cases := []struct {
		method, origin string
		want           int
	}{
		{http.MethodPost, "https://evil.example", http.StatusForbidden},
		{http.MethodPost, "https://allme.example", http.StatusNoContent},
		{http.MethodPost, "", http.StatusNoContent},
		{http.MethodGet, "https://evil.example", http.StatusNoContent},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, "/x", nil)
		if tc.origin != "" {
			req.Header.Set(echo.HeaderOrigin, tc.origin)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s with Origin %q: got %d, want %d", tc.method, tc.origin, rec.Code, tc.want)
		}
	}
}
