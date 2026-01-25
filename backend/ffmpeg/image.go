package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
)

// GetImageOrientation extracts the EXIF orientation from an image file using exiftool
func (s *FFmpegService) GetImageOrientation(imagePath string) (string, error) {
	// Use exiftool to get orientation information
	cmd := exec.Command("exiftool", "-Orientation", "-s3", imagePath)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return "Horizontal (normal)", nil // Default to normal orientation
	}

	orientation := strings.TrimSpace(out.String())
	if orientation == "" {
		orientation = "Horizontal (normal)" // Default if no orientation found
	}
	return orientation, nil
}

// GetOrientationFilter converts EXIF orientation to FFmpeg filter
func (s *FFmpegService) GetOrientationFilter(orientation string) string {
	switch orientation {
	// Text-based EXIF orientation values (from exiftool)
	case "Rotate 90 CW", "Right-top":
		return ",transpose=1" // 90° clockwise
	case "Rotate 180", "Bottom-right":
		return ",transpose=1,transpose=1" // 180° (two 90° rotations)
	case "Rotate 270 CW", "Left-bottom":
		return ",transpose=2" // 90° counter-clockwise
	case "Mirror horizontal", "Top-right":
		return ",hflip" // Horizontal flip
	case "Mirror vertical", "Bottom-left":
		return ",vflip" // Vertical flip (upside down)
	case "Mirror horizontal and rotate 270 CW", "Right-bottom":
		return ",transpose=0" // 90° counter-clockwise + horizontal flip
	case "Mirror horizontal and rotate 90 CW", "Left-top":
		return ",transpose=3" // 90° clockwise + horizontal flip
	case "Horizontal (normal)", "Top-left":
		return "" // No rotation needed

	// Numeric EXIF orientation values (standard EXIF specification)
	case "1":
		return "" // Top-left (normal)
	case "2":
		return ",hflip" // Top-right (flip horizontal)
	case "3":
		return ",transpose=1,transpose=1" // Bottom-right (rotate 180)
	case "4":
		return ",vflip" // Bottom-left (flip vertical)
	case "5":
		return ",transpose=3" // Left-top (transpose: 90° CW + horizontal flip)
	case "6":
		return ",transpose=1" // Right-top (rotate 90 CW)
	case "7":
		return ",transpose=0" // Right-bottom (transverse: 90° CCW + horizontal flip)
	case "8":
		return ",transpose=2" // Left-bottom (rotate 270 CW / 90° CCW)

	default:
		return "" // Default to no rotation for unknown orientations
	}
}

// GetImageDimensions extracts the dimensions of an image file using ffprobe
func (s *FFmpegService) GetImageDimensions(imagePath string) (width, height int, err error) {

	// Get HEIC dimensions (fallback to individual stream dimensions for nowe re
	probeCmd := exec.Command(
		s.ffprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0:s=x",
		imagePath,
	)

	var probeOut bytes.Buffer
	var probeErr bytes.Buffer
	probeCmd.Stdout = &probeOut
	probeCmd.Stderr = &probeErr

	if err = probeCmd.Run(); err != nil {
		return 0, 0, fmt.Errorf("ffprobe command failed on image file '%s': %w", imagePath, err)
	}

	dimensions := strings.TrimSpace(probeOut.String())

	// Handle cases where ffprobe returns multiple lines or extra characters
	// Take the first line and clean it up
	lines := strings.Split(dimensions, "\n")
	cleanDimensions := strings.TrimSpace(lines[0])

	// Remove any trailing 'x' characters that might appear
	cleanDimensions = strings.TrimSuffix(cleanDimensions, "x")

	parts := strings.Split(cleanDimensions, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid dimensions format: %s", cleanDimensions)
	}

	width, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width: %w", err)
	}

	height, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid height: %w", err)
	}

	return width, height, nil
}

// ConvertHEICToJPEGDirect converts a HEIC file to JPEG using direct FFmpeg conversion (fast method)
func (s *FFmpegService) ConvertHEICToJPEGDirect(ctx context.Context, heicPath string, targetWidth, targetHeight int, quality string) ([]byte, error) {
	// Check if context is cancelled before starting
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Get EXIF orientation and create appropriate filter
	orientation, err := s.GetImageOrientation(heicPath)
	if err != nil {
		orientation = "Horizontal (normal)"
	}
	orientationFilter := s.GetOrientationFilter(orientation)

	// Create temporary output file
	outputDir := s.cacheDir
	err = os.MkdirAll(outputDir, fileutils.PermDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	outputFile := filepath.Join(outputDir, fmt.Sprintf("heic_direct_%d.jpg", os.Getpid()))
	defer os.Remove(outputFile)

	// Build FFmpeg command for direct conversion
	args := []string{"-i", heicPath}

	// Build video filter chain with scaling and orientation
	var filterChain string
	if targetWidth > 0 && targetHeight > 0 {
		filterChain = fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease%s", targetWidth, targetHeight, orientationFilter)
	} else {
		// For original size, only apply orientation if needed
		if orientationFilter != "" {
			filterChain = orientationFilter[1:] // Remove leading comma
		}
	}

	// Add video filter if we have one
	if filterChain != "" {
		args = append(args, "-vf", filterChain)
	}

	// Add quality and output settings
	args = append(args, "-q:v", quality, "-pix_fmt", "yuvj420p", "-y", outputFile)

	cmd := exec.CommandContext(ctx, s.ffmpegPath, args...)
	var cmdOut bytes.Buffer
	var cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr

	if err = cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("direct HEIC conversion failed: %w", err)
	}

	// Read the converted file
	jpegBytes, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read converted JPEG: %w", err)
	}

	return jpegBytes, nil
}

// ConvertHEICToJPEG converts a HEIC file to JPEG with specified dimensions and quality using proper tile extraction
func (s *FFmpegService) ConvertHEICToJPEG(ctx context.Context, heicPath string, targetWidth, targetHeight int, quality string) ([]byte, error) {
	// Check if context is cancelled before starting
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	// Create temporary directory for tile processing
	tempDir, err := os.MkdirTemp(s.cacheDir, "heic_convert")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	defer os.RemoveAll(tempDir)

	// Step 1: Get grid info using trace output

	// Use ffprobe with trace level to get grid information (with time limit for speed)
	gridCmd := exec.CommandContext(ctx, s.ffmpegPath, "-loglevel", "trace", "-i", heicPath, "-f", "null", "-", "-t", "0.1")
	var gridOut bytes.Buffer
	var gridErr bytes.Buffer
	gridCmd.Stdout = &gridOut
	gridCmd.Stderr = &gridErr
	_ = gridCmd.Run() // Don't check error, we're just extracting info

	traceOutput := gridErr.String()

	// Parse grid dimensions from trace output
	var gridCols, gridRows = 8, 6 // Default fallback
	var val int

	// Look for grid_row and grid_col in trace output
	lines := strings.Split(traceOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "grid_row") {
			// Try to extract grid_row value
			if parts := strings.Split(line, "grid_row"); len(parts) > 1 {
				if matches := strings.Fields(parts[1]); len(matches) > 0 {
					if val, err = strconv.Atoi(strings.Trim(matches[0], " :=")); err == nil {
						gridRows = val + 1 // HEIC grid_row is 0-based
					}
				}
			}
		}
		if strings.Contains(line, "grid_col") {
			// Try to extract grid_col value
			if parts := strings.Split(line, "grid_col"); len(parts) > 1 {
				if matches := strings.Fields(parts[1]); len(matches) > 0 {
					if val, err = strconv.Atoi(strings.Trim(matches[0], " :=")); err == nil {
						gridCols = val + 1 // HEIC grid_col is 0-based
					}
				}
			}
		}
	}

	// Get EXIF orientation and create appropriate filter
	orientation, err := s.GetImageOrientation(heicPath)
	if err != nil {
		orientation = "Horizontal (normal)"
	}
	orientationFilter := s.GetOrientationFilter(orientation)

	// Step 2: Extract tile streams (skip stream 0 which is compatibility image)

	// Check context again before extraction
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Extract using JPEG for faster I/O (PNG was unnecessarily slow and large)
	// Skip the first output (stream 0) since it's the compatibility image
	tilesPattern := filepath.Join(tempDir, "output_%d.jpg")

	extractCmd := exec.CommandContext(ctx,
		s.ffmpegPath,
		"-i", heicPath,
		"-map", "0", // Map all streams
		"-q:v", "3", // Medium quality for intermediate tiles
		"-y",
		tilesPattern,
	)

	var extractOut bytes.Buffer
	var extractErr bytes.Buffer
	extractCmd.Stdout = &extractOut
	extractCmd.Stderr = &extractErr

	if err = extractCmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("tile extraction failed: %w", err)
	}

	// Step 3: Filter tiles (skip output_0.png which is compatibility image)

	files, err := filepath.Glob(filepath.Join(tempDir, "output_*.jpg"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no tiles extracted, found %d files", len(files))
	}

	// Create filtered tiles, skipping output_0.jpg (compatibility image)
	// and rename them to sequential tile_001.jpg, tile_002.jpg, etc.
	tileIndex := 1
	expectedTiles := gridCols * gridRows

	for i := 1; i <= len(files); i++ { // Start from 1 to skip output_0.jpg
		sourceFile := filepath.Join(tempDir, fmt.Sprintf("output_%d.jpg", i))
		var info os.FileInfo
		if info, err = os.Stat(sourceFile); err == nil && info.Size() > 500 {
			// This is a valid tile, rename it to sequential format
			targetFile := filepath.Join(tempDir, fmt.Sprintf("tile_%03d.jpg", tileIndex))
			if err = os.Rename(sourceFile, targetFile); err != nil {
				continue
			}
			tileIndex++

			// Stop if we have enough tiles
			if tileIndex > expectedTiles {
				break
			}
		}
	}

	actualTiles := tileIndex - 1

	// Check context before merge
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Step 4: Merge tiles back into final image with proper quality and rotation
	outputFile := filepath.Join(tempDir, "final_output.jpg")

	// Build the video filter chain
	inputPattern := filepath.Join(tempDir, "tile_%03d.jpg")

	// Create filter chain: tile assembly + proper scaling
	var filterChain string

	// For original size (targetWidth=0, targetHeight=0), don't scale
	if targetWidth == 0 && targetHeight == 0 {
		// This is original size - no scaling, maximum quality
		// Use padding=0:margin=0 to eliminate black borders between tiles
		filterChain = fmt.Sprintf("tile=%dx%d:nb_frames=%d:padding=0:margin=0%s",
			gridCols, gridRows, actualTiles, orientationFilter)
	} else {
		// This is a resize request - apply scaling with proper aspect handling
		filterChain = fmt.Sprintf("tile=%dx%d:nb_frames=%d:padding=0:margin=0%s,scale=%d:%d:force_original_aspect_ratio=decrease:eval=frame",
			gridCols, gridRows, actualTiles, orientationFilter, targetWidth, targetHeight)
	}

	mergeCmd := exec.CommandContext(ctx,
		s.ffmpegPath,
		"-i", inputPattern,
		"-vf", filterChain,
		"-frames:v", "1",
		"-q:v", quality, // Use the quality setting passed in
		"-pix_fmt", "yuvj420p", // Use full-range YUV for better JPEG compatibility
		"-y",
		outputFile,
	)

	var mergeOut bytes.Buffer
	var mergeErr bytes.Buffer
	mergeCmd.Stdout = &mergeOut
	mergeCmd.Stderr = &mergeErr

	if err = mergeCmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("tile merge failed: %w", err)
	}

	// Step 5: Read final output
	jpegBytes, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read merged JPEG: %w", err)
	}

	return jpegBytes, nil
}

// ConvertImageToJPEG converts any image format (including problematic JPEGs) to standard JPEG using FFmpeg
// This handles extended JPEG formats that Go's image/jpeg decoder doesn't support (e.g., SOF1/C1 marker)
// and can be used as a fallback for images that fail to decode with Go's standard library.
func (s *FFmpegService) ConvertImageToJPEG(ctx context.Context, imagePath string, targetWidth, targetHeight int, quality string) ([]byte, error) {
	// Use the direct conversion method which works for all image formats
	return s.ConvertHEICToJPEGDirect(ctx, imagePath, targetWidth, targetHeight, quality)
}
