package storage

import (
	"all-me-backend/internal/session"
	"all-me-backend/pkg/models"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type tokenStore struct{}

func (tokenStore) GetSessionToken(_, provider string) (*models.Token, error) {
	return &models.Token{AccessToken: "t", Provider: provider}, nil
}

type failingProvider struct{ *fakeProvider }

func (failingProvider) ParseShareLink(string, *models.Token) (*models.CloudItem, error) {
	return nil, errors.New("Graph API error 403 at https://graph.microsoft.com/v1.0/shares/u!secret: accessDenied")
}

func TestGetFolderContents_DoesNotLeakProviderErrors(t *testing.T) {
	p := failingProvider{&fakeProvider{}}
	e := echo.New()
	e.Logger.SetOutput(io.Discard)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { session.SetContext(c, "s"); return next(c) }
	})
	NewHandler(NewService(p, p), tokenStore{}).RegisterRoutes(e)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/storage/folder-contents?share_url=x&provider=onedrive", nil))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
	for _, secret := range []string{"graph.microsoft.com", "secret", "403", "accessDenied"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("response leaks %q: %s", secret, rec.Body.String())
		}
	}
}
