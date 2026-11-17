package thumbnail

import (
	"all-me-backend/internal/session"
	"all-me-backend/pkg/models"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type fakeStore struct{}

func (fakeStore) GetSessionToken(_, provider string) (*models.Token, error) {
	return &models.Token{AccessToken: "t", Provider: provider}, nil
}

type recordingProvider struct{ calls []string }

func (p *recordingProvider) GetThumbnailStream(u string, _ *models.Token) (io.ReadCloser, error) {
	p.calls = append(p.calls, u)
	return io.NopCloser(strings.NewReader("img")), nil
}

func TestThumbnailProxy_OnlyFetchesProviderHosts(t *testing.T) {
	google, onedrive := &recordingProvider{}, &recordingProvider{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { session.SetContext(c, "s"); return next(c) }
	})
	NewHandler(fakeStore{}, google, onedrive).RegisterRoutes(e)

	get := func(provider, target string) int {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
			"/thumbnail?provider="+provider+"&url="+url.QueryEscape(target), nil))
		return rec.Code
	}

	for _, bad := range []struct{ provider, url string }{
		{"onedrive", "http://face-service:8081/face/session/x"},
		{"googledrive", "https://169.254.169.254/latest/meta-data/"},
		{"googledrive", "https://evil.example/?googleapis.com"},
		{"onedrive", "https://www.googleapis.com/drive/v3/files/a?alt=media"},
	} {
		if code := get(bad.provider, bad.url); code != http.StatusBadRequest {
			t.Errorf("%s %s: got %d, want 400", bad.provider, bad.url, code)
		}
	}
	if len(google.calls)+len(onedrive.calls) != 0 {
		t.Fatalf("rejected URLs must not be fetched, got %v %v", google.calls, onedrive.calls)
	}

	if code := get("googledrive", "https://www.googleapis.com/drive/v3/files/a?alt=media&sz=s400"); code != http.StatusOK {
		t.Errorf("valid Drive thumbnail: got %d, want 200", code)
	}
	if len(google.calls) != 1 {
		t.Errorf("expected the valid URL to be fetched once, got %v", google.calls)
	}
}
