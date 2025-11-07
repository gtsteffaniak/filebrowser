package ffmpeg

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// VideoService handles video preview operations with ffmpeg
type FFmpegService struct {
	ffmpegPath    string
	ffprobePath   string
	debug         bool
	semaphore     chan struct{}
	cacheDir      string
	maxConcurrent int // For logging purposes
}

// NewVideoService creates a new video service instance
func NewFFmpegService(maxConcurrent int, debug bool, cacheDir string) *FFmpegService {
	if settings.Env.FFmpegPath == "" || settings.Env.FFprobePath == "" {
		return nil
	}
	return &FFmpegService{
		ffmpegPath:    settings.Env.FFmpegPath,
		ffprobePath:   settings.Env.FFprobePath,
		debug:         debug,
		semaphore:     make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
		cacheDir:      cacheDir,
	}
}

func (s *FFmpegService) Acquire(ctx context.Context) error {
	select {
	case s.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *FFmpegService) Release() {
	<-s.semaphore
}

// CheckValidFFmpeg checks for a valid ffmpeg executable.
// If a path is provided, it looks there. Otherwise, it searches the system's PATH.
func CheckValidFFmpeg(path string) (string, error) {
	return checkExecutable(path, "ffmpeg")
}

// CheckValidFFprobe checks for a valid ffprobe executable.
// If a path is provided, it looks there. Otherwise, it searches the system's PATH.
func CheckValidFFprobe(path string) (string, error) {
	return checkExecutable(path, "ffprobe")
}

// checkExecutable is an internal helper function to find and validate an executable.
// It checks a specific path if provided, otherwise falls back to searching the system PATH.
func checkExecutable(providedPath, execName string) (string, error) {
	// Add .exe extension for Windows systems
	if runtime.GOOS == "windows" {
		execName += ".exe"
	}

	var finalPath string
	var err error

	if providedPath != "" {
		// A path was provided, so we'll use it.
		finalPath = filepath.Join(providedPath, execName)
	} else {
		// No path was provided, so search the system's PATH for the executable.
		finalPath, err = exec.LookPath(execName)
		if err != nil {
			// The executable was not found in the system's PATH.
			return "", err
		}
	}

	// Verify the executable is valid by running the "-version" command.
	cmd := exec.Command(finalPath, "-version")
	err = cmd.Run()

	return finalPath, err
}

func SetFFmpegPaths() {
	ffmpegMainPath, err := CheckValidFFmpeg(settings.Env.FFmpegPath)
	if err != nil && settings.Env.FFmpegPath != "" {
		logger.Warningf("the configured ffmpeg path does not contain a valid ffmpeg binary %s, err: %v", settings.Env.FFmpegPath, err)
	}
	ffprobePath, errprobe := CheckValidFFprobe(settings.Env.FFprobePath)
	if errprobe != nil && settings.Env.FFprobePath != "" {
		logger.Warningf("the configured ffmpeg path is not a valid ffprobe binary %s, err: %v", settings.Env.FFprobePath, err)
	}
	settings.Env.FFmpegPath = ffmpegMainPath
	settings.Env.FFprobePath = ffprobePath
}
