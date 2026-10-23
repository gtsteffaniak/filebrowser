package transcoding

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/go-logger/logger"
)

// leasedSegmentFile wraps an opened VOD segment so Close() also releases the
// HLSVODSession serve-path lease once the handler finishes copying.
type leasedSegmentFile struct {
	*os.File
	release func()
	once    sync.Once
}

func (f *leasedSegmentFile) Close() error {
	var err error
	f.once.Do(func() {
		err = f.File.Close()
		if f.release != nil {
			f.release()
		}
	})
	return err
}

func vodJobRunning(p *producer) bool {
	return p != nil && p.job != nil && p.job.Running()
}

// vodJobView snapshots the producer for DecideHLSVODSegmentAction.
// EncodeHead mirrors go-ffmpeg's session: the first not-yet-ready index at or
// after the job start.
func vodJobView(p *producer) ffmpeg.HLSVODJobView {
	if p == nil {
		return ffmpeg.HLSVODJobView{}
	}
	return ffmpeg.HLSVODJobView{
		Running:    vodJobRunning(p) || p.jobStarting || p.vodRestartIndex >= 0,
		StartIndex: p.startIndex,
		EncodeHead: p.encodeHead,
	}
}

// openVODSegment waits for the segment the way go-ffmpeg's HLSVODSession does:
// serve a publishable fragment, wait while the request sits within the forward
// gap of the encode head, and reposition the producer when it is behind the
// job start or too far ahead. The returned file holds a serve-path lease on
// the segment so an in-flight read survives a producer rewrite; callers must
// Close it.
func (m *Manager) openVODSegment(ctx context.Context, sess *playbackSession, jobDir string, segIndex int) (io.ReadCloser, error) {
	if segIndex < 0 || segIndex >= sess.plan.vodList.Count() {
		return nil, ErrInvalidSegment
	}
	session := sess.vodSession
	segPath := ffmpeg.HLSVODSegmentPath(jobDir, segIndex)
	explicitCopy := planUsesExplicitCopy(sess.plan)
	cfg := vodDecisionConfig(sess.plan)
	waitStart := time.Now()
	waited := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		m.mu.Lock()
		p := m.producers[sess.info.ID]
		// Snapshot under the lock: the producer goroutine refreshes encodeHead
		// and swaps job while a request is in flight.
		view := vodJobView(p)
		m.mu.Unlock()
		var f *os.File
		var ready bool
		var err error
		if explicitCopy {
			f, ready, err = ffmpeg.HLSVODOpenReadyExplicitCopySegment(jobDir, segIndex, view.Running)
		} else {
			f, ready, err = ffmpeg.HLSVODOpenReadySegment(segPath)
		}
		if err != nil {
			return nil, err
		}
		if ready && f != nil {
			if waited {
				logger.Debugf("transcode segment served after wait session=%s seg=%d elapsed=%s",
					sess.info.ID, segIndex, time.Since(waitStart).Round(time.Millisecond))
			}
			if session == nil {
				return f, nil
			}
			session.AcquireServePath(segPath)
			return &leasedSegmentFile{
				File:    f,
				release: func() { session.ReleaseServePath(segPath) },
			}, nil
		}
		if f != nil {
			f.Close()
		}

		action := ffmpeg.DecideHLSVODSegmentAction(cfg, view, segIndex, false)
		if action == ffmpeg.HLSVODActionRestart {
			// Producers leave m.producers once their job finishes or fails, so
			// nil means no producer can ever write this segment (finished
			// generation or completed-generation viewer).
			if p == nil {
				return nil, ErrNotReady
			}
			logger.Debugf("transcode segment wait restart session=%s seg=%d head=%d jobStart=%d running=%v",
				sess.info.ID, segIndex, view.EncodeHead, view.StartIndex, view.Running)
			m.signalVODRestart(sess, p, segIndex)
		} else if p == nil {
			// Wait action only makes sense while a producer can still write.
			return nil, ErrNotReady
		}
		if p != nil {
			// Demand for an unproduced segment wakes a producer suspended by
			// all-paused playback; it relaunches at the encode head or the
			// reposition signalled above.
			m.mu.Lock()
			m.resumeProducerLocked(p)
			m.mu.Unlock()
		}
		waited = true
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(vodSegmentPollInterval * time.Millisecond):
		}
	}
}

// signalVODRestart repositions the producer to segIndex and cancels the running
// job; runProducerJob picks the pending restart up at its next job launch.
// Stale segments at or after segIndex are cleared by the new job so a rewrite
// cannot serve stale output. The restart is deferred while another attached
// session's playhead sits inside the job's servable window and the job still
// has reposition cooldown left — repositioning sooner would pull the encode
// out from under them before it produced anything there.
func (m *Manager) signalVODRestart(sess *playbackSession, p *producer, segIndex int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ffmpeg.DecideHLSVODSegmentAction(vodDecisionConfig(sess.plan), vodJobView(p), segIndex, false) != ffmpeg.HLSVODActionRestart {
		return
	}
	if m.vodRestartBlockedLocked(sess, p) {
		return
	}
	p.vodRestartIndex = segIndex
	p.startIndex = segIndex
	p.startSec = vodPlaylistStartSec(sess.plan, segIndex)
	p.clearSegmentsOnStart = true
	p.encodeHead = segIndex
	if p.job != nil {
		p.job.Cancel()
	}
	m.resumeProducerLocked(p)
}

func (m *Manager) refreshVODEncodeHead(p *producer, jobDir string, plan hlsPlan) {
	if p == nil {
		return
	}
	// Encode head only advances over ready segments and is reset to startIndex
	// on reposition, so the scan resumes where it left off instead of
	// re-parsing every produced segment each tick.
	head := p.startIndex
	if p.encodeHead > head {
		head = p.encodeHead
	}
	jobRunning := vodJobRunning(p)
	explicitCopy := planUsesExplicitCopy(plan)
	for head < plan.vodList.Count() {
		var ready bool
		var err error
		if explicitCopy {
			ready, err = ffmpeg.HLSVODExplicitCopySegmentReady(jobDir, head, jobRunning)
		} else {
			ready, err = ffmpeg.HLSVODSegmentReady(ffmpeg.HLSVODSegmentPath(jobDir, head))
		}
		if err != nil || !ready {
			break
		}
		head++
	}
	p.encodeHead = head
}

// firstVODHole returns the first missing segment that sits after a produced
// one, or -1 when the produced range is contiguous through the end. A
// generation that began mid-file has no hole: its range simply starts later.
func (m *Manager) firstVODHole(jobDir string, plan hlsPlan) int {
	explicitCopy := planUsesExplicitCopy(plan)
	seen := false
	for i := 0; i < plan.vodList.Count(); i++ {
		var ready bool
		var err error
		if explicitCopy {
			ready, err = ffmpeg.HLSVODExplicitCopySegmentReady(jobDir, i, false)
		} else {
			ready, err = ffmpeg.HLSVODSegmentReady(ffmpeg.HLSVODSegmentPath(jobDir, i))
		}
		if err == nil && ready {
			seen = true
		} else if seen {
			return i
		}
	}
	return -1
}

func (m *Manager) writeVODPlaylist(jobDir, body string) error {
	return m.store.WriteServePlaylist(jobDir, body)
}

func (m *Manager) markVODComplete(jobDir string, durationSec float64) {
	_ = m.store.WriteVODComplete(jobDir, durationSec)
}
