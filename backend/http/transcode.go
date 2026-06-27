package http

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-logger/logger"
)

func transcodeRejectRange(r *http.Request) bool {
	return r.Header.Get("Range") != ""
}

func probeVideoCodec(name string) capabilities.VideoCodec {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "h264", "avc", "avc1":
		return capabilities.CodecH264
	case "hevc", "h265":
		return capabilities.CodecHEVC
	case "vp9":
		return capabilities.CodecVP9
	case "av1":
		return capabilities.CodecAV1
	default:
		return capabilities.CodecH264
	}
}

func canFMP4StreamCopy(info ffmpeg.StreamInfo) bool {
	if !info.HasVideo {
		return false
	}
	if !isH264VideoCodec(info.VideoCodec) {
		return false
	}
	audio := strings.ToLower(info.AudioCodec)
	if audio != "" && audio != "aac" {
		return false
	}
	return true
}

func isH264VideoCodec(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "h264", "avc", "avc1":
		return true
	default:
		return false
	}
}

// canH264VideoCopy is true when H.264 can be stream-copied and only audio needs transcoding.
func canH264VideoCopy(info ffmpeg.StreamInfo) bool {
	if !info.HasVideo {
		return false
	}
	if !isH264VideoCodec(info.VideoCodec) {
		return false
	}
	audio := strings.ToLower(strings.TrimSpace(info.AudioCodec))
	return audio != "" && audio != "aac"
}

// hlsNeedsFullVideoTranscode is true when video must be re-encoded (optimized/datasaver, downscale, or non-H.264).
func hlsNeedsFullVideoTranscode(info ffmpeg.StreamInfo, profileMode string) bool {
	switch parseTranscodeProfileMode(profileMode) {
	case transcodeProfileOptimized, transcodeProfileDataSaver:
		return true
	}
	maxH := transcodeMaxHeightForMode(profileMode)
	if maxH > 0 && info.Height > maxH {
		return true
	}
	return !isH264VideoCodec(info.VideoCodec)
}

// hlsUseVideoCopy selects stream-copy for H.264 video with audio-only transcoding (quality path).
func hlsUseVideoCopy(info ffmpeg.StreamInfo, profileMode string) bool {
	if canFMP4StreamCopy(info) || !canH264VideoCopy(info) {
		return false
	}
	return !hlsNeedsFullVideoTranscode(info, profileMode)
}

const (
	transcodeProfileQuality   = "quality"
	transcodeProfileOptimized = "optimized"
	transcodeProfileDataSaver = "datasaver"
	transcodeDataSaverMaxHeight = 720
)

func parseTranscodeProfileMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case transcodeProfileOptimized:
		return transcodeProfileOptimized
	case transcodeProfileDataSaver, "data-saver", "data_saver":
		return transcodeProfileDataSaver
	default:
		return transcodeProfileQuality
	}
}

func transcodeEncodeProfile(info ffmpeg.StreamInfo) encode.VideoProfile {
	return transcodeEncodeProfileForMode(info, transcodeProfileQuality)
}

func transcodeEncodeProfileForMode(info ffmpeg.StreamInfo, mode string) encode.VideoProfile {
	switch parseTranscodeProfileMode(mode) {
	case transcodeProfileOptimized:
		return encode.VideoProfile{
			Codec:   encode.CodecH264,
			Quality: encode.PresetVeryfast,
			Bitrate: transcodeOptimizedBitrateConfig(info),
		}
	case transcodeProfileDataSaver:
		return encode.VideoProfile{
			Codec:   encode.CodecH264,
			Quality: encode.PresetVeryfast,
			Bitrate: transcodeDataSaverBitrateConfig(info),
		}
	default:
		return encode.VideoProfile{
			Codec:   encode.CodecH264,
			Quality: encode.PresetMedium,
			Bitrate: transcodeQualityBitrateConfig(info),
		}
	}
}

func transcodeMaxHeightForMode(profileMode string) int {
	globalMax := transcodeMaxHeight()
	switch parseTranscodeProfileMode(profileMode) {
	case transcodeProfileDataSaver:
		if globalMax > 0 && globalMax < transcodeDataSaverMaxHeight {
			return globalMax
		}
		return transcodeDataSaverMaxHeight
	default:
		return globalMax
	}
}

func transcodeOutputHeightForMode(info ffmpeg.StreamInfo, profileMode string) int {
	maxH := transcodeMaxHeightForMode(profileMode)
	if info.Height <= 0 {
		if maxH > 0 {
			return maxH
		}
		return 720
	}
	if maxH > 0 && info.Height > maxH {
		return maxH
	}
	return info.Height
}

func transcodeTargetVideoKbps(info ffmpeg.StreamInfo) int {
	return transcodeTargetVideoKbpsForMode(info, transcodeProfileQuality)
}

func transcodeTargetVideoKbpsForMode(info ffmpeg.StreamInfo, profileMode string) int {
	outHeight := transcodeOutputHeightForMode(info, profileMode)

	baseline := 1200
	switch {
	case outHeight >= 1080:
		baseline = 5000
	case outHeight >= 720:
		baseline = 3500
	case outHeight >= 480:
		baseline = 2000
	}

	target := baseline
	if info.VideoBitrate > 0 {
		sourceKbps := info.VideoBitrate / 1000
		if info.Height > 0 && outHeight > 0 && outHeight < info.Height {
			scale := float64(outHeight) / float64(info.Height)
			sourceKbps = int(float64(sourceKbps) * scale * scale)
		}
		target = sourceKbps
	}
	if target < baseline {
		target = baseline
	}

	const minKbps = 1500
	const maxKbps = 12000
	if target < minKbps {
		target = minKbps
	}
	if target > maxKbps {
		target = maxKbps
	}
	return target
}

// transcodeQualityBitrateConfig uses variable bitrate with medium preset (preview quality path).
func transcodeQualityBitrateConfig(info ffmpeg.StreamInfo) encode.BitrateConfig {
	targetKbps := transcodeTargetVideoKbpsForMode(info, transcodeProfileQuality)
	return encode.BitrateConfig{
		Target:  fmt.Sprintf("%dk", targetKbps),
		Min:     fmt.Sprintf("%dk", targetKbps/2),
		Max:     fmt.Sprintf("%dk", targetKbps*3/2),
		BufSize: fmt.Sprintf("%dk", targetKbps*2),
	}
}

// transcodeOptimizedBitrateConfig uses a fast preset with a hard bitrate cap.
func transcodeOptimizedBitrateConfig(info ffmpeg.StreamInfo) encode.BitrateConfig {
	targetKbps := transcodeTargetVideoKbpsForMode(info, transcodeProfileOptimized)
	capKbps := targetKbps * 75 / 100
	const minCapKbps = 1000
	if capKbps < minCapKbps {
		capKbps = minCapKbps
	}
	return encode.BitrateConfig{
		Target:  fmt.Sprintf("%dk", capKbps),
		Max:     fmt.Sprintf("%dk", capKbps),
		BufSize: fmt.Sprintf("%dk", capKbps*2),
	}
}

// transcodeDataSaverBitrateConfig caps output at 720p with a lower hard bitrate limit.
func transcodeDataSaverBitrateConfig(info ffmpeg.StreamInfo) encode.BitrateConfig {
	targetKbps := transcodeTargetVideoKbpsForMode(info, transcodeProfileDataSaver)
	const capKbps = 1500
	if targetKbps > capKbps {
		targetKbps = capKbps
	}
	const minKbps = 600
	if targetKbps < minKbps {
		targetKbps = minKbps
	}
	return encode.BitrateConfig{
		Target:  fmt.Sprintf("%dk", targetKbps),
		Max:     fmt.Sprintf("%dk", targetKbps),
		BufSize: fmt.Sprintf("%dk", targetKbps*2),
	}
}

func transcodeDecodeProfile(info ffmpeg.StreamInfo) encode.VideoDecodeProfile {
	if !isKnownInputVideoCodec(info.VideoCodec) {
		// WMV, VC-1, MPEG-4, etc. — let ffmpeg auto-select decoders.
		return encode.VideoDecodeProfile{ForceSoftware: true}
	}
	return encode.VideoDecodeProfile{Codec: probeVideoCodec(info.VideoCodec)}
}

func isKnownInputVideoCodec(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "h264", "avc", "avc1", "hevc", "h265", "vp9", "av1":
		return true
	default:
		return false
	}
}

func transcodeMaxHeight() int {
	return settings.Config.Integrations.Media.Transcode.MaxResolution
}

func serveTranscode(w http.ResponseWriter, r *http.Request, d *requestContext, source, scopedFilePath, displayFileName string) (int, error) {
	if !settings.TranscodeEnabled() {
		return http.StatusNotFound, fmt.Errorf("transcode not enabled")
	}

	svc := ffmpeg.Get()
	if svc == nil {
		return http.StatusServiceUnavailable, fmt.Errorf("ffmpeg unavailable")
	}

	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusInternalServerError, fmt.Errorf("source %s is not available", source)
	}

	if !accessStore.Permitted(idx.Path, utils.IndexPathFromNormalized(scopedFilePath, true), d.user.Username) {
		return http.StatusForbidden, fmt.Errorf("access denied to path %s", scopedFilePath)
	}

	realPath, _, err := idx.GetRealPath(scopedFilePath)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	acquire := activeTranscodeSessions.acquire(d.user.ID, d.user.Username, source, scopedFilePath, displayFileName)
	if !acquire.OK {
		logger.Infof(
			"transcode handler denied: user=%d (%s) source=%s path=%s reason=%s",
			d.user.ID, d.user.Username, source, scopedFilePath, acquire.Reason,
		)
		switch acquire.Reason {
		case "user_limit":
			return http.StatusConflict, fmt.Errorf("transcode user limit reached")
		default:
			return http.StatusServiceUnavailable, fmt.Errorf("transcode system limit reached")
		}
	}
	sessionKey := acquire.Session.ID
	logger.Infof(
		"transcode handler start: user=%d (%s) key=%s file=%q remote=%s",
		d.user.ID, d.user.Username, sessionKey, displayFileName, r.RemoteAddr,
	)
	var releaseStream sync.Once
	endStream := func(reason string) {
		releaseStream.Do(func() {
			logger.Infof("transcode handler end (%s): user=%d key=%s", reason, d.user.ID, sessionKey)
			activeTranscodeSessions.releaseStream(sessionKey)
		})
	}
	defer endStream("handler-return")
	go func() {
		<-r.Context().Done()
		endStream("client-disconnect")
	}()

	info, err := svc.ProbeFile(r.Context(), realPath)
	if err != nil || !info.IsValid {
		return http.StatusInternalServerError, fmt.Errorf("probe failed: %v", err)
	}

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Accept-Ranges", "none")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	flusher, _ := w.(http.Flusher)

	var streamErr error
	if canFMP4StreamCopy(info) {
		streamErr = svc.FMP4StreamCopy(r.Context(), w, realPath)
	} else {
		streamErr = svc.FMP4Transcode(r.Context(), w, realPath, transcodeDecodeProfile(info), transcodeEncodeProfile(info), transcodeMaxHeight())
	}
	if flusher != nil {
		flusher.Flush()
	}

	if streamErr != nil {
		if r.Context().Err() != nil {
			return 0, nil
		}
		logger.Debugf("transcode stream ended for %s: %v", displayFileName, streamErr)
		return http.StatusInternalServerError, streamErr
	}
	return http.StatusOK, nil
}

// transcodeSessionsHandler lists active transcode sessions and whether a new one can start.
// @Summary List active transcode sessions
// @Description Returns active transcode jobs for the current user, or all users when all=true and caller is admin. Optional source/file query params evaluate start eligibility for a target file.
// @Tags Media
// @Param source query string false "Source name for start eligibility check"
// @Param file query string false "File path for start eligibility check"
// @Param all query bool false "List all sessions (admin only)"
// @Router /api/media/transcode/sessions [get]
func transcodeSessionsHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !settings.TranscodeEnabled() {
		return http.StatusNotFound, fmt.Errorf("transcode not enabled")
	}

	listAll := r.URL.Query().Get("all") == "true"
	if listAll && !d.user.Permissions.Admin {
		return http.StatusForbidden, fmt.Errorf("admin permission required")
	}

	source := r.URL.Query().Get("source")
	targetPath := r.URL.Query().Get("file")
	if targetPath != "" {
		clean, err := utils.SanitizePath(targetPath)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid file path: %v", err)
		}
		targetPath = clean
	}

	var resp TranscodeSessionsResponse
	if source != "" && targetPath != "" {
		if userscope, err := d.user.GetScopeForSourceName(source); err == nil {
			targetPath = utils.JoinPathAsUnix(userscope, targetPath)
		}
		resp = activeTranscodeSessions.evaluate(d.user.ID, source, targetPath)
		if listAll {
			all := activeTranscodeSessions.list(d.user.ID, true)
			resp.Sessions = all
			resp.ActiveCount = len(all)
		}
	} else {
		sessions := activeTranscodeSessions.list(d.user.ID, listAll)
		resp = TranscodeSessionsResponse{
			SystemLimit: transcodeSystemLimit(),
			UserLimit:   transcodePerUserLimit,
			ActiveCount: len(sessions),
			Sessions:    sessions,
			CanStart:    true,
		}
	}

	return renderJSON(w, r, resp)
}

// transcodeSessionReleaseHandler releases an active transcode session for the given file.
// @Summary Release a transcode session
// @Description Decrements stream count and clears session state for source/file owned by the current user.
// @Tags Media
// @Param source query string true "Source name"
// @Param file query string true "File path"
// @Router /api/media/transcode/sessions [delete]
func transcodeSessionReleaseHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !settings.TranscodeEnabled() {
		return http.StatusNotFound, fmt.Errorf("transcode not enabled")
	}

	source := r.URL.Query().Get("source")
	if source == "" {
		return http.StatusBadRequest, fmt.Errorf("source required")
	}
	targetPath := r.URL.Query().Get("file")
	if targetPath == "" {
		activeTranscodeSessions.releaseAllForUser(d.user.ID)
		w.WriteHeader(http.StatusNoContent)
		return http.StatusNoContent, nil
	}
	clean, err := utils.SanitizePath(targetPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid file path: %v", err)
	}
	targetPath = clean
	if userscope, err := d.user.GetScopeForSourceName(source); err == nil {
		targetPath = utils.JoinPathAsUnix(userscope, targetPath)
	}

	activeTranscodeSessions.releaseForUserFile(d.user.ID, source, targetPath)
	w.WriteHeader(http.StatusNoContent)
	return http.StatusNoContent, nil
}

// transcodeHandler serves live fMP4 for preview playback when the browser cannot decode the source.
// @Summary Transcode media for inline preview playback
// @Description Live fragmented MP4 stream for MSE playback. Requires streamToken and authenticated user. Does not support Range requests and never counts as a download.
// @Tags Media
// @Param source query string true "Source name"
// @Param file query string true "File path"
// @Param streamToken query string true "Opaque stream grant token from file metadata"
// @Success 200 {file} file "Fragmented MP4 stream"
// @Failure 403 {object} map[string]string "Missing or invalid stream token"
// @Failure 409 {object} map[string]string "User transcode limit reached"
// @Failure 416 {object} map[string]string "Range requests not supported"
// @Failure 503 {object} map[string]string "System transcode limit reached"
// @Router /api/media/transcode [get]
func transcodeHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if transcodeRejectRange(r) {
		return http.StatusRequestedRangeNotSatisfiable, fmt.Errorf("range requests not supported for transcode")
	}
	source := r.URL.Query().Get("source")
	fileList := r.URL.Query()["file"]
	if len(fileList) != 1 {
		return http.StatusForbidden, fmt.Errorf("transcode supports single file only")
	}
	token := r.URL.Query().Get("streamToken")
	if token == "" {
		return http.StatusForbidden, fmt.Errorf("stream token required")
	}
	cleanPath, err := utils.SanitizePath(fileList[0])
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid file path: %v", err)
	}
	if err = validateStreamGrant(token, d, source, cleanPath); err != nil {
		return http.StatusForbidden, err
	}

	userscope, err := d.user.GetScopeForSourceName(source)
	if err != nil {
		return http.StatusForbidden, err
	}
	scopedPath := utils.JoinPathAsUnix(userscope, cleanPath)
	displayName := filepath.Base(scopedPath)
	return serveTranscode(w, r, d, source, scopedPath, displayName)
}
