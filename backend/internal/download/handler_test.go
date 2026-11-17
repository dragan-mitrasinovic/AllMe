package download

import (
	"all-me-backend/internal/session"
	"all-me-backend/pkg/models"
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type fakeStore struct{}

func (fakeStore) GetSessionToken(_, provider string) (*models.Token, error) {
	return &models.Token{AccessToken: "t", Provider: provider}, nil
}

type recordingStorage struct{ fetched []string }

func (s *recordingStorage) GetFileStream(item *models.CloudItem, _ *models.Token) (io.ReadCloser, error) {
	s.fetched = append(s.fetched, item.DownloadURL)
	return io.NopCloser(strings.NewReader("data")), nil
}

func newZipApp(storage *recordingStorage) *echo.Echo {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { session.SetContext(c, "s"); return next(c) }
	})
	NewHandler(NewService(storage), fakeStore{}).RegisterRoutes(e)
	return e
}

func postZip(e *echo.Echo, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/downloads/zip", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestDownloadZip_RejectsForeignURLsBeforeFetching(t *testing.T) {
	storage := &recordingStorage{}
	e := newZipApp(storage)

	rec := postZip(e, `{"provider":"googledrive","files":[
		{"name":"ok.jpg","download_url":"https://www.googleapis.com/drive/v3/files/a?alt=media"},
		{"name":"x.jpg","download_url":"http://face-service:8081/face/session/x"}]}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
	if len(storage.fetched) != 0 {
		t.Errorf("no file may be fetched when any URL is rejected, got %v", storage.fetched)
	}
}

func TestDownloadZip_FlattensAndDeduplicatesEntryNames(t *testing.T) {
	storage := &recordingStorage{}
	e := newZipApp(storage)
	u := `"https://www.googleapis.com/drive/v3/files/a?alt=media"`

	rec := postZip(e, `{"provider":"googledrive","files":[
		{"name":"../../.bashrc","download_url":`+u+`},
		{"name":"..\\..\\evil.exe","download_url":`+u+`},
		{"name":"IMG_1.jpg","download_url":`+u+`},
		{"name":"img_1.jpg","download_url":`+u+`},
		{"name":"..","download_url":`+u+`}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}

	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	want := []string{".bashrc", "evil.exe", "IMG_1.jpg", "img_1 (2).jpg", "photo"}
	if strings.Join(names, "|") != strings.Join(want, "|") {
		t.Errorf("entry names = %q, want %q", names, want)
	}
}

func TestDownloadZip_RejectsMoreThanMaxFiles(t *testing.T) {
	storage := &recordingStorage{}
	e := newZipApp(storage)

	files := make([]string, MaxFilesPerZip+1)
	for i := range files {
		files[i] = `{"name":"a.jpg","download_url":"https://www.googleapis.com/drive/v3/files/a?alt=media"}`
	}
	rec := postZip(e, `{"provider":"googledrive","files":[`+strings.Join(files, ",")+`]}`)

	if rec.Code != http.StatusBadRequest || len(storage.fetched) != 0 {
		t.Errorf("got %d with %d fetches; want 400 and nothing fetched", rec.Code, len(storage.fetched))
	}
}
