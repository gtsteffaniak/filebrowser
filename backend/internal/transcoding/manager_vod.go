package transcoding

import (
	"context"
	"errors"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
)

func (m *Manager) ensureVODSegment(ctx context.Context, sess *playbackSession, p *producer, segIndex int) error {
	if segIndex < 0 || segIndex >= sess.plan.vodList.Count() {
		return ErrInvalidSegment
	}
	path := ffmpeg.HLSVODSegmentPath(sess.jobDir, segIndex)
	if ready, err := ffmpeg.HLSVODSegmentReady(path); err != nil {
		return err
	} else if ready {
		return nil
	}
	if p == nil {
		return ErrNotReady
	}
	head := p.encodeHead
	if segIndex <= head+vodSegmentLookahead && p.job != nil {
		return m.waitVODSegment(ctx, path, time.Duration(vodSegmentWaitTimeout)*time.Second)
	}
	return m.restartVODProducer(ctx, sess, p, segIndex)
}

func (m *Manager) waitVODSegment(ctx context.Context, path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	interval := time.Duration(vodSegmentPollInterval) * time.Millisecond
	for {
		ready, err := ffmpeg.HLSVODSegmentReady(path)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrNotReady
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (m *Manager) restartVODProducer(ctx context.Context, sess *playbackSession, p *producer, segIndex int) error {
	if p == nil || p.job == nil {
		return ErrNotReady
	}
	playlistStart := vodPlaylistStartSec(sess.plan, segIndex)
	clear := segIndex < p.encodeHead || segIndex > p.encodeHead+vodSegmentLookahead

	m.mu.Lock()
	p.vodRestartIndex = segIndex
	p.startIndex = segIndex
	p.startSec = playlistStart
	p.clearSegmentsOnStart = clear
	if p.job != nil {
		p.job.Cancel()
	}
	m.mu.Unlock()

	path := ffmpeg.HLSVODSegmentPath(sess.jobDir, segIndex)
	if err := m.waitVODSegment(ctx, path, time.Duration(vodSegmentWaitTimeout)*time.Second); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ErrNotReady
		}
		return err
	}
	return nil
}

func (m *Manager) refreshVODEncodeHead(p *producer, jobDir string, plan hlsPlan) {
	if p == nil || plan.vodList.Count() == 0 {
		return
	}
	head := p.startIndex - 1
	for i := p.startIndex; i < plan.vodList.Count(); i++ {
		path := ffmpeg.HLSVODSegmentPath(jobDir, i)
		ready, err := ffmpeg.HLSVODSegmentReady(path)
		if err != nil || !ready {
			break
		}
		head = i
	}
	p.encodeHead = head
}

func (m *Manager) writeVODPlaylist(jobDir, body string) error {
	return m.store.WriteServePlaylist(jobDir, body)
}

func (m *Manager) markVODComplete(jobDir string, durationSec float64) {
	_ = m.store.WriteVODComplete(jobDir, durationSec)
}
