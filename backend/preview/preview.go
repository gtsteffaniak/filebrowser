package preview

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/diskcache"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

var (
	ErrUnsupportedFormat = errors.New("preview is not available for provided file format")
	ErrUnsupportedMedia  = errors.New("unsupported media type")
	service              *Service
)

type Service struct {
	sem         chan struct{}
	ffmpegPath  string
	ffprobePath string
	fileCache   diskcache.Interface
	debug       bool
	docGenMutex sync.Mutex // Mutex to serialize access to doc generation
}

func NewPreviewGenerator(concurrencyLimit int, ffmpegPath string, cacheDir string) *Service {
	var fileCache diskcache.Interface
	// Use file cache if cacheDir is specified
	if cacheDir != "" {
		var err error
		fileCache, err = diskcache.NewFileCache(cacheDir)
		if err != nil {
			if cacheDir == "tmp" {
				logger.Error("The cache dir could not be created. Make sure the user that you executed the program with has access to create directories in the local path. filebrowser is trying to use the default `server.cacheDir: tmp` , but you can change this location if you need to. Please see configuration wiki for more information about this error. https://github.com/gtsteffaniak/filebrowser/wiki/Configuration")
			}
			logger.Fatalf("failed to create file cache path, which is now require to run the server: %v", err)
		}
	} else {
		// No-op cache if no cacheDir is specified
		fileCache = diskcache.NewNoOp()
	}
	// Create directories recursively
	err := os.MkdirAll(filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "docs"), fileutils.GetDirectoryPermissions())
	if err != nil {
		logger.Error(err)
	}
	err = os.MkdirAll(filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "videos"), fileutils.GetDirectoryPermissions())
	if err != nil {
		logger.Error(err)
	}
	err = os.MkdirAll(filepath.Join(settings.Config.Server.CacheDir, "heic"), fileutils.GetDirectoryPermissions())
	if err != nil {
		logger.Error(err)
	}
	ffmpegMainPath, err := CheckValidFFmpeg(ffmpegPath)
	if err != nil && ffmpegPath != "" {
		logger.Fatalf("the configured ffmpeg path does not contain a valid ffmpeg binary %s, err: %v", ffmpegPath, err)
	}
	ffprobePath, errprobe := CheckValidFFprobe(ffmpegPath)
	if errprobe != nil && ffmpegPath != "" {
		logger.Fatalf("the configured ffmpeg path is not a valid ffprobe binary %s, err: %v", ffmpegPath, err)
	}
	if errprobe == nil && err == nil {
		settings.Config.Integrations.Media.FfmpegPath = filepath.Base(ffmpegMainPath)
	}
	logger.Debugf("Media Enabled            : %v", ffmpegMainPath != "" && ffprobePath != "")
	settings.Config.Server.MuPdfAvailable = docEnabled()
	logger.Debugf("MuPDF Enabled            : %v", settings.Config.Server.MuPdfAvailable)
	return &Service{
		sem:         make(chan struct{}, concurrencyLimit),
		ffmpegPath:  ffmpegMainPath,
		ffprobePath: ffprobePath,
		fileCache:   fileCache,
		debug:       settings.Config.Server.DebugMedia,
	}
}

func StartPreviewGenerator(concurrencyLimit int, ffmpegPath, cacheDir string) error {
	service = NewPreviewGenerator(concurrencyLimit, ffmpegPath, cacheDir)
	return nil
}

func GetPreviewForFile(file iteminfo.ExtendedFileInfo, previewSize, url string, seekPercentage int) ([]byte, error) {
	if !file.HasPreview {
		return nil, ErrUnsupportedMedia
	}
	var thisMd5 string
	if file.AudioMeta != nil && file.AudioMeta.AlbumArt != "" {
		// md5 is based on album art
		// md5 file.AlbumArt
		hasher := md5.New()
		_, _ = hasher.Write([]byte(file.AudioMeta.AlbumArt))
		thisMd5 = hex.EncodeToString(hasher.Sum(nil))
		file.Checksums = make(map[string]string)
		file.Checksums["md5"] = thisMd5
	} else {
		var err error
		thisMd5, err = utils.GetChecksum(file.RealPath, "md5")
		if err != nil {
			return nil, fmt.Errorf("failed to get checksum: %w", err)
		}
		// Ensure the file.Checksums map is initialized and MD5 is set
		if file.Checksums == nil {
			file.Checksums = make(map[string]string)
		}
		file.Checksums["md5"] = thisMd5
	}
	cacheKey := CacheKey(thisMd5, previewSize, seekPercentage)
	if data, found, err := service.fileCache.Load(context.Background(), cacheKey); err != nil {
		return nil, fmt.Errorf("failed to load from cache: %w", err)
	} else if found {
		return data, nil
	}
	return GeneratePreview(file, previewSize, url, seekPercentage)
}

func GeneratePreview(file iteminfo.ExtendedFileInfo, previewSize, officeUrl string, seekPercentage int) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(file.Name))
	var (
		err        error
		imageBytes []byte
	)

	// Generate thumbnail image from video
	hasher := md5.New()
	_, _ = hasher.Write([]byte(CacheKey(file.Checksums["md5"], previewSize, seekPercentage)))
	hash := hex.EncodeToString(hasher.Sum(nil))
	// Generate an image from office document
	if iteminfo.HasDocConvertableExtension(file.Name, file.Type) {
		tempFilePath := filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "docs", hash) + ".txt"
		imageBytes, err = service.GenerateImageFromDoc(file, tempFilePath, 0) // 0 for the first page
		if err != nil {
			return nil, fmt.Errorf("failed to create image for PDF file: %w", err)
		}
	} else if file.OnlyOfficeId != "" {
		imageBytes, err = service.GenerateOfficePreview(filepath.Ext(file.Name), file.OnlyOfficeId, file.Name, officeUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to create image for office file: %w", err)
		}
	} else if strings.HasPrefix(file.Type, "image/heic") {
		// HEIC files need FFmpeg conversion to JPEG with proper size/quality handling
		imageBytes, err = service.convertHEICToJPEGWithFFmpeg(file.RealPath, previewSize)
		if err != nil {
			return nil, fmt.Errorf("failed to process HEIC image file: %w", err)
		}
		// For HEIC files, we've already done the resize/conversion, so cache and return directly
		cacheKey := CacheKey(file.Checksums["md5"], previewSize, seekPercentage)
		if err = service.fileCache.Store(context.Background(), cacheKey, imageBytes); err != nil {
			logger.Errorf("failed to cache HEIC image: %v", err)
		}
		return imageBytes, nil
	} else if strings.HasPrefix(file.Type, "image") {
		imageBytes, err = os.ReadFile(file.RealPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read image file: %w", err)
		}
	} else if strings.HasPrefix(file.Type, "video") {
		if seekPercentage == 0 {
			seekPercentage = 10
		}
		outPathPattern := filepath.Join(settings.Config.Server.CacheDir, "thumbnails", "videos", hash) + ".jpg"
		defer os.Remove(outPathPattern) // cleanup
		imageBytes, err = service.GenerateVideoPreview(file.RealPath, outPathPattern, seekPercentage)
		if err != nil {
			return nil, fmt.Errorf("failed to create image for video file: %w", err)
		}
	} else if strings.HasPrefix(file.Type, "audio") {
		// Extract album artwork from audio files
		if file.AudioMeta != nil && file.AudioMeta.AlbumArt != "" {
			imageBytes, err = base64.StdEncoding.DecodeString(file.AudioMeta.AlbumArt)
			if err != nil {
				return nil, fmt.Errorf("failed to decode album artwork: %w", err)
			}
		} else {
			return nil, fmt.Errorf("no album artwork available for audio file: %s", file.Name)
		}
	} else {
		return nil, fmt.Errorf("unsupported media type: %s", ext)
	}
	if len(imageBytes) < 100 {
		return nil, fmt.Errorf("generated image is too small, likely an error occurred: %d bytes", len(imageBytes))
	}

	if previewSize != "original" {
		// resize image
		resizedBytes, err := service.CreatePreview(imageBytes, previewSize)
		if err != nil {
			return nil, fmt.Errorf("failed to resize preview image: %w", err)
		}
		// Cache and return
		cacheKey := CacheKey(file.Checksums["md5"], previewSize, seekPercentage)
		if err := service.fileCache.Store(context.Background(), cacheKey, resizedBytes); err != nil {
			logger.Errorf("failed to cache resized image: %v", err)
		}
		return resizedBytes, nil
	} else {
		cacheKey := CacheKey(file.Checksums["md5"], previewSize, seekPercentage)
		if err := service.fileCache.Store(context.Background(), cacheKey, imageBytes); err != nil {
			logger.Errorf("failed to cache original image: %v", err)
		}
		return imageBytes, nil
	}

}

func (s *Service) CreatePreview(data []byte, previewSize string) ([]byte, error) {
	var (
		width   int
		height  int
		options []Option
	)

	switch previewSize {
	case "large":
		width, height = 640, 640
		options = []Option{WithMode(ResizeModeFit), WithQuality(QualityHigh), WithFormat(FormatJpeg)}
	case "small":
		width, height = 256, 256
		options = []Option{WithMode(ResizeModeFit), WithQuality(QualityMedium), WithFormat(FormatJpeg)}
	default:
		return nil, ErrUnsupportedFormat
	}

	input := bytes.NewReader(data)
	output := &bytes.Buffer{}

	if err := s.Resize(input, width, height, output, options...); err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

func CacheKey(md5, previewSize string, percentage int) string {
	return fmt.Sprintf("%x%x%x", md5, previewSize, percentage)
}

func DelThumbs(ctx context.Context, file iteminfo.ExtendedFileInfo) {
	errSmall := service.fileCache.Delete(ctx, CacheKey(file.Checksums["md5"], "small", 0))
	if errSmall != nil {
		errLarge := service.fileCache.Delete(ctx, CacheKey(file.Checksums["md5"], "large", 0))
		if errLarge != nil {
			logger.Debugf("Could not delete thumbnail: %v", file.Name)
		}
	}
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
