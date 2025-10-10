package face

import (
	"all-me-backend/pkg/models"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// StorageService defines the interface for cloud storage operations needed by face service
type StorageService interface {
	ParseShareLink(shareURL string, token *models.Token) (*models.CloudItem, error)
	ListImages(item *models.CloudItem, token *models.Token, recursive bool) ([]*models.CloudItem, error)
	GetFaceRecognitionOptimizedStream(item *models.CloudItem, token *models.Token) (io.ReadCloser, error)
}

// Service handles face comparison operations
type Service struct {
	pythonServiceURL string
	httpClient       *http.Client
	storageService   StorageService
}

// NewService creates a new face service with storage service dependency
func NewService(storageService StorageService) *Service {
	return &Service{
		pythonServiceURL: os.Getenv("FACE_SERVICE_URL"),
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
		storageService: storageService,
	}
}

// RegisterBaseFace registers a base face image with the Python service
func (s *Service) RegisterBaseFace(sessionID string, imageData []byte) error {
	encodedImage := base64.StdEncoding.EncodeToString(imageData)

	payload := pythonRegisterRequest{
		SessionID: sessionID,
		Image:     encodedImage,
	}

	var result pythonRegisterResponse
	if err := s.callPythonService("/face/register", payload, &result); err != nil {
		return err
	}

	if !result.Success {
		if result.Error != "" {
			return errors.New(result.Error)
		}
		return errors.New("registration failed")
	}

	return nil
}

// jobContext stores the context needed to process a job
type jobContext struct {
	allImages []*models.CloudItem
	token     *models.Token
}

var jobContexts = make(map[string]*jobContext)

// CompareFolderImages starts an async comparison job and returns the job ID
func (s *Service) CompareFolderImages(sessionID string, folderLink string, token *models.Token, recursive bool) (string, error) {
	folderItem, err := s.storageService.ParseShareLink(folderLink, token)
	if err != nil {
		return "", fmt.Errorf("failed to parse folder link: %w", err)
	}

	allImages, err := s.storageService.ListImages(folderItem, token, recursive)
	if err != nil {
		return "", fmt.Errorf("failed to list images: %w", err)
	}

	if len(allImages) == 0 {
		return "", fmt.Errorf("no images found in folder")
	}

	encodedImages, err := s.downloadAndEncodeBatch(allImages, token)
	if err != nil {
		return "", fmt.Errorf("failed to download and encode images: %w", err)
	}

	jobID, err := s.StartCompareBatch(sessionID, encodedImages)
	if err != nil {
		return "", err
	}

	jobContexts[jobID] = &jobContext{
		allImages: allImages,
		token:     token,
	}

	return jobID, nil
}

// GetJobStatus retrieves the status of a comparison job
func (s *Service) GetJobStatus(jobID string) (*JobStatusResponse, error) {
	var pythonStatus pythonJobStatusResponse
	url := fmt.Sprintf("/face/job-status/%s", jobID)

	if err := s.callPythonServiceGet(url, &pythonStatus); err != nil {
		return nil, err
	}

	response := &JobStatusResponse{
		JobID:        pythonStatus.JobID,
		Status:       pythonStatus.Status,
		Progress:     pythonStatus.Progress,
		CurrentImage: pythonStatus.CurrentImage,
		TotalImages:  pythonStatus.TotalImages,
		MatchesFound: pythonStatus.MatchesFound,
		Message:      pythonStatus.Message,
		Error:        pythonStatus.Error,
	}

	if pythonStatus.Status == "completed" && pythonStatus.Matches != nil {
		ctx, exists := jobContexts[jobID]
		if exists {
			matchingItems := make([]*models.CloudItem, 0, len(pythonStatus.Matches))
			for _, matchResult := range pythonStatus.Matches {
				if matchResult.Index < len(ctx.allImages) {
					item := ctx.allImages[matchResult.Index]
					// Create a copy and add the match distance
					itemCopy := *item
					itemCopy.MatchDistance = &matchResult.Distance
					matchingItems = append(matchingItems, &itemCopy)
				}
			}
			response.Matches = matchingItems
			delete(jobContexts, jobID)
		}
	}

	return response, nil
}

// downloadAndEncodeBatch downloads images in parallel and encodes them as base64
func (s *Service) downloadAndEncodeBatch(items []*models.CloudItem, token *models.Token) ([]string, error) {
	numWorkers := 10 // Concurrent download limit

	// Pre-allocate results slice to maintain order
	results := make([]string, len(items))

	// Semaphore to limit concurrent downloads
	sem := make(chan struct{}, numWorkers)

	// Error handling
	var firstErr error
	var errOnce sync.Once

	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		go func(idx int, itm *models.CloudItem) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }() // Release semaphore

			// Download - use FaceRecognitionOptimizedURL (800px) for face recognition if available, otherwise DownloadURL
			itemToDownload := itm
			if itm.FaceRecognitionOptimizedURL != "" {
				// Create a copy with FaceRecognitionOptimizedURL as DownloadURL for face recognition
				itemCopy := *itm
				itemCopy.DownloadURL = itm.FaceRecognitionOptimizedURL
				itemToDownload = &itemCopy
			}

			stream, err := s.storageService.GetFaceRecognitionOptimizedStream(itemToDownload, token)
			if err != nil {
				errOnce.Do(func() {
					firstErr = fmt.Errorf("failed to download image %s: %w", itm.Name, err)
				})
				return
			}

			imageData, err := io.ReadAll(stream)
			stream.Close()
			if err != nil {
				errOnce.Do(func() {
					firstErr = fmt.Errorf("failed to read image %s: %w", itm.Name, err)
				})
				return
			}

			// Encode
			encoded := base64.StdEncoding.EncodeToString(imageData)
			results[idx] = encoded
		}(i, item)
	}

	// Wait for all downloads to complete
	wg.Wait()

	// Check if any errors occurred
	if firstErr != nil {
		return nil, firstErr
	}

	return results, nil
}

// StartCompareBatch sends a batch of images to Python service for async comparison
func (s *Service) StartCompareBatch(sessionID string, encodedImages []string) (string, error) {
	payload := pythonCompareBatchRequest{
		SessionID: sessionID,
		Images:    encodedImages,
	}

	var result pythonCompareBatchResponse
	if err := s.callPythonService("/face/compare-batch", payload, &result); err != nil {
		return "", err
	}

	return result.JobID, nil
}

// callPythonService is a generic helper for making HTTP POST calls to the Python service
func (s *Service) callPythonService(endpoint string, payload any, result any) error {
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
		return errors.New("face comparison service request timed out")
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("face comparison service request timed out")
	}

	if strings.Contains(err.Error(), "connection refused") {
		return errors.New("face comparison service unavailable")
	}

	if strings.Contains(err.Error(), "no such host") {
		return errors.New("face comparison service unavailable")
	}

	return errors.New("face comparison service unavailable")
}

// handlePythonServiceError handles non-200 responses from Python service
func handlePythonServiceError(statusCode int, body []byte, result any) error {
	// Try to parse FastAPI error format first ({"detail": "message"})
	var fastAPIError struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(body, &fastAPIError); err == nil && fastAPIError.Detail != "" {
		return errors.New(fastAPIError.Detail)
	}

	// Try to parse our custom error format ({"error": "message"})
	if err := json.Unmarshal(body, result); err == nil {
		switch v := result.(type) {
		case *pythonRegisterResponse:
			if v.Error != "" {
				return errors.New(v.Error)
			}
		case *pythonCompareBatchResponse:
			if v.Error != "" {
				return errors.New(v.Error)
			}
		}
	}

	// Fallback to generic error messages based on status code
	switch statusCode {
	case http.StatusBadRequest:
		return errors.New("invalid request to face comparison service")
	case http.StatusNotFound:
		return errors.New("session not found")
	case http.StatusInternalServerError:
		return errors.New("face comparison service encountered an error")
	case http.StatusServiceUnavailable:
		return errors.New("face comparison service unavailable")
	default:
		return fmt.Errorf("face comparison service returned status %d", statusCode)
	}
}
