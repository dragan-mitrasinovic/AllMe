package face

import (
	"all-me-backend/pkg/models"
	"sync"
	"time"
)

type jobContext struct {
	allImages    []*models.CloudItem
	token        *models.Token
	createdAt    time.Time
	status       string
	currentImage int
	totalImages  int
	matchesFound int
	matches      []pythonMatchResult
	errorMessage string
}

// JobManager manages job contexts for face comparison operations
// It provides thread-safe storage and retrieval of job contexts
type JobManager struct {
	contexts map[string]*jobContext
	mu       sync.RWMutex
}

func NewJobManager() *JobManager {
	jm := &JobManager{
		contexts: make(map[string]*jobContext),
	}

	go jm.cleanupExpiredJobs()

	return jm
}

func (jm *JobManager) cleanupExpiredJobs() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		jm.mu.Lock()
		now := time.Now()
		for jobID, ctx := range jm.contexts {
			// Remove contexts older than 24 hours
			if now.Sub(ctx.createdAt) > 24*time.Hour {
				delete(jm.contexts, jobID)
			}
		}
		jm.mu.Unlock()
	}
}

func (jm *JobManager) Store(jobID string, allImages []*models.CloudItem, token *models.Token) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	jm.contexts[jobID] = &jobContext{
		allImages:    allImages,
		token:        token,
		createdAt:    time.Now(),
		status:       "processing",
		totalImages:  len(allImages),
		currentImage: 0,
		matchesFound: 0,
	}
}

func (jm *JobManager) UpdateProgress(jobID string, currentImage, totalImages, matchesFound int) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if ctx, exists := jm.contexts[jobID]; exists {
		ctx.currentImage = currentImage
		ctx.totalImages = totalImages
		ctx.matchesFound = matchesFound
	}
}

func (jm *JobManager) MarkCompleted(jobID string, matches []pythonMatchResult) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if ctx, exists := jm.contexts[jobID]; exists {
		ctx.status = "completed"
		ctx.matches = matches
		ctx.matchesFound = len(matches)
		ctx.currentImage = ctx.totalImages
	}
}

func (jm *JobManager) MarkFailed(jobID string, errorMessage string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if ctx, exists := jm.contexts[jobID]; exists {
		ctx.status = "failed"
		ctx.errorMessage = errorMessage
	}
}

func (jm *JobManager) Get(jobID string) (*jobContext, bool) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	ctx, exists := jm.contexts[jobID]
	return ctx, exists
}

func (jm *JobManager) Delete(jobID string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	delete(jm.contexts, jobID)
}
