package face

import (
	"all-me-backend/internal/session"
	"all-me-backend/internal/storage"
	"all-me-backend/pkg/models"
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Service struct {
	pythonServiceURL string
	httpClient       *http.Client
	storageService   StorageService
	jobManager       *JobManager
}

func NewService(storageService StorageService) *Service {
	return &Service{
		pythonServiceURL: os.Getenv("FACE_SERVICE_URL"),
		httpClient: &http.Client{
			Timeout: 60 * time.Minute,
		},
		storageService: storageService,
		jobManager:     NewJobManager(),
	}
}

// RegisterBaseFace registers a base face image with the Python service
// This image is used as the reference for future comparisons in a given session
func (s *Service) RegisterBaseFace(sessionID string, imageData []byte) error {
	encodedImage := base64.StdEncoding.EncodeToString(imageData)

	payload := pythonRegisterRequest{
		SessionID: sessionID,
		Image:     encodedImage,
	}

	var result pythonRegisterResponse
	if err := s.callPythonServicePost("/face/register", payload, &result); err != nil {
		return err
	}

	if !result.Success {
		if result.Error != "" {
			// Map Python service errors to custom error types
			if strings.Contains(strings.ToLower(result.Error), "no face detected") {
				return ErrNoFaceDetected
			}
			if strings.Contains(strings.ToLower(result.Error), "multiple faces") {
				return ErrMultipleFaces
			}
			return fmt.Errorf("%w: %s", ErrInvalidImageFormat, result.Error)
		}
		return ErrInvalidImageFormat
	}

	return nil
}

// CompareFolderImages starts an async comparison job and returns the job ID
func (s *Service) CompareFolderImages(sessionID string, folderLink string, token *models.Token, recursive bool) (string, error) {
	folderItem, err := s.storageService.ParseShareLink(folderLink, token)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidFolderLink, err)
	}

	allImages, err := s.storageService.ListImages(folderItem, token, recursive)
	if errors.Is(err, storage.ErrTooManyImages) {
		return "", ErrTooManyImages
	}
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrFolderAccess, err)
	}

	if len(allImages) == 0 {
		return "", ErrNoImages
	}

	jobID, err := s.processFolderInBatches(sessionID, allImages, token)
	if err != nil {
		return "", err
	}

	return jobID, nil
}

// GetJobStatus retrieves the status of a comparison job. Jobs are only visible to the session that
// started them; any other session gets ErrJobNotFound.
func (s *Service) GetJobStatus(jobID, sessionID string) (*JobStatusResponse, error) {
	ctx, exists := s.jobManager.Get(jobID)
	if !exists || sessionID == "" || subtle.ConstantTimeCompare([]byte(ctx.ownerSessionID), []byte(sessionID)) != 1 {
		return nil, ErrJobNotFound
	}
	s.jobManager.Touch(jobID)

	// Return status from our job manager
	response := &JobStatusResponse{
		JobID:        jobID,
		Status:       ctx.status,
		CurrentImage: ctx.currentImage,
		TotalImages:  ctx.totalImages,
		MatchesFound: ctx.matchesFound,
		Error:        ctx.errorMessage,
	}

	// Calculate progress percentage
	if ctx.totalImages > 0 {
		response.Progress = (ctx.currentImage * 100) / ctx.totalImages
	}

	// Set message
	if ctx.status == "processing" {
		response.Message = fmt.Sprintf("Processing image %d of %d", ctx.currentImage, ctx.totalImages)
	} else if ctx.status == "completed" {
		response.Message = fmt.Sprintf("Completed! Found %d matches", ctx.matchesFound)
	} else if ctx.status == "failed" {
		response.Message = ctx.errorMessage
	}

	// Map matches to cloud items if completed
	if ctx.status == "completed" && ctx.matches != nil {
		matchingItems := make([]*models.CloudItem, 0, len(ctx.matches))
		for _, matchResult := range ctx.matches {
			if matchResult.Index < len(ctx.allImages) {
				item := ctx.allImages[matchResult.Index]
				// Create a copy and add the match distance
				itemCopy := *item
				itemCopy.MatchDistance = &matchResult.Distance
				matchingItems = append(matchingItems, &itemCopy)
			}
		}
		response.Matches = matchingItems

		// Clean up the job context after successful completion
		s.jobManager.Delete(jobID)
	}

	// Also clean up on error or failed status
	if ctx.status == "failed" || ctx.status == "error" {
		s.jobManager.Delete(jobID)
	}

	return response, nil
}

// downloadAndEncodeBatch downloads images in parallel using a worker pool and encodes them as base64
func (s *Service) downloadAndEncodeBatch(items []*models.CloudItem, token *models.Token) ([]string, error) {
	const numWorkers = 10

	// Pre-allocate results slice to maintain order
	results := make([]string, len(items))

	// Channel for work items
	type job struct {
		index int
		item  *models.CloudItem
	}
	jobs := make(chan job, len(items))

	// Channel for errors
	type result struct {
		index   int
		encoded string
		err     error
	}
	resultsChan := make(chan result, len(items))

	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				encoded, err := s.downloadAndEncodeImage(j.item, token)
				resultsChan <- result{
					index:   j.index,
					encoded: encoded,
					err:     err,
				}
			}
		}()
	}

	// Send jobs to workers
	for i, item := range items {
		jobs <- job{index: i, item: item}
	}
	close(jobs)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var firstErr error
	for res := range resultsChan {
		if res.err != nil && firstErr == nil {
			firstErr = res.err
		}
		if res.err == nil {
			results[res.index] = res.encoded
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}

	return results, nil
}

// downloadAndEncodeImage downloads a single image and encodes it to base64
func (s *Service) downloadAndEncodeImage(item *models.CloudItem, token *models.Token) (string, error) {
	// Use FaceRecognitionOptimizedURL if available, otherwise use DownloadURL
	itemToDownload := item
	if item.FaceRecognitionOptimizedURL != "" {
		itemCopy := *item
		itemCopy.DownloadURL = item.FaceRecognitionOptimizedURL
		itemToDownload = &itemCopy
	}

	stream, err := s.storageService.GetFaceRecognitionOptimizedStream(itemToDownload, token)
	if err != nil {
		return "", fmt.Errorf("failed to download image %s: %w", item.Name, err)
	}
	defer stream.Close()

	imageData, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("failed to read image %s: %w", item.Name, err)
	}

	return base64.StdEncoding.EncodeToString(imageData), nil
}

// processFolderInBatches creates a job for the client and processes its images in the background
func (s *Service) processFolderInBatches(sessionID string, allImages []*models.CloudItem, token *models.Token) (string, error) {
	// Create a unified job ID for the client
	jobID, err := session.NewID()
	if err != nil {
		return "", fmt.Errorf("failed to create job ID: %w", err)
	}
	unifiedJobID := "job-" + jobID

	// Store the job context
	s.jobManager.Store(unifiedJobID, sessionID, allImages, token)

	// Process batches in the background
	go s.processBatchesBackground(unifiedJobID, sessionID, allImages, token)

	return unifiedJobID, nil
}

const (
	batchSize = 100
	// A single batch of batchSize images; the face-service may be busy with other searches' batches too.
	batchTimeout = 30 * time.Minute
	// The browser polls the job status every 1.5 s; a job nobody has polled for this long was abandoned.
	abandonAfter = 2 * time.Minute
)

var errJobAbandoned = errors.New("job abandoned")

// processBatchesBackground downloads and compares the images one batch at a time: the next batch is only
// downloaded and sent once the face-service has finished the previous one. Memory per search stays at about
// one batch, and concurrent searches share the face-service's workers. A job nobody polls any more stops.
func (s *Service) processBatchesBackground(jobID, sessionID string, allImages []*models.CloudItem, token *models.Token) {
	totalImages := len(allImages)
	var allMatches []pythonMatchResult

	for offset := 0; offset < totalImages; offset += batchSize {
		if s.jobManager.IsAbandoned(jobID, abandonAfter) {
			s.jobManager.Delete(jobID)
			return
		}

		end := min(offset+batchSize, totalImages)
		encodedImages, err := s.downloadAndEncodeBatch(allImages[offset:end], token)
		if err != nil {
			s.failJob(jobID, "Couldn't download some photos from your cloud storage. Please try again.", err)
			return
		}

		pythonJobID, err := s.startPythonCompareBatch(sessionID, encodedImages)
		if err != nil {
			s.failJob(jobID, jobFailureMessage(err, "Face matching isn't available right now. Please try again in a moment."), err)
			return
		}

		matches, err := s.waitForPythonBatch(jobID, pythonJobID, offset, totalImages, len(allMatches))
		if errors.Is(err, errJobAbandoned) {
			s.jobManager.Delete(jobID)
			return
		}
		if err != nil {
			s.failJob(jobID, jobFailureMessage(err, "Face matching failed. Please try again."), err)
			return
		}
		allMatches = append(allMatches, matches...)
	}

	s.jobManager.MarkCompleted(jobID, allMatches)
}

// jobFailureMessage picks the message for a failed job: when the face-service no longer has the reference
// photo (it expired or the service restarted), retrying can't help, so the user is asked to upload it again.
func jobFailureMessage(err error, fallback string) string {
	if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrNoBaseFace) {
		return msgUploadPhotoAgain
	}
	return fallback
}

// failJob marks the job failed with a user-facing message and logs the underlying error.
func (s *Service) failJob(jobID, public string, err error) {
	log.Printf("face job %s failed: %v", jobID, err)
	s.jobManager.MarkFailed(jobID, public)
}

// waitForPythonBatch polls one face-service batch until it finishes, reporting overall progress, and returns
// its matches with indices shifted to positions in the whole job.
func (s *Service) waitForPythonBatch(jobID, pythonJobID string, offset, totalImages, matchesSoFar int) ([]pythonMatchResult, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(batchTimeout)

	for {
		select {
		case <-timeout:
			return nil, errors.New("Processing timeout")
		case <-ticker.C:
		}

		var status pythonJobStatusResponse
		if err := s.callPythonServiceGet(fmt.Sprintf("/face/job-status/%s", pythonJobID), &status); err != nil {
			return nil, fmt.Errorf("Failed to get job status: %v", err)
		}

		switch status.Status {
		case "failed", "error":
			if status.Error == "Session not found" {
				return nil, fmt.Errorf("%w: face-service job %s", ErrSessionNotFound, pythonJobID)
			}
			return nil, fmt.Errorf("face-service job %s failed: %s", pythonJobID, status.Error)
		case "completed":
			matches := make([]pythonMatchResult, 0, len(status.Matches))
			for _, match := range status.Matches {
				matches = append(matches, pythonMatchResult{Index: match.Index + offset, Distance: match.Distance})
			}
			s.jobManager.UpdateProgress(jobID, offset+status.TotalImages, totalImages, matchesSoFar+len(matches))
			return matches, nil
		}

		s.jobManager.UpdateProgress(jobID, offset+status.CurrentImage, totalImages, matchesSoFar+status.MatchesFound)
		if s.jobManager.IsAbandoned(jobID, abandonAfter) {
			return nil, errJobAbandoned
		}
	}
}

// startPythonCompareBatch sends a batch of images to Python service for async comparison
func (s *Service) startPythonCompareBatch(sessionID string, encodedImages []string) (string, error) {
	payload := pythonCompareBatchRequest{
		SessionID: sessionID,
		Images:    encodedImages,
	}

	var result pythonCompareBatchResponse
	if err := s.callPythonServicePost("/face/compare-batch", payload, &result); err != nil {
		return "", err
	}

	return result.JobID, nil
}

// callPythonServicePost is a generic helper for making HTTP POST calls to the Python service
func (s *Service) callPythonServicePost(endpoint string, payload any, result any) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := s.pythonServiceURL + endpoint

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return handleNetworkError(err, url)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response from face comparison service")
	}

	if resp.StatusCode != http.StatusOK {
		return handlePythonServiceError(resp.StatusCode, body, result)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to parse response from face comparison service")
	}

	return nil
}

// callPythonServiceGet is a helper for making HTTP GET calls to the Python service
func (s *Service) callPythonServiceGet(endpoint string, result any) error {
	url := s.pythonServiceURL + endpoint

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return handleNetworkError(err, url)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response from face comparison service")
	}

	if resp.StatusCode != http.StatusOK {
		return handlePythonServiceError(resp.StatusCode, body, result)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to parse response from face comparison service")
	}

	return nil
}

// ClearReferenceImage clears the reference face image for a session
func (s *Service) ClearReferenceImage(sessionID string) error {
	url := fmt.Sprintf("%s/face/session/%s", s.pythonServiceURL, sessionID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return handleNetworkError(err, url)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to clear reference image")
	}

	return nil
}

// handleNetworkError provides user-friendly error messages for network errors
func handleNetworkError(err error, url string) error {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrTimeout
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}

	if strings.Contains(err.Error(), "connection refused") {
		return ErrServiceUnavailable
	}

	if strings.Contains(err.Error(), "no such host") {
		return ErrServiceUnavailable
	}

	return ErrServiceUnavailable
}

// handlePythonServiceError handles non-200 responses from Python service
func handlePythonServiceError(statusCode int, body []byte, result any) error {
	// Try to parse FastAPI error format first ({"detail": "message"})
	var fastAPIError struct {
		Detail string `json:"detail"`
	}

	if err := json.Unmarshal(body, &fastAPIError); err == nil && fastAPIError.Detail != "" {
		errorMsg := fastAPIError.Detail
		// Map known error messages to custom error types
		if strings.Contains(strings.ToLower(errorMsg), "no base face") {
			return ErrNoBaseFace
		}
		if strings.Contains(strings.ToLower(errorMsg), "session not found") {
			return ErrSessionNotFound
		}
		if strings.Contains(strings.ToLower(errorMsg), "no face detected") {
			return ErrNoFaceDetected
		}
		if strings.Contains(strings.ToLower(errorMsg), "multiple faces") {
			return ErrMultipleFaces
		}
		return errors.New(errorMsg)
	}

	// Try to parse our custom error format ({"error": "message"})
	if err := json.Unmarshal(body, result); err == nil {
		switch v := result.(type) {
		case *pythonRegisterResponse:
			if v.Error != "" {
				if strings.Contains(strings.ToLower(v.Error), "no face detected") {
					return ErrNoFaceDetected
				}
				if strings.Contains(strings.ToLower(v.Error), "multiple faces") {
					return ErrMultipleFaces
				}
				return errors.New(v.Error)
			}
		case *pythonCompareBatchResponse:
			if v.Error != "" {
				if strings.Contains(strings.ToLower(v.Error), "no base face") {
					return ErrNoBaseFace
				}
				return errors.New(v.Error)
			}
		}
	}

	// Fallback to generic error messages based on status code
	switch statusCode {
	case http.StatusBadRequest:
		return ErrInvalidImageFormat
	case http.StatusNotFound:
		return ErrSessionNotFound
	case http.StatusInternalServerError:
		return ErrServiceUnavailable
	case http.StatusServiceUnavailable:
		return ErrServiceUnavailable
	default:
		return fmt.Errorf("%w: service returned status %d", ErrServiceUnavailable, statusCode)
	}
}
