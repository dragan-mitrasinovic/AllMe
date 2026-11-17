package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// Every data endpoint needs the session cookie; a session_id parameter is not enough.
func TestDataEndpointsRequireSessionCookie(t *testing.T) {
	e := echo.New()
	initialize(e)

	for _, tc := range []struct{ method, target string }{
		{http.MethodGet, "/storage/folder-contents?share_url=x&provider=onedrive&session_id=guess"},
		{http.MethodGet, "/thumbnail?url=x&provider=onedrive&session_id=guess"},
		{http.MethodPost, "/downloads/zip"},
		{http.MethodPost, "/face/register-base"},
		{http.MethodPost, "/face/compare-folder"},
		{http.MethodGet, "/face/job-status/job-1"},
	} {
		req := httptest.NewRequest(tc.method, tc.target, nil)
		req.AddCookie(&http.Cookie{Name: "allme_session", Value: "not-a-real-session"})
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401", tc.method, tc.target, rec.Code)
		}
	}
}
