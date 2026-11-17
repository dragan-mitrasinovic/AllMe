package face

import (
	"all-me-backend/pkg/models"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeStorage struct{}

func (fakeStorage) ParseShareLink(string, *models.Token) (*models.CloudItem, error) { return nil, nil }
func (fakeStorage) ListImages(*models.CloudItem, *models.Token, bool) ([]*models.CloudItem, error) {
	return nil, nil
}
func (fakeStorage) GetFaceRecognitionOptimizedStream(*models.CloudItem, *models.Token) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("jpeg")), nil
}

// fakeFaceService completes each batch after a few status polls and matches the first image of every batch.
type fakeFaceService struct {
	mu          sync.Mutex
	inFlight    int
	maxInFlight int
	batches     int
	polls       map[string]int
	sizes       map[string]int
}

func (f *fakeFaceService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if r.URL.Path == "/face/compare-batch" {
		var req pythonCompareBatchRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.batches++
		id := fmt.Sprint("py-", f.batches)
		f.sizes[id] = len(req.Images)
		f.inFlight++
		f.maxInFlight = max(f.maxInFlight, f.inFlight)
		_ = json.NewEncoder(w).Encode(pythonCompareBatchResponse{JobID: id, Status: "processing"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/face/job-status/")
	f.polls[id]++
	status := pythonJobStatusResponse{JobID: id, Status: "processing", TotalImages: f.sizes[id]}
	if f.polls[id] >= 2 {
		f.inFlight--
		status.Status = "completed"
		status.CurrentImage = f.sizes[id]
		status.Matches = []pythonMatchResult{{Index: 0, Distance: 0.3}}
	}
	_ = json.NewEncoder(w).Encode(status)
}

func newPacingService(t *testing.T) (*Service, *fakeFaceService) {
	t.Helper()
	fake := &fakeFaceService{polls: map[string]int{}, sizes: map[string]int{}}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	return &Service{
		pythonServiceURL: server.URL,
		httpClient:       server.Client(),
		storageService:   fakeStorage{},
		jobManager:       NewJobManager(),
	}, fake
}

func images(n int) []*models.CloudItem {
	items := make([]*models.CloudItem, n)
	for i := range items {
		items[i] = &models.CloudItem{ID: fmt.Sprint(i), Name: fmt.Sprint(i, ".jpg")}
	}
	return items
}

func TestProcessBatches_SendsOneBatchAtATime(t *testing.T) {
	s, fake := newPacingService(t)
	all := images(2*batchSize + 50)
	s.jobManager.Store("job", "owner", all, &models.Token{})

	s.processBatchesBackground("job", "owner", all, &models.Token{})

	if fake.batches != 3 || fake.maxInFlight != 1 {
		t.Errorf("got %d batches with up to %d in flight; want 3 batches, 1 at a time", fake.batches, fake.maxInFlight)
	}
	status, err := s.GetJobStatus("job", "owner")
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "completed" || status.CurrentImage != len(all) {
		t.Errorf("unexpected final status %+v", status)
	}
	var got []string
	for _, m := range status.Matches {
		got = append(got, m.ID)
	}
	// The first image of each batch, at its position in the whole job
	if want := fmt.Sprint([]string{"0", "100", "200"}); fmt.Sprint(got) != want {
		t.Errorf("matches = %v, want %v", got, want)
	}
}

func TestProcessBatches_StopsWhenNobodyPolls(t *testing.T) {
	s, fake := newPacingService(t)
	all := images(10)
	s.jobManager.Store("job", "owner", all, &models.Token{})
	ctx, _ := s.jobManager.Get("job")
	ctx.lastPolled = time.Now().Add(-abandonAfter - time.Second)

	s.processBatchesBackground("job", "owner", all, &models.Token{})

	if fake.batches != 0 {
		t.Errorf("an abandoned job must not send batches, sent %d", fake.batches)
	}
	if _, exists := s.jobManager.Get("job"); exists {
		t.Error("an abandoned job should be removed")
	}
}

func TestErrorResponses_NeverIncludeErrorDetails(t *testing.T) {
	for _, err := range []error{
		fmt.Errorf("%w: Traceback (most recent call last): PIL.UnidentifiedImageError", ErrInvalidImageFormat),
		fmt.Errorf("%w: GET https://www.googleapis.com/drive/v3/files/secret: 404", ErrInvalidFolderLink),
		fmt.Errorf("%w: status 500 from http://face-service:8081", ErrServiceUnavailable),
		errors.New("dial tcp 172.18.0.3:8081: connection refused"),
	} {
		msg := GetErrorResponse(err).Message
		for _, secret := range []string{"Traceback", "googleapis", "secret", "face-service", "172.18", "PIL"} {
			if strings.Contains(msg, secret) {
				t.Errorf("message for %v leaks %q: %s", err, secret, msg)
			}
		}
	}
}

func TestProcessBatches_FailedSearchShowsFriendlyMessage(t *testing.T) {
	s, _ := newPacingService(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/face/compare-batch" {
			_ = json.NewEncoder(w).Encode(pythonCompareBatchResponse{JobID: "py-1", Status: "processing"})
			return
		}
		_ = json.NewEncoder(w).Encode(pythonJobStatusResponse{Status: "failed", Error: "Traceback: MemoryError in /app/main.py"})
	}))
	defer server.Close()
	s.pythonServiceURL = server.URL

	all := images(3)
	s.jobManager.Store("job", "owner", all, &models.Token{})
	s.processBatchesBackground("job", "owner", all, &models.Token{})

	status, err := s.GetJobStatus("job", "owner")
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "failed" || strings.Contains(status.Message+status.Error, "Traceback") ||
		status.Message != "Face matching failed. Please try again." {
		t.Errorf("unexpected failed status %+v", status)
	}
}

func TestProcessBatches_ExpiredReferencePhotoAsksToUploadAgain(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"rejected batch": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Session not found"})
		},
		"failed during matching": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/face/compare-batch" {
				_ = json.NewEncoder(w).Encode(pythonCompareBatchResponse{JobID: "py-1", Status: "processing"})
				return
			}
			_ = json.NewEncoder(w).Encode(pythonJobStatusResponse{Status: "failed", Error: "Session not found"})
		},
	} {
		s, _ := newPacingService(t)
		server := httptest.NewServer(handler)
		s.pythonServiceURL = server.URL

		all := images(3)
		s.jobManager.Store("job", "owner", all, &models.Token{})
		s.processBatchesBackground("job", "owner", all, &models.Token{})
		server.Close()

		status, err := s.GetJobStatus("job", "owner")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if status.Status != "failed" || status.Message != msgUploadPhotoAgain {
			t.Errorf("%s: got %q / %q, want the upload-again message", name, status.Status, status.Message)
		}
	}
}
