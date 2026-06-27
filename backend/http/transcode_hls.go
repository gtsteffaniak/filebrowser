package http

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

const hlsSegmentEncodeTimeout = 25 * time.Second

func setTranscodeNoCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

type transcodeFileRequest struct {
	source      string
	scopedPath  string
	displayName string
	realPath    string
}

func resolveTranscodeFile(r *http.Request, d *requestContext) (transcodeFileRequest, int, error) {
	if !settings.TranscodeEnabled() {
		return transcodeFileRequest{}, http.StatusNotFound, fmt.Errorf("transcode not enabled")
	}

	source := r.URL.Query().Get("source")
	fileList := r.URL.Query()["file"]
	if len(fileList) != 1 {
		return transcodeFileRequest{}, http.StatusForbidden, fmt.Errorf("transcode supports single file only")
	}
	token := r.URL.Query().Get("streamToken")
	if token == "" {
		return transcodeFileRequest{}, http.StatusForbidden, fmt.Errorf("stream token required")
	}
	cleanPath, err := utils.SanitizePath(fileList[0])
	if err != nil {
		return transcodeFileRequest{}, http.StatusBadRequest, fmt.Errorf("invalid file path: %v", err)
	}
	if err = validateStreamGrant(token, d, source, cleanPath); err != nil {
		return transcodeFileRequest{}, http.StatusForbidden, err
	}

	userscope, err := d.user.GetScopeForSourceName(source)
	if err != nil {
		return transcodeFileRequest{}, http.StatusForbidden, err
	}
	scopedPath := utils.JoinPathAsUnix(userscope, cleanPath)

	idx := indexing.GetIndex(source)
	if idx == nil {
		return transcodeFileRequest{}, http.StatusInternalServerError, fmt.Errorf("source %s is not available", source)
	}
	if !accessStore.Permitted(idx.Path, utils.IndexPathFromNormalized(scopedPath, true), d.user.Username) {
		return transcodeFileRequest{}, http.StatusForbidden, fmt.Errorf("access denied to path %s", scopedPath)
	}
	realPath, _, err := idx.GetRealPath(scopedPath)
	if err != nil {
		return transcodeFileRequest{}, http.StatusInternalServerError, err
	}

	return transcodeFileRequest{
		source:      source,
		scopedPath:  scopedPath,
		displayName: filepath.Base(scopedPath),
		realPath:    realPath,
	}, 0, nil
}

func hlsSegmentCount(durationSec float64) int {
	if durationSec <= 0 {
		return 1
	}
	return int(math.Ceil(durationSec / ffmpeg.HLSSegmentDurationSec))
}

func buildHLSSegmentParams(svc *ffmpeg.Service, ctx context.Context, realPath string, info ffmpeg.StreamInfo, profileMode string) ffmpeg.HLSSegmentParams {
	remux := canFMP4StreamCopy(info)
	videoCopy := hlsUseVideoCopy(info, profileMode)
	params := ffmpeg.HLSSegmentParams{
		Remux:     remux,
		VideoCopy: videoCopy,
		MaxHeight: transcodeMaxHeightForMode(profileMode),
	}
	if !remux && !videoCopy {
		params.Decode = transcodeDecodeProfile(info)
		params.Profile = transcodeEncodeProfileForMode(info, profileMode)
	}
	fps, err := svc.ProbeVideoFPS(ctx, realPath)
	if err != nil {
		fps = 30
	}
	params.GOP = int(fps * ffmpeg.HLSSegmentDurationSec)
	if params.GOP < 1 {
		params.GOP = 120
	}
	return params
}

func hlsSegmentDurationSec(index int, h *hlsSessionState) float64 {
	if h == nil {
		return ffmpeg.HLSSegmentDurationSec
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if index >= 0 && index < len(h.segmentDurations) {
		return h.segmentDurations[index]
	}
	return ffmpeg.HLSSegmentDurationSec
}

func hlsSegmentBounds(index int, starts, durations []float64) (startSec, durSec float64) {
	startSec = float64(index) * ffmpeg.HLSSegmentDurationSec
	durSec = ffmpeg.HLSSegmentDurationSec
	if index >= 0 && index < len(starts) {
		startSec = starts[index]
	}
	if index >= 0 && index < len(durations) {
		durSec = durations[index]
	}
	return startSec, durSec
}

func (entry *transcodeSessionEntry) ensureHLSState(svc *ffmpeg.Service, ctx context.Context, realPath string, info ffmpeg.StreamInfo, profileMode string) error {
	entry.hls.mu.Lock()
	defer entry.hls.mu.Unlock()
	profileMode = parseTranscodeProfileMode(profileMode)
	params := buildHLSSegmentParams(svc, ctx, realPath, info, profileMode)
	needsKeyframes := params.VideoCopy || params.Remux
	if entry.hls.realPath == realPath && entry.hls.profileMode == profileMode {
		if !needsKeyframes || entry.hls.keyframeTimeline {
			return nil
		}
		// Video-copy with a fixed-grid fallback: retry keyframe probe on refresh instead of
		// serving a stale playlist/encode mismatch from cached grid segments.
		hlsLogInfo(entry, "retrying keyframe probe after fixed-grid fallback")
	}

	entry.hls.realPath = realPath
	entry.hls.profileMode = profileMode
	entry.hls.params = params
	entry.hls.durationSec = info.Duration

	var starts, durations []float64
	keyframeTimeline := false
	if needsKeyframes {
		// H.264 stream copy can only cut on keyframes; a fixed grid produces overlapping
		// or empty fragments on sources with GOP > segment size (common on WEB-DL).
		keyframes, err := svc.ProbeVideoKeyframeTimes(ctx, realPath)
		if err != nil {
			hlsLogInfo(entry, "keyframe probe failed, using fixed grid: %v", err)
			starts, durations = ffmpeg.BuildHLSSegmentTimeline(info.Duration, nil)
		} else {
			sanitized := ffmpeg.SanitizeHLSKeyframes(keyframes, info.Duration)
			if sanitized == nil {
				hlsLogInfo(entry, "keyframe probe unusable, using fixed grid")
				starts, durations = ffmpeg.BuildHLSSegmentTimeline(info.Duration, nil)
			} else {
				keyframeTimeline = true
				starts, durations = ffmpeg.BuildHLSSegmentTimeline(info.Duration, sanitized)
			}
		}
	} else {
		starts, durations = ffmpeg.BuildHLSSegmentTimeline(info.Duration, nil)
	}
	entry.hls.keyframeTimeline = keyframeTimeline
	entry.hls.segmentStarts = starts
	entry.hls.segmentDurations = durations
	entry.hls.segmentCount = len(starts)
	if entry.hls.segmentCount < 1 {
		entry.hls.segmentCount = hlsSegmentCount(info.Duration)
	}

	entry.hls.init = nil
	entry.hls.inits = make(map[int][]byte)
	entry.hls.segments = make(map[int][]byte)
	return nil
}

func (h *hlsSessionState) cachedInit(index int) ([]byte, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if index == 0 && len(h.init) > 0 {
		return append([]byte(nil), h.init...), true
	}
	if h.inits == nil {
		return nil, false
	}
	data, ok := h.inits[index]
	if !ok || len(data) == 0 {
		return nil, false
	}
	return append([]byte(nil), data...), true
}

func (h *hlsSessionState) cachedSegment(index int) ([]byte, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if index >= h.segmentCount {
		return nil, false
	}
	data, ok := h.segments[index]
	if !ok || len(data) == 0 {
		return nil, false
	}
	return append([]byte(nil), data...), true
}

func (h *hlsSessionState) storeInitAndSegment(index int, init, media []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if index == 0 && len(init) > 0 {
		h.init = append([]byte(nil), init...)
	}
	if len(init) > 0 {
		if h.inits == nil {
			h.inits = make(map[int][]byte)
		}
		h.inits[index] = append([]byte(nil), init...)
	}
	if len(media) > 0 {
		if h.segments == nil {
			h.segments = make(map[int][]byte)
		}
		h.segments[index] = append([]byte(nil), media...)
	}
}

func (h *hlsSessionState) storeSegment(index int, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.segments[index] = append([]byte(nil), data...)
}

func (h *hlsSessionState) segmentInRange(index int) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return index >= 0 && index < h.segmentCount
}

func (h *hlsSessionState) snapshotEncodeParams() (realPath string, params ffmpeg.HLSSegmentParams, profileMode string, starts, durations []float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.realPath, h.params, h.profileMode, append([]float64(nil), h.segmentStarts...), append([]float64(nil), h.segmentDurations...)
}

func hlsBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	basePath := strings.TrimSuffix(settings.Config.Server.BaseURL, "/")
	return fmt.Sprintf("%s://%s%s", scheme, r.Host, basePath)
}

func hlsInitURL(base, sessionKey string, index int) string {
	if index <= 0 {
		return fmt.Sprintf("%s/api/media/transcode/hls/init.m4s?session=%s", base, url.QueryEscape(sessionKey))
	}
	return fmt.Sprintf("%s/api/media/transcode/hls/init/%d.m4s?session=%s", base, index, url.QueryEscape(sessionKey))
}

func hlsInitIndexFromRequest(r *http.Request) int {
	if idxStr := r.PathValue("index"); idxStr != "" {
		idxStr = strings.TrimSuffix(idxStr, ".m4s")
		if index, err := strconv.Atoi(idxStr); err == nil && index >= 0 {
			return index
		}
	}
	return 0
}

func hlsUsesMPEGTS(params ffmpeg.HLSSegmentParams) bool {
	return !params.Remux && !params.VideoCopy
}

func hlsSegURL(base, sessionKey string, index int, mpegts bool) string {
	ext := ".m4s"
	if mpegts {
		ext = ".ts"
	}
	return fmt.Sprintf("%s/api/media/transcode/hls/seg/%d%s?session=%s", base, index, ext, url.QueryEscape(sessionKey))
}

// transcodeHLSPlaylistHandler serves a dynamic VOD HLS playlist for on-demand transcoding.
func transcodeHLSPlaylistHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	fileReq, status, err := resolveTranscodeFile(r, d)
	if err != nil {
		return status, err
	}

	svc := ffmpeg.Get()
	if svc == nil {
		return http.StatusServiceUnavailable, fmt.Errorf("ffmpeg unavailable")
	}

	acquire := activeTranscodeSessions.acquireHLS(d.user.ID, d.user.Username, fileReq.source, fileReq.scopedPath, fileReq.displayName)
	if !acquire.OK {
		switch acquire.Reason {
		case "user_limit":
			return http.StatusConflict, fmt.Errorf("transcode user limit reached")
		default:
			return http.StatusServiceUnavailable, fmt.Errorf("transcode system limit reached")
		}
	}

	entry, ok := activeTranscodeSessions.getHLSEntry(acquire.Session.ID, d.user.ID)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("hls session missing")
	}

	if entry.hls == nil {
		entry.hls = &hlsSessionState{}
	}

	// One ffmpeg job at a time per session (probe, keyframe scan, segment encode).
	entry.hls.encodeMu.Lock()
	defer entry.hls.encodeMu.Unlock()

	info, err := svc.ProbeFile(r.Context(), fileReq.realPath)
	if err != nil || !info.IsValid {
		return http.StatusInternalServerError, fmt.Errorf("probe failed: %v", err)
	}

	profileMode := parseTranscodeProfileMode(r.URL.Query().Get("profile"))
	if err := entry.ensureHLSState(svc, r.Context(), fileReq.realPath, info, profileMode); err != nil {
		return http.StatusInternalServerError, err
	}

	segCount := entry.hls.segmentCount
	durationSec := entry.hls.durationSec
	// Each segment is encoded independently with -reset_timestamps 1; discontinuity tags
	// tell hls.js/MSE to advance the timeline by #EXTINF duration.
	useDiscontinuity := true
	useMPEGTS := hlsUsesMPEGTS(entry.hls.params)
	hlsLogInfo(entry, "playlist profile=%s segments=%d duration=%.1fs keyframes=%t discontinuity=%t mpegts=%t %s",
		profileMode, segCount, durationSec, entry.hls.keyframeTimeline, useDiscontinuity, useMPEGTS,
		hlsLogParams(entry.hls.params, profileMode))
	base := hlsBaseURL(r)
	sessionKey := acquire.Session.ID

	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	if useMPEGTS {
		b.WriteString("#EXT-X-VERSION:3\n")
	} else {
		b.WriteString("#EXT-X-VERSION:7\n")
	}
	targetDur := 4
	entry.hls.mu.Lock()
	segmentDurations := append([]float64(nil), entry.hls.segmentDurations...)
	entry.hls.mu.Unlock()
	for _, d := range segmentDurations {
		if ceil := int(d + 0.999); ceil > targetDur {
			targetDur = ceil
		}
	}
	b.WriteString("#EXT-X-TARGETDURATION:")
	fmt.Fprintf(&b, "%d\n", targetDur)
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	for i := 0; i < segCount; i++ {
		if useDiscontinuity && i > 0 {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		if !useMPEGTS {
			// Each on-demand fMP4 encode produces its own init; hls.js needs a MAP per fragment group.
			b.WriteString("#EXT-X-MAP:URI=\"")
			b.WriteString(hlsInitURL(base, sessionKey, i))
			b.WriteString("\"\n")
		}
		segDur := hlsSegmentDurationSec(i, entry.hls)
		b.WriteString("#EXTINF:")
		fmt.Fprintf(&b, "%.3f,\n", segDur)
		b.WriteString(hlsSegURL(base, sessionKey, i, useMPEGTS))
		b.WriteString("\n")
	}
	b.WriteString("#EXT-X-ENDLIST\n")

	setTranscodeNoCacheHeaders(w)
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	_, _ = w.Write([]byte(b.String()))
	return http.StatusOK, nil
}

// encodeHLSSegment generates init+media for one segment index and caches both.
func (entry *transcodeSessionEntry) encodeHLSSegment(ctx context.Context, svc *ffmpeg.Service, index int) (init, media []byte, err error) {
	realPath, params, profileMode, starts, durations := entry.hls.snapshotEncodeParams()
	startSec, durSec := hlsSegmentBounds(index, starts, durations)
	hlsLogInfo(entry, "segment %d encode start=%.3f dur=%.3f %s", index, startSec, durSec, hlsLogParams(params, profileMode))

	opts := ffmpeg.BuildHLSSegmentOptions(realPath, index, params, starts, durations)

	if hlsUsesMPEGTS(params) {
		data, segErr := svc.HLSSegment(ctx, opts)
		if segErr != nil {
			return nil, nil, segErr
		}
		if len(data) == 0 {
			return nil, nil, fmt.Errorf("empty segment output")
		}
		entry.hls.storeSegment(index, data)
		return nil, data, nil
	}

	init, media, err = svc.HLSInitAndSegment(ctx, opts)
	if err != nil && !params.Remux && !params.VideoCopy {
		hlsLogInfo(entry, "segment %d transcode failed, retrying with video copy: %v", index, err)
		fallback := opts
		fallback.VideoCopy = true
		fallback.Decode = encode.VideoDecodeProfile{}
		fallback.Profile = encode.VideoProfile{}
		init, media, err = svc.HLSInitAndSegment(ctx, fallback)
	}
	if err != nil {
		return nil, nil, err
	}
	if len(media) == 0 {
		return nil, nil, fmt.Errorf("empty segment output")
	}
	entry.hls.storeInitAndSegment(index, init, media)
	return init, media, nil
}

// transcodeHLSInitHandler serves the fMP4 init segment for an HLS session.
func transcodeHLSInitHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !settings.TranscodeEnabled() {
		return http.StatusNotFound, fmt.Errorf("transcode not enabled")
	}

	sessionKey := r.URL.Query().Get("session")
	if sessionKey == "" {
		return http.StatusBadRequest, fmt.Errorf("session required")
	}
	index := hlsInitIndexFromRequest(r)
	entry, ok := activeTranscodeSessions.getHLSEntry(sessionKey, d.user.ID)
	if !ok || entry.hls == nil {
		hlsLogError(nil, "init rejected: session not found key=%q user=%d index=%d", sessionKey, d.user.ID, index)
		return http.StatusNotFound, fmt.Errorf("hls session not found")
	}
	if !entry.hls.segmentInRange(index) {
		return http.StatusNotFound, fmt.Errorf("init segment out of range")
	}
	if hlsUsesMPEGTS(entry.hls.params) {
		return http.StatusNotFound, fmt.Errorf("init not used for mpegts sessions")
	}

	svc := ffmpeg.Get()
	if svc == nil {
		return http.StatusServiceUnavailable, fmt.Errorf("ffmpeg unavailable")
	}

	if init, ok := entry.hls.cachedInit(index); ok {
		hlsLogInfo(entry, "init %d cache hit bytes=%d", index, len(init))
		setTranscodeNoCacheHeaders(w)
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(init)
		return http.StatusOK, nil
	}

	entry.hls.encodeMu.Lock()
	defer entry.hls.encodeMu.Unlock()

	if init, ok := entry.hls.cachedInit(index); ok {
		hlsLogInfo(entry, "init %d cache hit after wait bytes=%d", index, len(init))
		setTranscodeNoCacheHeaders(w)
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(init)
		return http.StatusOK, nil
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), hlsSegmentEncodeTimeout)
	defer cancel()

	init, media, err := entry.encodeHLSSegment(ctx, svc, index)
	if err != nil {
		hlsLogError(entry, "init %d encode failed after %s: %v", index, time.Since(start), err)
		return http.StatusInternalServerError, fmt.Errorf("init encode failed")
	}
	hlsLogInfo(entry, "init %d encode ok after %s initBytes=%d segBytes=%d cachedSeg=%t",
		index, time.Since(start), len(init), len(media), len(media) > 0)

	setTranscodeNoCacheHeaders(w)
	w.Header().Set("Content-Type", "video/mp4")
	_, _ = w.Write(init)
	return http.StatusOK, nil
}

// transcodeHLSSegmentHandler serves one on-demand fMP4 media segment.
func transcodeHLSSegmentHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !settings.TranscodeEnabled() {
		return http.StatusNotFound, fmt.Errorf("transcode not enabled")
	}

	sessionKey := r.URL.Query().Get("session")
	if sessionKey == "" {
		return http.StatusBadRequest, fmt.Errorf("session required")
	}
	indexStr := r.PathValue("segment")
	if indexStr == "" {
		indexStr = r.PathValue("index")
	}
	if indexStr == "" {
		indexStr = filepath.Base(r.URL.Path)
	}
	indexStr = strings.TrimSuffix(indexStr, ".m4s")
	indexStr = strings.TrimSuffix(indexStr, ".ts")
	index, err := strconv.Atoi(indexStr)
	if err != nil || index < 0 {
		return http.StatusBadRequest, fmt.Errorf("invalid segment index")
	}

	entry, ok := activeTranscodeSessions.getHLSEntry(sessionKey, d.user.ID)
	if !ok || entry.hls == nil {
		hlsLogError(nil, "segment rejected: session not found key=%q user=%d index=%d", sessionKey, d.user.ID, index)
		return http.StatusNotFound, fmt.Errorf("hls session not found")
	}

	if !entry.hls.segmentInRange(index) {
		hlsLogError(entry, "segment out of range index=%d", index)
		return http.StatusNotFound, fmt.Errorf("segment out of range")
	}

	if data, ok := entry.hls.cachedSegment(index); ok {
		hlsLogInfo(entry, "segment %d cache hit bytes=%d", index, len(data))
		setTranscodeNoCacheHeaders(w)
		w.Header().Set("Content-Type", hlsSegmentContentType(entry.hls.params))
		_, _ = w.Write(data)
		return http.StatusOK, nil
	}

	entry.hls.encodeMu.Lock()
	defer entry.hls.encodeMu.Unlock()

	if data, ok := entry.hls.cachedSegment(index); ok {
		hlsLogInfo(entry, "segment %d cache hit after wait bytes=%d", index, len(data))
		setTranscodeNoCacheHeaders(w)
		w.Header().Set("Content-Type", hlsSegmentContentType(entry.hls.params))
		_, _ = w.Write(data)
		return http.StatusOK, nil
	}

	svc := ffmpeg.Get()
	if svc == nil {
		hlsLogError(entry, "segment %d encode failed: ffmpeg unavailable", index)
		return http.StatusServiceUnavailable, fmt.Errorf("ffmpeg unavailable")
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), hlsSegmentEncodeTimeout)
	defer cancel()

	_, media, err := entry.encodeHLSSegment(ctx, svc, index)
	if err != nil {
		hlsLogError(entry, "segment %d encode failed after %s: %v", index, time.Since(start), err)
		return http.StatusInternalServerError, fmt.Errorf("segment encode failed")
	}
	hlsLogInfo(entry, "segment %d encode ok after %s bytes=%d", index, time.Since(start), len(media))

	setTranscodeNoCacheHeaders(w)
	w.Header().Set("Content-Type", hlsSegmentContentType(entry.hls.params))
	_, _ = w.Write(media)
	return http.StatusOK, nil
}

func hlsSegmentContentType(params ffmpeg.HLSSegmentParams) string {
	if hlsUsesMPEGTS(params) {
		return "video/MP2T"
	}
	return "video/mp4"
}
