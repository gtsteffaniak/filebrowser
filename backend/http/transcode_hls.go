package http

import (
	"bytes"
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
	"github.com/gtsteffaniak/go-ffmpeg/mp4"
)

func hlsSegmentEncodeTimeout(entry *transcodeSessionEntry) time.Duration {
	if entry != nil && entry.hls != nil {
		return entry.hls.delivery.Normalized().SegmentEncodeTimeout
	}
	return ffmpeg.ActiveHLSConfig().SegmentEncodeTimeout
}

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
	return int(math.Ceil(durationSec / ffmpeg.SegmentDurationSec()))
}

func buildHLSSegmentParamsFast(info ffmpeg.StreamInfo, profileMode string) ffmpeg.HLSSegmentParams {
	mode := ffmpeg.ParseHLSTranscodeProfile(profileMode)
	maxH := transcodeMaxHeightForMode(profileMode)
	in := ffmpeg.BuildHLSSegmentBuildInput(info, mode, maxH)
	return ffmpeg.BuildHLSSegmentParamsFast(in)
}

func buildHLSSegmentParamsWithGOP(svc *ffmpeg.Service, ctx context.Context, realPath string, info ffmpeg.StreamInfo, profileMode string, probeFPS bool) (ffmpeg.HLSSegmentParams, error) {
	mode := ffmpeg.ParseHLSTranscodeProfile(profileMode)
	maxH := transcodeMaxHeightForMode(profileMode)
	in := ffmpeg.BuildHLSSegmentBuildInput(info, mode, maxH)
	return svc.BuildHLSSegmentParams(ctx, realPath, in, probeFPS)
}

func hlsSegmentDurationSec(index int, h *hlsSessionState) float64 {
	if h == nil {
		return ffmpeg.SegmentDurationSec()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if index >= 0 && index < len(h.segmentDurations) {
		return h.segmentDurations[index]
	}
	return ffmpeg.SegmentDurationSec()
}

func hlsSegmentBounds(index int, starts, durations []float64) (startSec, durSec float64) {
	segDur := ffmpeg.SegmentDurationSec()
	startSec = float64(index) * segDur
	durSec = segDur
	if index >= 0 && index < len(starts) {
		startSec = starts[index]
	}
	if index >= 0 && index < len(durations) {
		durSec = durations[index]
	}
	return startSec, durSec
}

func (entry *transcodeSessionEntry) ensureHLSState(svc *ffmpeg.Service, ctx context.Context, realPath string, info ffmpeg.StreamInfo, profileMode string) error {
	stateStart := time.Now()
	entry.hls.mu.Lock()
	profileMode = parseTranscodeProfileMode(profileMode)
	params := buildHLSSegmentParamsFast(info, profileMode)
	needsKeyframes := params.VideoCopy || params.Remux
	if entry.hls.realPath == realPath && entry.hls.profileMode == profileMode && entry.hls.segmentCount > 0 {
		entry.hls.mu.Unlock()
		hlsLogInfo(entry, "hls state cache hit total=%s", hlsFormatMs(time.Since(stateStart)))
		return nil
	}
	entry.hls.mu.Unlock()

	var starts, durations []float64
	keyframeTimeline := false
	var keyframeSeekTimes []float64
	if needsKeyframes {
		cfg := ffmpeg.ActiveHLSConfig()
		probeCtx, cancel := context.WithTimeout(ctx, cfg.KeyframeProbeTimeout)
		probeStart := time.Now()
		keyframes, probeErr := svc.ProbeVideoKeyframeTimes(probeCtx, realPath)
		cancel()
		probeDur := time.Since(probeStart)
		keyframeSeekTimes = ffmpeg.SanitizeHLSKeyframes(keyframes, info.Duration)
		hlsLogInfo(entry, "hls state keyframe seek kf=%d probe=%s err=%v",
			len(keyframeSeekTimes), hlsFormatMs(probeDur), probeErr)
	}
	// Playlist uses a fixed grid so EXTINF matches what hls.js expects; stream copy seeks
	// to the nearest keyframe at or before each grid point via keyframeSeekTimes.
	starts, durations = ffmpeg.BuildHLSSegmentTimeline(info.Duration, nil)

	entry.hls.mu.Lock()
	defer entry.hls.mu.Unlock()
	if entry.hls.realPath == realPath && entry.hls.profileMode == profileMode && entry.hls.segmentCount > 0 {
		return nil
	}

	entry.hls.realPath = realPath
	entry.hls.profileMode = profileMode
	entry.hls.delivery = ffmpeg.ActiveHLSConfig()
	entry.hls.params = params
	entry.hls.durationSec = info.Duration
	entry.hls.gopResolved = false
	entry.hls.keyframeTimeline = keyframeTimeline
	entry.hls.segmentStarts = starts
	entry.hls.segmentDurations = durations
	entry.hls.keyframeSeekTimes = keyframeSeekTimes
	entry.hls.segmentMediaEnds = nil
	entry.hls.segmentCount = len(starts)
	if entry.hls.segmentCount < 1 {
		entry.hls.segmentCount = hlsSegmentCount(info.Duration)
	}
	entry.hls.sharedInit = true
	entry.hls.lastActivity = time.Now()

	entry.hls.init = nil
	entry.hls.inits = make(map[int][]byte)
	entry.hls.segments = make(map[int][]byte)
	hlsLogInfo(entry,
		"hls state ready total=%s segments=%d keyframes=%t %s %s",
		hlsFormatMs(time.Since(stateStart)),
		entry.hls.segmentCount,
		keyframeTimeline,
		hlsLogParams(params, profileMode),
		svc.DescribeHLSEncodePlan(params),
	)
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

func (h *hlsSessionState) snapshotEncodeParams() (realPath string, params ffmpeg.HLSSegmentParams, profileMode string, starts, durations, keyframeSeekTimes []float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.realPath, h.params, h.profileMode,
		append([]float64(nil), h.segmentStarts...),
		append([]float64(nil), h.segmentDurations...),
		append([]float64(nil), h.keyframeSeekTimes...)
}

func (h *hlsSessionState) mediaTimelineSec(index int) float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.mediaTimelineSecLocked(index)
}

func (h *hlsSessionState) mediaTimelineSecLocked(index int) float64 {
	if index <= 0 {
		return 0
	}
	if index-1 < len(h.segmentMediaEnds) && h.segmentMediaEnds[index-1] > 0 {
		return h.segmentMediaEnds[index-1]
	}
	if index >= 0 && index < len(h.segmentStarts) {
		return h.segmentStarts[index]
	}
	return float64(index) * ffmpeg.SegmentDurationSec()
}

func (h *hlsSessionState) applySegmentMediaMetrics(index int, actualDurSec float64) {
	if actualDurSec <= 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	actualDurSec = roundSegmentDurationSec(actualDurSec)
	if index >= 0 && index < len(h.segmentDurations) {
		h.segmentDurations[index] = actualDurSec
	}
	start := h.mediaTimelineSecLocked(index)
	end := start + actualDurSec
	if len(h.segmentMediaEnds) < index+1 {
		next := make([]float64, index+1)
		copy(next, h.segmentMediaEnds)
		h.segmentMediaEnds = next
	}
	h.segmentMediaEnds[index] = end
}

func roundSegmentDurationSec(sec float64) float64 {
	if sec <= 0 {
		return 0
	}
	return float64(int(sec*1000+0.5)) / 1000
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

func hlsUsesMPEGTS(_ ffmpeg.HLSSegmentParams) bool {
	// fMP4 + shared init + output_ts_offset gives a continuous MSE timeline in hls.js.
	// MPEG-TS independent segments stall the playhead around the first segment boundary.
	return false
}

func (entry *transcodeSessionEntry) warmHLSSegments(svc *ffmpeg.Service, count int) {
	if svc == nil || entry == nil || entry.hls == nil || count <= 0 {
		return
	}
	cfg := entry.hls.delivery.Normalized()
	if cfg.Mode != ffmpeg.HLSModeOnDemand {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SegmentEncodeTimeout*time.Duration(count))
	defer cancel()
	for idx := 0; idx < count; idx++ {
		if ctx.Err() != nil {
			return
		}
		if !entry.hls.segmentInRange(idx) {
			return
		}
		entry.hls.encodeMu.Lock()
		if _, ok := entry.hls.cachedSegment(idx); ok {
			entry.hls.encodeMu.Unlock()
			continue
		}
		_, _, err := entry.encodeHLSSegment(ctx, svc, idx)
		entry.hls.encodeMu.Unlock()
		if err != nil {
			hlsLogInfo(entry, "warm segment %d failed: %v", idx, err)
			return
		}
	}
}

func (entry *transcodeSessionEntry) ensureSegmentGOP(ctx context.Context, svc *ffmpeg.Service, realPath string) {
	entry.hls.mu.Lock()
	if entry.hls.gopResolved {
		entry.hls.mu.Unlock()
		return
	}
	entry.hls.mu.Unlock()

	info, err := svc.ProbeFile(ctx, realPath)
	if err != nil || !info.IsValid {
		return
	}
	params, err := buildHLSSegmentParamsWithGOP(svc, ctx, realPath, info, entry.hls.profileMode, true)
	if err != nil {
		return
	}

	entry.hls.mu.Lock()
	entry.hls.params.GOP = params.GOP
	entry.hls.gopResolved = true
	entry.hls.mu.Unlock()
}

func hlsSegURL(base, sessionKey string, index int, mpegts bool, runtimeSec float64) string {
	ext := ".m4s"
	if mpegts {
		ext = ".ts"
	}
	url := fmt.Sprintf("%s/api/media/transcode/hls/seg/%d%s?session=%s&runtimeSec=%.3f",
		base, index, ext, url.QueryEscape(sessionKey), runtimeSec)
	return url
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
		entry.hls = &hlsSessionState{delivery: ffmpeg.ActiveHLSConfig()}
	}

	reqStart := time.Now()
	probeStart := time.Now()
	info, err := svc.ProbeFile(r.Context(), fileReq.realPath)
	probeDur := time.Since(probeStart)
	if err != nil || !info.IsValid {
		return http.StatusInternalServerError, fmt.Errorf("probe failed: %v", err)
	}

	profileMode := ffmpeg.ParseHLSTranscodeProfile(r.URL.Query().Get("profile"))
	stateStart := time.Now()
	if err := entry.ensureHLSState(svc, r.Context(), fileReq.realPath, info, string(profileMode)); err != nil {
		return http.StatusInternalServerError, err
	}
	stateDur := time.Since(stateStart)

	hlsCfg := entry.hls.delivery.Normalized()
	if hlsCfg.Mode != ffmpeg.HLSModeOnDemand {
		return http.StatusNotImplemented, fmt.Errorf("hls mode %q is not implemented", hlsCfg.Mode)
	}

	entry.hls.encodeMu.Lock()
	if _, ok := entry.hls.cachedSegment(0); !ok {
		warmCtx, warmCancel := context.WithTimeout(r.Context(), hlsCfg.SegmentEncodeTimeout)
		_, _, warmErr := entry.encodeHLSSegment(warmCtx, svc, 0)
		warmCancel()
		if warmErr != nil {
			entry.hls.encodeMu.Unlock()
			return http.StatusInternalServerError, fmt.Errorf("warm segment 0: %w", warmErr)
		}
	}
	entry.hls.encodeMu.Unlock()
	go entry.warmHLSSegments(svc, hlsCfg.WarmPlaylistSegments)

	segCount := entry.hls.segmentCount
	durationSec := entry.hls.durationSec
	useMPEGTS := hlsUsesMPEGTS(entry.hls.params)
	entry.hls.mu.Lock()
	sharedInit := entry.hls.sharedInit
	segmentDurations := append([]float64(nil), entry.hls.segmentDurations...)
	segmentStarts := append([]float64(nil), entry.hls.segmentStarts...)
	entry.hls.mu.Unlock()
	// Segments use output_ts_offset for a continuous timeline; discontinuity tags
	// cause hls.js to jump backward at each segment boundary.
	useDiscontinuity := false
	hlsLogInfo(entry, "playlist profile=%s segments=%d duration=%.1fs keyframes=%t sharedInit=%t mpegts=%t probe=%s stateSetup=%s total=%s %s",
		profileMode, segCount, durationSec, entry.hls.keyframeTimeline, sharedInit, useMPEGTS,
		hlsFormatMs(probeDur), hlsFormatMs(stateDur), hlsFormatMs(time.Since(reqStart)),
		hlsLogParams(entry.hls.params, string(profileMode)))
	base := hlsBaseURL(r)
	sessionKey := acquire.Session.ID
	entry.hls.touchActivity(-1)

	var b strings.Builder
	b.Grow(segCount * 128)
	b.WriteString("#EXTM3U\n")
	b.WriteString(hlsCfg.PlaylistConfigComment())
	b.WriteString("\n")
	if useMPEGTS {
		b.WriteString("#EXT-X-VERSION:3\n")
	} else {
		b.WriteString("#EXT-X-VERSION:7\n")
	}
	targetDur := int(math.Ceil(ffmpeg.SegmentDurationSec()))
	for _, d := range segmentDurations {
		if ceil := int(d + 0.999); ceil > targetDur {
			targetDur = ceil
		}
	}
	b.WriteString("#EXT-X-TARGETDURATION:")
	fmt.Fprintf(&b, "%d\n", targetDur)
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	if sharedInit && !useMPEGTS {
		b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
		b.WriteString("#EXT-X-MAP:URI=\"")
		b.WriteString(hlsInitURL(base, sessionKey, 0))
		b.WriteString("\"\n")
	}
	for i := 0; i < segCount; i++ {
		if useDiscontinuity && i > 0 {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		if !useMPEGTS && !sharedInit {
			b.WriteString("#EXT-X-MAP:URI=\"")
			b.WriteString(hlsInitURL(base, sessionKey, i))
			b.WriteString("\"\n")
		}
		segDur := ffmpeg.SegmentDurationSec()
		if i >= 0 && i < len(segmentDurations) {
			segDur = segmentDurations[i]
		}
		runtimeSec := float64(i) * ffmpeg.SegmentDurationSec()
		if i >= 0 && i < len(segmentStarts) {
			runtimeSec = segmentStarts[i]
		}
		b.WriteString("#EXTINF:")
		fmt.Fprintf(&b, "%.3f,\n", segDur)
		b.WriteString(hlsSegURL(base, sessionKey, i, useMPEGTS, runtimeSec))
		b.WriteString("\n")
	}
	b.WriteString("#EXT-X-ENDLIST\n")

	setTranscodeNoCacheHeaders(w)
	ffmpeg.WriteHLSConfigHeaders(w, hlsCfg)
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("X-Transcode-Session", sessionKey)
	_, _ = w.Write([]byte(b.String()))
	return http.StatusOK, nil
}

// encodeHLSSegment generates init+media for one segment index and caches both.
func (entry *transcodeSessionEntry) encodeHLSSegment(ctx context.Context, svc *ffmpeg.Service, index int) (init, media []byte, err error) {
	realPath, _, _, _, _, _ := entry.hls.snapshotEncodeParams()
	entry.ensureSegmentGOP(ctx, svc, realPath)
	realPath, params, profileMode, starts, durations, keyframeSeekTimes := entry.hls.snapshotEncodeParams()
	startSec, durSec := hlsSegmentBounds(index, starts, durations)
	mediaTimelineSec := entry.hls.mediaTimelineSec(index)
	hlsLogInfo(entry, "segment %d encode start=%.3f mediaT=%.3f dur=%.3f %s plan=%s",
		index, startSec, mediaTimelineSec, durSec, hlsLogParams(params, profileMode), svc.DescribeHLSEncodePlan(params))

	opts := ffmpeg.BuildHLSSegmentOptions(realPath, index, params, starts, durations, entry.hls.keyframeTimeline, keyframeSeekTimes)
	opts.MediaTimelineSec = mediaTimelineSec

	entry.hls.mu.Lock()
	sharedInit := entry.hls.sharedInit
	entry.hls.mu.Unlock()

	if hlsUsesMPEGTS(params) {
		data, segErr := svc.HLSSegment(ctx, opts)
		if segErr != nil {
			return nil, nil, segErr
		}
		if len(data) == 0 {
			return nil, nil, fmt.Errorf("empty segment output")
		}
		entry.recordSegmentMediaMetrics(index, data, params)
		entry.hls.storeSegment(index, data)
		return nil, data, nil
	}

	if sharedInit && index > 0 {
		var buf bytes.Buffer
		if err = svc.HLSSegmentMedia(ctx, &buf, opts); err != nil {
			return nil, nil, err
		}
		media = buf.Bytes()
		if len(media) == 0 {
			return nil, nil, fmt.Errorf("empty segment output")
		}
		entry.recordSegmentMediaMetrics(index, media, params)
		entry.hls.storeSegment(index, media)
		return nil, media, nil
	}

	init, media, err = svc.HLSInitAndSegment(ctx, opts)
	if err != nil && params.Remux {
		hlsLogInfo(entry, "segment %d remux failed, retrying with video copy: %v", index, err)
		fallbackParams := params
		fallbackParams.Remux = false
		fallbackParams.VideoCopy = true
		fallbackParams.Decode = encode.VideoDecodeProfile{}
		fallbackParams.Profile = encode.VideoProfile{}
		fallbackOpts := ffmpeg.BuildHLSSegmentOptions(realPath, index, fallbackParams, starts, durations, entry.hls.keyframeTimeline, keyframeSeekTimes)
		fallbackOpts.MediaTimelineSec = mediaTimelineSec
		init, media, err = svc.HLSInitAndSegment(ctx, fallbackOpts)
		if err == nil {
			entry.hls.mu.Lock()
			entry.hls.params = fallbackParams
			entry.hls.mu.Unlock()
		}
	}
	if err != nil {
		return nil, nil, err
	}
	if len(media) == 0 {
		return nil, nil, fmt.Errorf("empty segment output")
	}
	entry.recordSegmentMediaMetrics(index, media, params)
	entry.hls.storeInitAndSegment(index, init, media)
	return init, media, nil
}

func (entry *transcodeSessionEntry) recordSegmentMediaMetrics(index int, media []byte, params ffmpeg.HLSSegmentParams) {
	if hlsUsesMPEGTS(params) || len(media) == 0 {
		return
	}
	actualDur := mp4.FragmentDurationSec(media)
	if actualDur <= 0 {
		actualDur = hlsSegmentDurationSec(index, entry.hls)
	}
	entry.hls.applySegmentMediaMetrics(index, actualDur)
}

// transcodeHLSInitHandler serves the fMP4 init segment for an HLS session.
func transcodeHLSInitHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	reqStart := time.Now()
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
	entry.hls.touchActivity(-1)
	entry.hls.mu.Lock()
	sharedInit := entry.hls.sharedInit
	entry.hls.mu.Unlock()
	if sharedInit && index > 0 {
		index = 0
	}
	if hlsUsesMPEGTS(entry.hls.params) {
		return http.StatusNotFound, fmt.Errorf("init not used for mpegts sessions")
	}

	svc := ffmpeg.Get()
	if svc == nil {
		return http.StatusServiceUnavailable, fmt.Errorf("ffmpeg unavailable")
	}

	if init, ok := entry.hls.cachedInit(index); ok {
		hlsLogInfo(entry, "init %d cache hit bytes=%d total=%s", index, len(init), hlsFormatMs(time.Since(reqStart)))
		setTranscodeNoCacheHeaders(w)
		ffmpeg.WriteHLSConfigHeaders(w, entry.hls.delivery)
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(init)
		return http.StatusOK, nil
	}

	queueStart := time.Now()
	entry.hls.encodeMu.Lock()
	queueWait := time.Since(queueStart)
	defer entry.hls.encodeMu.Unlock()

	if init, ok := entry.hls.cachedInit(index); ok {
		hlsLogInfo(entry, "init %d cache hit after wait queue=%s bytes=%d total=%s",
			index, hlsFormatMs(queueWait), len(init), hlsFormatMs(time.Since(reqStart)))
		setTranscodeNoCacheHeaders(w)
		ffmpeg.WriteHLSConfigHeaders(w, entry.hls.delivery)
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(init)
		return http.StatusOK, nil
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), hlsSegmentEncodeTimeout(entry))
	defer cancel()

	init, media, err := entry.encodeHLSSegment(ctx, svc, index)
	if err != nil {
		hlsLogError(entry, "init %d encode failed queue=%s encode=%s total=%s: %v",
			index, hlsFormatMs(queueWait), hlsFormatMs(time.Since(start)), hlsFormatMs(time.Since(reqStart)), err)
		if status := hlsEncodeHTTPStatus(err); status != 0 {
			return status, fmt.Errorf("init encode failed")
		}
		return 0, nil
	}
	encodeDur := time.Since(start)
	hlsLogInfo(entry, "init %d encode ok queue=%s encode=%s total=%s initBytes=%d segBytes=%d cachedSeg=%t",
		index, hlsFormatMs(queueWait), hlsFormatMs(encodeDur), hlsFormatMs(time.Since(reqStart)),
		len(init), len(media), len(media) > 0)

	setTranscodeNoCacheHeaders(w)
	ffmpeg.WriteHLSConfigHeaders(w, entry.hls.delivery)
	w.Header().Set("Content-Type", "video/mp4")
	_, _ = w.Write(init)
	return http.StatusOK, nil
}

// transcodeHLSSegmentHandler serves one on-demand fMP4 media segment.
func transcodeHLSSegmentHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	reqStart := time.Now()
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
	entry.hls.touchActivity(-1)
	if runtimeSec, ok := parseRuntimeSecQuery(r); ok {
		entry.hls.mu.Lock()
		starts := append([]float64(nil), entry.hls.segmentStarts...)
		entry.hls.mu.Unlock()
		expected := segmentIndexForPlayhead(runtimeSec, starts)
		if expected != index {
			hlsLogInfo(entry, "segment index=%d runtimeSec=%.3f maps to index=%d", index, runtimeSec, expected)
		}
	}

	if data, ok := entry.hls.cachedSegment(index); ok {
		hlsLogInfo(entry, "segment %d cache hit bytes=%d total=%s", index, len(data), hlsFormatMs(time.Since(reqStart)))
		setTranscodeNoCacheHeaders(w)
		w.Header().Set("Content-Type", hlsSegmentContentType(entry.hls.params))
		_, _ = w.Write(data)
		return http.StatusOK, nil
	}

	queueStart := time.Now()
	entry.hls.encodeMu.Lock()
	queueWait := time.Since(queueStart)
	defer entry.hls.encodeMu.Unlock()

	if data, ok := entry.hls.cachedSegment(index); ok {
		hlsLogInfo(entry, "segment %d cache hit after wait queue=%s bytes=%d total=%s",
			index, hlsFormatMs(queueWait), len(data), hlsFormatMs(time.Since(reqStart)))
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
	ctx, cancel := context.WithTimeout(r.Context(), hlsSegmentEncodeTimeout(entry))
	defer cancel()

	_, media, err := entry.encodeHLSSegment(ctx, svc, index)
	if err != nil {
		hlsLogError(entry, "segment %d encode failed queue=%s encode=%s total=%s: %v",
			index, hlsFormatMs(queueWait), hlsFormatMs(time.Since(start)), hlsFormatMs(time.Since(reqStart)), err)
		if status := hlsEncodeHTTPStatus(err); status != 0 {
			return status, fmt.Errorf("segment encode failed")
		}
		return 0, nil
	}
	entry.hls.pruneSegmentCache()
	encodeDur := time.Since(start)
	realtimeRatio := 0.0
	if segDur := hlsSegmentDurationSec(index, entry.hls); segDur > 0 {
		realtimeRatio = segDur / encodeDur.Seconds()
	}
	hlsLogInfo(entry, "segment %d encode ok queue=%s encode=%s total=%s bytes=%d segDur=%.3fs realtime=%.2fx",
		index, hlsFormatMs(queueWait), hlsFormatMs(encodeDur), hlsFormatMs(time.Since(reqStart)),
		len(media), hlsSegmentDurationSec(index, entry.hls), realtimeRatio)

	setTranscodeNoCacheHeaders(w)
	ffmpeg.WriteHLSConfigHeaders(w, entry.hls.delivery)
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
