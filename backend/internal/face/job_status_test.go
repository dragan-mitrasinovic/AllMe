package face

import (
	"all-me-backend/pkg/models"
	"errors"
	"testing"
)

func TestGetJobStatus_OnlyOwnerSessionSeesJob(t *testing.T) {
	s := &Service{jobManager: NewJobManager()}
	s.jobManager.Store("job-1", "owner-session", []*models.CloudItem{{ID: "a"}}, &models.Token{})

	for _, other := range []string{"other-session", ""} {
		if _, err := s.GetJobStatus("job-1", other); !errors.Is(err, ErrJobNotFound) {
			t.Errorf("session %q: expected ErrJobNotFound, got %v", other, err)
		}
	}

	status, err := s.GetJobStatus("job-1", "owner-session")
	if err != nil {
		t.Fatalf("owner: unexpected error %v", err)
	}
	if status.Status != "processing" || status.TotalImages != 1 {
		t.Errorf("owner: unexpected status %+v", status)
	}

	if _, err := s.GetJobStatus("unknown-job", "owner-session"); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("unknown job: expected ErrJobNotFound, got %v", err)
	}
}
