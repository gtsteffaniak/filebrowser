package ffmpeg

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ImageService handles image operations with ffmpeg
type ImageService struct {
	ffmpegPath  string
	ffprobePath string
	debug       bool
}

// NewImageService creates a new image service instance
func NewImageService(ffmpegPath, ffprobePath string, debug bool) *ImageService {
	return &ImageService{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		debug:       debug,
	}
}

// GetImageDimensions extracts the dimensions of an image file using ffprobe
func (s *ImageService) GetImageDimensions(imagePath string) (width, height int, err error) {
	fmt.Printf("🔍 IMAGE SERVICE: Getting dimensions for %s\n", filepath.Base(imagePath))
	fmt.Printf("🔧 IMAGE SERVICE: Using ffprobe at: %s\n", s.ffprobePath)

	// Get HEIC dimensions (fallback to individual stream dimensions for nowe re
	probeCmd := exec.Command(
		s.ffprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0:s=x",
		imagePath,
	)

	fmt.Printf("🚀 IMAGE SERVICE: Running command: %s %s\n", s.ffprobePath, strings.Join(probeCmd.Args[1:], " "))

	var probeOut bytes.Buffer
	var probeErr bytes.Buffer
	probeCmd.Stdout = &probeOut
	probeCmd.Stderr = &probeErr

	if err = probeCmd.Run(); err != nil {
		fmt.Printf("❌ IMAGE SERVICE: ffprobe failed with error: %v\n", err)
		fmt.Printf("❌ IMAGE SERVICE: ffprobe stderr: %s\n", probeErr.String())
		return 0, 0, fmt.Errorf("ffprobe command failed on image file '%s': %w", imagePath, err)
	}

	dimensions := strings.TrimSpace(probeOut.String())
	fmt.Printf("📏 IMAGE SERVICE: Got raw dimensions string: '%s'\n", dimensions)

	// Handle cases where ffprobe returns multiple lines or extra characters
	// Take the first line and clean it up
	lines := strings.Split(dimensions, "\n")
	cleanDimensions := strings.TrimSpace(lines[0])

	// Remove any trailing 'x' characters that might appear
	cleanDimensions = strings.TrimSuffix(cleanDimensions, "x")

	fmt.Printf("📏 IMAGE SERVICE: Cleaned dimensions string: '%s'\n", cleanDimensions)

	parts := strings.Split(cleanDimensions, "x")
	if len(parts) != 2 {
		fmt.Printf("❌ IMAGE SERVICE: Invalid dimensions format after cleaning: %s\n", cleanDimensions)
		return 0, 0, fmt.Errorf("invalid dimensions format: %s", cleanDimensions)
	}

	width, err = strconv.Atoi(parts[0])
	if err != nil {
		fmt.Printf("❌ IMAGE SERVICE: Invalid width: %s\n", parts[0])
		return 0, 0, fmt.Errorf("invalid width: %w", err)
	}

	height, err = strconv.Atoi(parts[1])
	if err != nil {
		fmt.Printf("❌ IMAGE SERVICE: Invalid height: %s\n", parts[1])
		return 0, 0, fmt.Errorf("invalid height: %w", err)
	}

	fmt.Printf("✅ IMAGE SERVICE: Parsed dimensions: %dx%d\n", width, height)
	return width, height, nil
}

// ConvertHEICToJPEG converts a HEIC file to JPEG with specified dimensions and quality using proper tile extraction
func (s *ImageService) ConvertHEICToJPEG(heicPath string, targetWidth, targetHeight int, quality string) ([]byte, error) {
	fmt.Printf("🎯 IMAGE SERVICE: Converting HEIC %s to JPEG (%dx%d, quality %s) using proper tile extraction\n", filepath.Base(heicPath), targetWidth, targetHeight, quality)

	// Create temporary directory for tile processing
	outputDir := filepath.Dir(heicPath)
	tempDir := filepath.Join(outputDir, fmt.Sprintf("heic_tiles_%d", os.Getpid()))
	err := os.MkdirAll(tempDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		fmt.Printf("🧹 IMAGE SERVICE: Cleaning up temporary directory: %s\n", tempDir)
		os.RemoveAll(tempDir)
	}()

	// Step 1: Get grid info using trace output
	fmt.Printf("📐 IMAGE SERVICE: Step 1 - Getting grid info from HEIC trace\n")

	// Use ffprobe with trace level to get grid information
	gridCmd := exec.Command(s.ffmpegPath, "-loglevel", "trace", "-i", heicPath, "-f", "null", "-")
	var gridOut bytes.Buffer
	var gridErr bytes.Buffer
	gridCmd.Stdout = &gridOut
	gridCmd.Stderr = &gridErr
	_ = gridCmd.Run() // Don't check error, we're just extracting info

	traceOutput := gridErr.String()
	fmt.Printf("📊 IMAGE SERVICE: Got trace output: %d bytes\n", len(traceOutput))

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
						fmt.Printf("📐 IMAGE SERVICE: Parsed grid_row: %d (rows: %d)\n", val, gridRows)
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
						fmt.Printf("📐 IMAGE SERVICE: Parsed grid_col: %d (cols: %d)\n", val, gridCols)
					}
				}
			}
		}
	}

	fmt.Printf("📊 IMAGE SERVICE: Using grid dimensions: %dx%d\n", gridCols, gridRows)

	// Use FFmpeg's auto-orientation feature instead of forced rotation
	fmt.Printf("🔄 IMAGE SERVICE: Using FFmpeg auto-orientation for proper image orientation\n")
	orientationFilter := ",auto-orient=1"

	// Step 2: Extract tile streams (skip stream 0 which is compatibility image)
	fmt.Printf("🔧 IMAGE SERVICE: Step 2 - Extracting tile streams\n")

	// Extract using the exact approach that works: ffmpeg -i input.heic -map 0 output_%d.png
	// But we'll skip the first output (stream 0) since it's the compatibility image
	tilesPattern := filepath.Join(tempDir, "output_%d.png")

	extractCmd := exec.Command(
		s.ffmpegPath,
		"-i", heicPath,
		"-map", "0", // Map all streams
		"-y",
		tilesPattern,
	)

	var extractOut bytes.Buffer
	var extractErr bytes.Buffer
	extractCmd.Stdout = &extractOut
	extractCmd.Stderr = &extractErr

	fmt.Printf("🚀 IMAGE SERVICE: Running tile extraction: %s %s\n", s.ffmpegPath, strings.Join(extractCmd.Args[1:], " "))

	if err = extractCmd.Run(); err != nil {
		fmt.Printf("❌ IMAGE SERVICE: Tile extraction failed: %v\n", err)
		fmt.Printf("❌ IMAGE SERVICE: stderr: %s\n", extractErr.String())
		return nil, fmt.Errorf("tile extraction failed: %w", err)
	}

	fmt.Printf("✅ IMAGE SERVICE: Tiles extracted successfully\n")

	// Step 3: Filter tiles (skip output_0.png which is compatibility image)
	fmt.Printf("🔧 IMAGE SERVICE: Step 3 - Filtering tiles\n")

	files, err := filepath.Glob(filepath.Join(tempDir, "output_*.png"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no tiles extracted, found %d files", len(files))
	}

	fmt.Printf("📊 IMAGE SERVICE: Found %d extracted files\n", len(files))

	// Create filtered tiles, skipping output_0.png (compatibility image)
	// and rename them to sequential tile_001.png, tile_002.png, etc.
	tileIndex := 1
	expectedTiles := gridCols * gridRows

	for i := 1; i <= len(files); i++ { // Start from 1 to skip output_0.png
		sourceFile := filepath.Join(tempDir, fmt.Sprintf("output_%d.png", i))
		var info os.FileInfo
		if info, err = os.Stat(sourceFile); err == nil && info.Size() > 500 {
			// This is a valid tile, rename it to sequential format
			targetFile := filepath.Join(tempDir, fmt.Sprintf("tile_%03d.png", tileIndex))
			if err = os.Rename(sourceFile, targetFile); err != nil {
				fmt.Printf("⚠️ IMAGE SERVICE: Failed to rename %s: %v\n", filepath.Base(sourceFile), err)
				continue
			}
			fmt.Printf("✅ IMAGE SERVICE: Valid tile %d: %s (%d bytes)\n", tileIndex, filepath.Base(targetFile), info.Size())
			tileIndex++

			// Stop if we have enough tiles
			if tileIndex > expectedTiles {
				break
			}
		} else {
			fmt.Printf("⚠️ IMAGE SERVICE: Skipping invalid tile: output_%d.png\n", i)
		}
	}

	actualTiles := tileIndex - 1
	fmt.Printf("📊 IMAGE SERVICE: Using %d valid tiles (expected %d) with grid %dx%d\n", actualTiles, expectedTiles, gridCols, gridRows)

	// Step 4: Merge tiles back into final image with proper quality and rotation
	outputFile := filepath.Join(tempDir, "final_output.jpg")

	// Build the video filter chain
	inputPattern := filepath.Join(tempDir, "tile_%03d.png")

	// Create filter chain: tile assembly + auto-orientation + proper scaling
	var filterChain string

	// For original size (targetWidth=0, targetHeight=0), don't scale
	if targetWidth == 0 && targetHeight == 0 {
		// This is original size - no scaling, maximum quality
		// Use padding=0:margin=0 to eliminate black borders between tiles
		filterChain = fmt.Sprintf("tile=%dx%d:nb_frames=%d:padding=0:margin=0%s",
			gridCols, gridRows, actualTiles, orientationFilter)
		fmt.Printf("🎯 IMAGE SERVICE: Using ORIGINAL size - no scaling, auto-orientation applied\n")
	} else {
		// This is a resize request - apply scaling with proper aspect handling
		filterChain = fmt.Sprintf("tile=%dx%d:nb_frames=%d:padding=0:margin=0%s,scale=%d:%d:force_original_aspect_ratio=decrease:eval=frame",
			gridCols, gridRows, actualTiles, orientationFilter, targetWidth, targetHeight)
		fmt.Printf("🔽 IMAGE SERVICE: Scaling to %dx%d with auto-orientation applied\n", targetWidth, targetHeight)
	}

	fmt.Printf("🎨 IMAGE SERVICE: Using filter chain: %s\n", filterChain)

	mergeCmd := exec.Command(
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

	fmt.Printf("🚀 IMAGE SERVICE: Running tile merge: %s %s\n", s.ffmpegPath, strings.Join(mergeCmd.Args[1:], " "))

	if err = mergeCmd.Run(); err != nil {
		fmt.Printf("❌ IMAGE SERVICE: Tile merge failed: %v\n", err)
		fmt.Printf("❌ IMAGE SERVICE: stderr: %s\n", mergeErr.String())
		return nil, fmt.Errorf("tile merge failed: %w", err)
	}

	fmt.Printf("✅ IMAGE SERVICE: Tiles merged successfully\n")

	fmt.Printf("📖 IMAGE SERVICE: Reading final merged JPEG file: %s\n", outputFile)
	jpegBytes, err := os.ReadFile(outputFile)
	if err != nil {
		fmt.Printf("❌ IMAGE SERVICE: Failed to read merged JPEG: %v\n", err)
		return nil, fmt.Errorf("failed to read merged JPEG: %w", err)
	}

	fmt.Printf("✅ IMAGE SERVICE: Successfully read %d bytes from merged JPEG\n", len(jpegBytes))
	return jpegBytes, nil
}

// ConvertImageToFormat converts any supported image format to another format
func (s *ImageService) ConvertImageToFormat(inputPath, outputFormat string, targetWidth, targetHeight int, quality string) ([]byte, error) {
	// Create temporary output file
	outputDir := filepath.Dir(inputPath)
	ext := ".jpg"
	if outputFormat == "png" {
		ext = ".png"
	}
	outputFile := filepath.Join(outputDir, fmt.Sprintf("converted_%d_%d%s", targetWidth, targetHeight, ext))
	defer os.Remove(outputFile) // Clean up

	// Build FFmpeg command
	args := []string{
		"-i", inputPath,
	}

	// Add scaling and auto-orientation
	if targetWidth > 0 && targetHeight > 0 {
		scaleFilter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,auto-orient=1", targetWidth, targetHeight)
		args = append(args, "-vf", scaleFilter)
	} else {
		args = append(args, "-vf", "auto-orient=1")
	}

	// Add quality settings
	if quality != "" {
		args = append(args, "-q:v", quality)
	}

	// Add output file and overwrite flag
	args = append(args, "-y", outputFile)

	cmd := exec.Command(s.ffmpegPath, args...)

	if s.debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg image conversion failed on file '%s': %w", inputPath, err)
	}

	// Read the converted file
	convertedBytes, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read converted image: %w", err)
	}

	return convertedBytes, nil
}
