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
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrFolderAccess, err)
	}

	if len(allImages) == 0 {
		return "", fmt.Errorf("%w: no images found in folder", ErrFolderAccess)
	}

	// Process images in batches of 100
	jobID, err := s.processFolderInBatches(sessionID, allImages, token)
	if err != nil {
		return "", err
	}

	return jobID, nil
}

// GetJobStatus retrieves the status of a comparison job
func (s *Service) GetJobStatus(jobID string) (*JobStatusResponse, error) {
	// Check if this is a batch job managed by Go
	ctx, isBatchJob := s.jobManager.Get(jobID)

	if isBatchJob {
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
			response.Message = fmt.Sprintf("Failed: %s", ctx.errorMessage)
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

	// Fall back to querying Python service directly (for backwards compatibility)
	var pythonStatus pythonJobStatusResponse
	url := fmt.Sprintf("/face/job-status/%s", jobID)

	if err := s.callPythonServiceGet(url, &pythonStatus); err != nil {
		// Check if it's a "not found" error
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "session not found") {
			return nil, ErrJobNotFound
		}
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

// processFolderInBatches processes images in batches of 100 and creates a unified job
func (s *Service) processFolderInBatches(sessionID string, allImages []*models.CloudItem, token *models.Token) (string, error) {
	// Create a unified job ID for the client
	unifiedJobID := fmt.Sprintf("batch-%d-%s", time.Now().Unix(), sessionID)

	// Store the job context
	s.jobManager.Store(unifiedJobID, allImages, token)

	// Process batches in the background
	go s.processBatchesBackground(unifiedJobID, sessionID, allImages, token)

	return unifiedJobID, nil
}

// processBatchesBackground downloads and processes all image batches
func (s *Service) processBatchesBackground(unifiedJobID, sessionID string, allImages []*models.CloudItem, token *models.Token) {
	const batchSize = 100
	totalImages := len(allImages)

	// Split images into batches and send each to Python service
	var pythonJobIDs []string
	var batchOffsets []int // Track the starting index of each batch

	for i := 0; i < totalImages; i += batchSize {
		end := i + batchSize
		if end > totalImages {
			end = totalImages
		}

		batch := allImages[i:end]

		// Download and encode this batch
		encodedImages, err := s.downloadAndEncodeBatch(batch, token)
		if err != nil {
			// Mark job as failed
			s.jobManager.MarkFailed(unifiedJobID, fmt.Sprintf("Failed to download batch: %v", err))
			return
		}

		// Send batch to Python service
		pythonJobID, err := s.startPythonCompareBatch(sessionID, encodedImages)
		if err != nil {
			s.jobManager.MarkFailed(unifiedJobID, fmt.Sprintf("Failed to start Python job: %v", err))
			return
		}

		pythonJobIDs = append(pythonJobIDs, pythonJobID)
		batchOffsets = append(batchOffsets, i)
	}

	// Poll all Python jobs and aggregate results
	s.aggregateBatchResults(unifiedJobID, pythonJobIDs, batchOffsets, totalImages)
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

// aggregateBatchResults polls Python jobs and combines their results
func (s *Service) aggregateBatchResults(unifiedJobID string, pythonJobIDs []string, batchOffsets []int, totalImages int) {
	// Track completion of all Python jobs
	completedJobs := make(map[string]*pythonJobStatusResponse)

	// Poll until all jobs complete or one fails
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(60 * time.Minute)

	for {
		select {
		case <-timeout:
			s.jobManager.MarkFailed(unifiedJobID, "Processing timeout")
			return
		case <-ticker.C:
			allComplete := true
			var totalProcessed int
			var totalMatches int
			var failedJob string

			// Check status of each Python job
			for _, pythonJobID := range pythonJobIDs {
				if _, exists := completedJobs[pythonJobID]; exists {
					continue
				}

				var status pythonJobStatusResponse
				url := fmt.Sprintf("/face/job-status/%s", pythonJobID)
				if err := s.callPythonServiceGet(url, &status); err != nil {
					s.jobManager.MarkFailed(unifiedJobID, fmt.Sprintf("Failed to get job status: %v", err))
					return
				}

				if status.Status == "failed" || status.Status == "error" {
					failedJob = status.Error
					break
				}

				if status.Status == "completed" {
					completedJobs[pythonJobID] = &status
				} else {
					allComplete = false
				}

				// Update progress - add current batch progress
				totalProcessed += status.CurrentImage
				totalMatches += status.MatchesFound
			}

			// Update unified job progress
			s.jobManager.UpdateProgress(unifiedJobID, totalProcessed, totalImages, totalMatches)

			if failedJob != "" {
				s.jobManager.MarkFailed(unifiedJobID, failedJob)
				return
			}

			if allComplete {
				// Aggregate all matches with adjusted indices
				var allMatches []pythonMatchResult
				for idx, pythonJobID := range pythonJobIDs {
					jobResult := completedJobs[pythonJobID]
					offset := batchOffsets[idx]

					// Adjust match indices to global positions
					for _, match := range jobResult.Matches {
						adjustedMatch := pythonMatchResult{
							Index:    match.Index + offset,
							Distance: match.Distance,
						}
						allMatches = append(allMatches, adjustedMatch)
					}
				}

				s.jobManager.MarkCompleted(unifiedJobID, allMatches)
				return
			}
		}
	}
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
