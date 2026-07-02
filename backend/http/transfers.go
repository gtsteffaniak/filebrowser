package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/events"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/preview"
	"github.com/gtsteffaniak/go-logger/logger"
)

type TransferStatus string

const (
	TransferStatusPending     TransferStatus = "pending"
	TransferStatusCalculating TransferStatus = "calculating"
	TransferStatusRunning     TransferStatus = "running"
	TransferStatusCompleted   TransferStatus = "completed"
	TransferStatusFailed      TransferStatus = "failed"
	TransferStatusCancelled   TransferStatus = "cancelled"
)

type TransferJob struct {
	ID             string         `json:"id"`
	Action         string         `json:"action"`
	Items          []MoveCopyItem `json:"items"`
	Status         TransferStatus `json:"status"`
	TotalBytes     int64          `json:"totalBytes"`
	CopiedBytes    int64          `json:"copiedBytes"`
	CurrentFile    string         `json:"currentFile"`
	ItemsTotal     int            `json:"itemsTotal"`
	ItemsCompleted int            `json:"itemsCompleted"`
	Error          string         `json:"error,omitempty"`
	Username       string         `json:"username"`
	CreatedAt      time.Time      `json:"createdAt"`
	CompletedAt    *time.Time     `json:"completedAt,omitempty"`
	cancelFunc     context.CancelFunc
	mu             sync.Mutex
}

type TransferManager struct {
	mu       sync.RWMutex
	jobs     map[string]*TransferJob
	userSems map[string]chan struct{}
}

type resolvedTransferParams struct {
	action   string
	srcIndex string
	dstIndex string
	realSrc  string
	realDst  string
	isSrcDir bool
	item      MoveCopyItem
	userScope string
	showHidden   bool
	hideFileExt  string
}

var transferMgr = &TransferManager{
	jobs:     make(map[string]*TransferJob),
	userSems: make(map[string]chan struct{}),
}

func init() {
	go transferCleanupLoop()
}

func transferCleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		transferMgr.cleanupCompleted(10 * time.Minute)
	}
}

func (tm *TransferManager) CreateJob(action, username string, items []MoveCopyItem, params []resolvedTransferParams) *TransferJob {
	job := &TransferJob{
		ID:         uuid.New().String(),
		Action:     action,
		Items:      items,
		Status:     TransferStatusPending,
		ItemsTotal: len(params),
		Username:   username,
		CreatedAt:  time.Now(),
	}
	tm.mu.Lock()
	tm.jobs[job.ID] = job
	tm.mu.Unlock()
	return job
}

func (tm *TransferManager) GetJob(id string) *TransferJob {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.jobs[id]
}

func (tm *TransferManager) GetJobsForUser(username string) []*TransferJob {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	var result []*TransferJob
	for _, job := range tm.jobs {
		if job.Username == username {
			result = append(result, job)
		}
	}
	return result
}

func (tm *TransferManager) CancelJob(id string) error {
	tm.mu.RLock()
	job, ok := tm.jobs[id]
	tm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("job not found")
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.Status != TransferStatusRunning && job.Status != TransferStatusCalculating && job.Status != TransferStatusPending {
		return fmt.Errorf("job is not active")
	}
	if job.cancelFunc != nil {
		job.cancelFunc()
	}
	job.Status = TransferStatusCancelled
	now := time.Now()
	job.CompletedAt = &now
	return nil
}

func (tm *TransferManager) getUserSem(username string) chan struct{} {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if _, ok := tm.userSems[username]; !ok {
		tm.userSems[username] = make(chan struct{}, 1)
	}
	return tm.userSems[username]
}

func (tm *TransferManager) cleanupCompleted(maxAge time.Duration) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	for id, job := range tm.jobs {
		if job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
			delete(tm.jobs, id)
		}
	}
}

func (tm *TransferManager) RunJob(job *TransferJob, params []resolvedTransferParams) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("[TRANSFER] RunJob panicked: %v", r)
			job.mu.Lock()
			job.Status = TransferStatusFailed
			job.Error = fmt.Sprintf("internal error: %v", r)
			now := time.Now()
			job.CompletedAt = &now
			job.mu.Unlock()
			sendTransferEvent(job)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	job.mu.Lock()
	job.cancelFunc = cancel
	// Status stays "pending" while the job waits for its queue slot.
	job.mu.Unlock()
	sendTransferEvent(job)

	// Acquire per-user semaphore so only one transfer runs at a time per user.
	sem := tm.getUserSem(job.Username)
	select {
	case sem <- struct{}{}:
		// acquired slot
	case <-ctx.Done():
		job.mu.Lock()
		if job.Status == TransferStatusPending {
			job.Status = TransferStatusCancelled
			now := time.Now()
			job.CompletedAt = &now
		}
		job.mu.Unlock()
		sendTransferEvent(job)
		return
	}
	defer func() { <-sem }()

	job.mu.Lock()
	job.Status = TransferStatusCalculating
	job.mu.Unlock()
	logger.Infof("[TRANSFER] Job %s: status=calculating, items=%d", job.ID, len(params))
	sendTransferEvent(job)

	// Phase 1: Calculate total size
	var totalBytes int64
	for _, p := range params {
		select {
		case <-ctx.Done():
			job.mu.Lock()
			job.Status = TransferStatusCancelled
			now := time.Now()
			job.CompletedAt = &now
			job.mu.Unlock()
			sendTransferEvent(job)
			return
		default:
		}

		info, err := os.Stat(p.realSrc)
		if err != nil {
			job.mu.Lock()
			job.Status = TransferStatusFailed
			job.Error = fmt.Sprintf("cannot stat source: %v", err)
			now := time.Now()
			job.CompletedAt = &now
			job.mu.Unlock()
			sendTransferEvent(job)
			return
		}

		if info.IsDir() {
			size, err := fileutils.CalculateTotalSize(p.realSrc)
			if err != nil {
				job.mu.Lock()
				job.Status = TransferStatusFailed
				job.Error = fmt.Sprintf("cannot calculate size: %v", err)
				now := time.Now()
				job.CompletedAt = &now
				job.mu.Unlock()
				sendTransferEvent(job)
				return
			}
			totalBytes += size
		} else {
			totalBytes += info.Size()
		}
	}

	job.mu.Lock()
	job.TotalBytes = totalBytes
	job.Status = TransferStatusRunning
	job.mu.Unlock()
	logger.Infof("[TRANSFER] Job %s: status=running, totalBytes=%d", job.ID, totalBytes)
	sendTransferEvent(job)

	// Phase 2: Execute operations
	var lastSendTime time.Time
	var cumulativeBytes int64

	for i, p := range params {
		select {
		case <-ctx.Done():
			job.mu.Lock()
			job.Status = TransferStatusCancelled
			now := time.Now()
			job.CompletedAt = &now
			job.mu.Unlock()
			sendTransferEvent(job)
			return
		default:
		}

		job.mu.Lock()
		job.CurrentFile = filepath.Base(p.realSrc)
		job.mu.Unlock()

		// Pre-calculate this item's size before potential move deletes it
		var itemSize int64
		info, _ := os.Stat(p.realSrc)
		if info != nil {
			if info.IsDir() {
				itemSize, _ = fileutils.CalculateTotalSize(p.realSrc)
			} else {
				itemSize = info.Size()
			}
		}

		// Track per-item progress relative to cumulative total
		itemBaseBytes := cumulativeBytes
		itemCb := func(bytesCopied int64) {
			job.mu.Lock()
			job.CopiedBytes = itemBaseBytes + bytesCopied
			job.mu.Unlock()

			now := time.Now()
			if now.Sub(lastSendTime) >= 250*time.Millisecond {
				lastSendTime = now
				sendTransferEvent(job)
			}
		}

		var err error
		switch p.action {
		case "copy":
			if p.isSrcDir {
				err = fileutils.CopyDirectoryWithProgress(ctx, p.realSrc, p.realDst, itemCb)
			} else {
				err = fileutils.CopyFileWithProgress(ctx, p.realSrc, p.realDst, itemCb)
			}
			if err == nil {
				refreshIndexAfterCopy(p)
			}
		case "rename", "move":
			delThumbsForMove(ctx, p)
			err = fileutils.MoveFileWithProgress(ctx, p.realSrc, p.realDst, itemCb)
			if err == nil {
				// os.Rename is instant — callback never fires, so set progress directly
				job.mu.Lock()
				job.CopiedBytes = itemBaseBytes + itemSize
				job.mu.Unlock()
				refreshIndexAfterMove(p)
			}
		default:
			err = fmt.Errorf("unsupported action: %s", p.action)
		}

		if err != nil {
			if ctx.Err() != nil {
				job.mu.Lock()
				job.Status = TransferStatusCancelled
				now := time.Now()
				job.CompletedAt = &now
				job.mu.Unlock()
				sendTransferEvent(job)
				return
			}
			job.mu.Lock()
			job.Status = TransferStatusFailed
			job.Error = err.Error()
			now := time.Now()
			job.CompletedAt = &now
			job.mu.Unlock()
			sendTransferEvent(job)
			return
		}

		cumulativeBytes += itemSize
		job.mu.Lock()
		job.ItemsCompleted = i + 1
		job.CopiedBytes = cumulativeBytes
		job.mu.Unlock()
		sendTransferEvent(job)
	}

	// Phase 3: Complete
	logger.Infof("[TRANSFER] Job %s: status=completed, copiedBytes=%d", job.ID, cumulativeBytes)
	job.mu.Lock()
	job.Status = TransferStatusCompleted
	job.CopiedBytes = job.TotalBytes
	job.CurrentFile = ""
	now := time.Now()
	job.CompletedAt = &now
	job.mu.Unlock()
	sendTransferEvent(job)
}

func refreshIndexAfterCopy(p resolvedTransferParams) {
	srcIdx := indexing.GetIndex(p.srcIndex)
	if srcIdx != nil && !srcIdx.Config.ResolvedRules.IndexingDisabled {
		srcRefreshPath := p.realSrc
		if !p.isSrcDir {
			srcRefreshPath = filepath.Dir(p.realSrc)
		}
		go files.RefreshIndex(p.srcIndex, srcRefreshPath, true, false) //nolint:errcheck
	}

	dstIdx := indexing.GetIndex(p.dstIndex)
	if dstIdx != nil && !dstIdx.Config.ResolvedRules.IndexingDisabled {
		if p.isSrcDir {
			go func() {
				if err := files.RefreshIndex(p.dstIndex, p.realDst, true, true); err != nil {
					logger.Errorf("[TRANSFER] Failed to index copied directory %s: %v", p.realDst, err)
					return
				}
				parentDir := filepath.Dir(p.realDst)
				if err := files.RefreshIndex(p.dstIndex, parentDir, true, false); err != nil {
					logger.Errorf("[TRANSFER] Failed to refresh parent %s: %v", parentDir, err)
				}
			}()
		} else {
			dstParent := filepath.Dir(p.realDst)
			go files.RefreshIndex(p.dstIndex, dstParent, true, false) //nolint:errcheck
		}
	}
}

func refreshIndexAfterMove(p resolvedTransferParams) {
	srcIdx := indexing.GetIndex(p.srcIndex)
	if srcIdx != nil && !srcIdx.Config.ResolvedRules.IndexingDisabled {
		srcIndexPath := srcIdx.MakeIndexPath(p.realSrc, p.isSrcDir)
		srcParentPath := filepath.Dir(p.realSrc)
		go func() {
			if p.isSrcDir {
				srcIdx.DeleteMetadata(srcIndexPath, true, true)
			} else {
				srcIdx.DeleteMetadata(srcIndexPath, false, false)
			}
			if err := files.RefreshIndex(p.srcIndex, srcParentPath, true, false); err != nil {
				logger.Errorf("[TRANSFER] Failed to refresh source parent %s: %v", srcParentPath, err)
			}
		}()
	}

	dstIdx := indexing.GetIndex(p.dstIndex)
	if dstIdx != nil && !dstIdx.Config.ResolvedRules.IndexingDisabled {
		if p.isSrcDir {
			go func() {
				if err := files.RefreshIndex(p.dstIndex, p.realDst, true, true); err != nil {
					logger.Errorf("[TRANSFER] Failed to index moved directory %s: %v", p.realDst, err)
					return
				}
				parentDir := filepath.Dir(p.realDst)
				if err := files.RefreshIndex(p.dstIndex, parentDir, true, false); err != nil {
					logger.Errorf("[TRANSFER] Failed to refresh destination parent %s: %v", parentDir, err)
				}
			}()
		} else {
			parentDir := filepath.Dir(p.realDst)
			go files.RefreshIndex(p.dstIndex, parentDir, true, false) //nolint:errcheck
		}
	}

	if srcIdx != nil {
		dstIdx := indexing.GetIndex(p.dstIndex)
		if dstIdx != nil {
			go store.Share.UpdateShares(srcIdx.Path, srcIdx.MakeIndexPath(p.realSrc, p.isSrcDir), dstIdx.Path, dstIdx.MakeIndexPath(p.realDst, p.isSrcDir)) //nolint:errcheck
			if store.Access != nil && srcIdx.Path == dstIdx.Path {
				go store.Access.UpdateRules(srcIdx.Path, srcIdx.MakeIndexPath(p.realSrc, p.isSrcDir), dstIdx.MakeIndexPath(p.realDst, p.isSrcDir)) //nolint:errcheck
			}
		}
	}
}

func delThumbsForMove(ctx context.Context, p resolvedTransferParams) {
	idx := indexing.GetIndex(p.srcIndex)
	if idx == nil {
		return
	}
	srcPath := idx.MakeIndexPath(p.realSrc, p.isSrcDir)
	if p.userScope != "" && p.userScope != "/" {
		srcPath = strings.TrimPrefix(srcPath, p.userScope)
	}
	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		FollowSymlinks: true,
		Path:           srcPath,
		Source:         p.srcIndex,
		IsDir:          p.isSrcDir,
		ShowHidden:     p.showHidden,
		HideFileExt:    p.hideFileExt,
	}, store.Access, nil, store.Share)
	if err != nil {
		logger.Debugf("[TRANSFER] Could not get file info for thumbnail deletion: %v", err)
		return
	}
	preview.DelThumbs(ctx, *fileInfo)
}

func sendTransferEvent(job *TransferJob) {
	job.mu.Lock()
	payload := map[string]interface{}{
		"jobId":          job.ID,
		"status":         job.Status,
		"action":         job.Action,
		"totalBytes":     job.TotalBytes,
		"copiedBytes":    job.CopiedBytes,
		"currentFile":    job.CurrentFile,
		"itemsTotal":     job.ItemsTotal,
		"itemsCompleted": job.ItemsCompleted,
		"error":          job.Error,
	}
	job.mu.Unlock()
	jsonData, err := json.Marshal(payload)
	if err != nil {
		logger.Errorf("[TRANSFER] Failed to marshal event: %v", err)
		return
	}
	logger.Debugf("[TRANSFER] SSE event to user=%s: %s", job.Username, string(jsonData))
	events.SendToUsers("transferProgress", string(jsonData), []string{job.Username})
}

// transferListHandler returns all transfer jobs for the authenticated user.
// @Summary List transfers
// @Description Returns all background transfer jobs belonging to the current user.
// @Tags Transfers
// @Produce json
// @Success 200 {array} TransferJob "List of transfer jobs"
// @Router /api/transfers [get]
func transferListHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	jobs := transferMgr.GetJobsForUser(d.user.Username)
	if jobs == nil {
		jobs = []*TransferJob{}
	}
	return renderJSON(w, r, jobs)
}

// transferGetHandler returns the status of a single transfer job.
// @Summary Get transfer
// @Description Returns the details and progress of a specific transfer job.
// @Tags Transfers
// @Produce json
// @Param id path string true "Transfer job ID"
// @Success 200 {object} TransferJob "Transfer job details"
// @Failure 400 {object} map[string]string "Bad request - missing id"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Not found"
// @Router /api/transfers/{id} [get]
func transferGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	id := r.PathValue("id")
	if id == "" {
		return http.StatusBadRequest, fmt.Errorf("transfer id is required")
	}
	job := transferMgr.GetJob(id)
	if job == nil {
		return http.StatusNotFound, fmt.Errorf("transfer not found")
	}
	if job.Username != d.user.Username {
		return http.StatusForbidden, fmt.Errorf("access denied")
	}
	return renderJSON(w, r, job)
}

// transferCancelHandler cancels an active transfer job.
// @Summary Cancel transfer
// @Description Cancels a running, calculating, or pending transfer job.
// @Tags Transfers
// @Produce json
// @Param id path string true "Transfer job ID"
// @Success 200 {object} map[string]string "Cancellation confirmed"
// @Failure 400 {object} map[string]string "Bad request - job not active or missing id"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Not found"
// @Router /api/transfers/{id} [delete]
func transferCancelHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	id := r.PathValue("id")
	if id == "" {
		return http.StatusBadRequest, fmt.Errorf("transfer id is required")
	}
	job := transferMgr.GetJob(id)
	if job == nil {
		return http.StatusNotFound, fmt.Errorf("transfer not found")
	}
	if job.Username != d.user.Username {
		return http.StatusForbidden, fmt.Errorf("access denied")
	}
	if err := transferMgr.CancelJob(id); err != nil {
		return http.StatusBadRequest, err
	}
	return renderJSON(w, r, map[string]string{"status": "cancelled"})
}
