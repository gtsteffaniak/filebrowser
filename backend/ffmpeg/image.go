package ffmpeg

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ImageService handles image operations with ffmpeg
type ImageService struct {
	ffmpegPath  string
	ffprobePath string
	debug       bool
	cacheDir    string
}

// NewImageService creates a new image service instance
func NewImageService(ffmpegPath, ffprobePath string, debug bool, cacheDir string) *ImageService {
	return &ImageService{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		debug:       debug,
		cacheDir:    cacheDir,
	}
}

// GetImageOrientation extracts the EXIF orientation from an image file using exiftool
func (s *ImageService) GetImageOrientation(imagePath string) (string, error) {
	fmt.Printf("🧭 IMAGE SERVICE: Getting orientation for %s\n", filepath.Base(imagePath))

	// Use exiftool to get orientation information
	cmd := exec.Command("exiftool", "-Orientation", "-s3", imagePath)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️ IMAGE SERVICE: exiftool failed, assuming normal orientation: %v\n", err)
		return "Horizontal (normal)", nil // Default to normal orientation
	}

	orientation := strings.TrimSpace(out.String())
	if orientation == "" {
		orientation = "Horizontal (normal)" // Default if no orientation found
	}

	fmt.Printf("🧭 IMAGE SERVICE: Found orientation: '%s'\n", orientation)
	return orientation, nil
}

// GetOrientationFilter converts EXIF orientation to FFmpeg filter
func (s *ImageService) GetOrientationFilter(orientation string) string {
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
		fmt.Printf("⚠️ IMAGE SERVICE: Unknown orientation '%s', no rotation applied\n", orientation)
		return "" // Default to no rotation for unknown orientations
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

// ConvertHEICToJPEGDirect converts a HEIC file to JPEG using direct FFmpeg conversion (fast method)
func (s *ImageService) ConvertHEICToJPEGDirect(heicPath string, targetWidth, targetHeight int, quality string) ([]byte, error) {
	overallStart := time.Now()
	fmt.Printf("🚀 IMAGE SERVICE: Direct HEIC conversion %s to JPEG (%dx%d, quality %s)\n", filepath.Base(heicPath), targetWidth, targetHeight, quality)

	// Get EXIF orientation and create appropriate filter
	fmt.Printf("🔄 IMAGE SERVICE: Detecting EXIF orientation for direct conversion\n")
	orientation, err := s.GetImageOrientation(heicPath)
	if err != nil {
		fmt.Printf("⚠️ IMAGE SERVICE: Failed to get orientation, using default: %v\n", err)
		orientation = "Horizontal (normal)"
	}
	orientationFilter := s.GetOrientationFilter(orientation)
	if orientationFilter != "" {
		fmt.Printf("🔧 IMAGE SERVICE: Applying orientation filter: %s\n", orientationFilter)
	} else {
		fmt.Printf("✅ IMAGE SERVICE: No orientation correction needed\n")
	}

	// Create temporary output file
	setupStart := time.Now()
	outputDir := s.cacheDir
	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	outputFile := filepath.Join(outputDir, fmt.Sprintf("heic_direct_%d.jpg", os.Getpid()))
	defer func() {
		cleanupStart := time.Now()
		fmt.Printf("🧹 IMAGE SERVICE: Cleaning up output file: %s\n", outputFile)
		os.Remove(outputFile)
		fmt.Printf("⏱️  IMAGE SERVICE: Cleanup completed in %v\n", time.Since(cleanupStart))
	}()
	fmt.Printf("⏱️  IMAGE SERVICE: Setup completed in %v\n", time.Since(setupStart))

	// Build FFmpeg command for direct conversion
	conversionStart := time.Now()
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

	cmd := exec.Command(s.ffmpegPath, args...)
	var cmdOut bytes.Buffer
	var cmdErr bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdErr

	fmt.Printf("🚀 IMAGE SERVICE: Running direct conversion: %s %s\n", s.ffmpegPath, strings.Join(args, " "))

	if err = cmd.Run(); err != nil {
		fmt.Printf("❌ IMAGE SERVICE: Direct conversion failed: %v\n", err)
		fmt.Printf("❌ IMAGE SERVICE: stderr: %s\n", cmdErr.String())
		return nil, fmt.Errorf("direct HEIC conversion failed: %w", err)
	}

	fmt.Printf("✅ IMAGE SERVICE: Direct conversion successful\n")
	fmt.Printf("⏱️  IMAGE SERVICE: Conversion completed in %v\n", time.Since(conversionStart))

	// Read the converted file
	readStart := time.Now()
	jpegBytes, err := os.ReadFile(outputFile)
	if err != nil {
		fmt.Printf("❌ IMAGE SERVICE: Failed to read converted JPEG: %v\n", err)
		return nil, fmt.Errorf("failed to read converted JPEG: %w", err)
	}

	fmt.Printf("✅ IMAGE SERVICE: Successfully read %d bytes from converted JPEG\n", len(jpegBytes))
	fmt.Printf("⏱️  IMAGE SERVICE: File read completed in %v\n", time.Since(readStart))
	fmt.Printf("🎉 IMAGE SERVICE: TOTAL DIRECT CONVERSION TIME: %v\n", time.Since(overallStart))
	return jpegBytes, nil
}

// ConvertHEICToJPEG converts a HEIC file to JPEG with specified dimensions and quality using proper tile extraction
func (s *ImageService) ConvertHEICToJPEG(heicPath string, targetWidth, targetHeight int, quality string) ([]byte, error) {
	overallStart := time.Now()
	fmt.Printf("🎯 IMAGE SERVICE: Converting HEIC %s to JPEG (%dx%d, quality %s) using OPTIMIZED tile extraction\n", filepath.Base(heicPath), targetWidth, targetHeight, quality)
	fmt.Printf("🚀 IMAGE SERVICE: Optimizations: JPEG intermediate tiles, reduced grid parsing time\n")

	// Create temporary directory for tile processing
	setupStart := time.Now()
	outputDir := s.cacheDir
	tempDir := filepath.Join(outputDir, fmt.Sprintf("heic_tiles_%d", os.Getpid()))
	err := os.MkdirAll(tempDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	fmt.Printf("⏱️  IMAGE SERVICE: Setup completed in %v\n", time.Since(setupStart))

	defer func() {
		cleanupStart := time.Now()
		fmt.Printf("🧹 IMAGE SERVICE: Cleaning up temporary directory: %s\n", tempDir)
		os.RemoveAll(tempDir)
		fmt.Printf("⏱️  IMAGE SERVICE: Cleanup completed in %v\n", time.Since(cleanupStart))
	}()

	// Step 1: Get grid info using trace output
	gridStart := time.Now()
	fmt.Printf("📐 IMAGE SERVICE: Step 1 - Getting grid info from HEIC trace\n")

	// Use ffprobe with trace level to get grid information (with time limit for speed)
	gridCmd := exec.Command(s.ffmpegPath, "-loglevel", "trace", "-i", heicPath, "-f", "null", "-", "-t", "0.1")
	var gridOut bytes.Buffer
	var gridErr bytes.Buffer
	gridCmd.Stdout = &gridOut
	gridCmd.Stderr = &gridErr
	_ = gridCmd.Run() // Don't check error, we're just extracting info

	traceOutput := gridErr.String()
	fmt.Printf("📊 IMAGE SERVICE: Got grid info: %d bytes\n", len(traceOutput))

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
	fmt.Printf("⏱️  IMAGE SERVICE: Grid parsing completed in %v\n", time.Since(gridStart))

	// Get EXIF orientation and create appropriate filter
	fmt.Printf("🔄 IMAGE SERVICE: Detecting and applying EXIF orientation\n")
	orientation, err := s.GetImageOrientation(heicPath)
	if err != nil {
		fmt.Printf("⚠️ IMAGE SERVICE: Failed to get orientation, using default: %v\n", err)
		orientation = "Horizontal (normal)"
	}
	orientationFilter := s.GetOrientationFilter(orientation)
	if orientationFilter != "" {
		fmt.Printf("🔧 IMAGE SERVICE: Applying orientation filter: %s\n", orientationFilter)
	} else {
		fmt.Printf("✅ IMAGE SERVICE: No orientation correction needed\n")
	}

	// Step 2: Extract tile streams (skip stream 0 which is compatibility image)
	extractStart := time.Now()
	fmt.Printf("🔧 IMAGE SERVICE: Step 2 - Extracting tile streams\n")

	// Extract using JPEG for faster I/O (PNG was unnecessarily slow and large)
	// Skip the first output (stream 0) since it's the compatibility image
	tilesPattern := filepath.Join(tempDir, "output_%d.jpg")

	extractCmd := exec.Command(
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

	fmt.Printf("🚀 IMAGE SERVICE: Running tile extraction: %s %s\n", s.ffmpegPath, strings.Join(extractCmd.Args[1:], " "))

	if err = extractCmd.Run(); err != nil {
		fmt.Printf("❌ IMAGE SERVICE: Tile extraction failed: %v\n", err)
		fmt.Printf("❌ IMAGE SERVICE: stderr: %s\n", extractErr.String())
		return nil, fmt.Errorf("tile extraction failed: %w", err)
	}

	fmt.Printf("✅ IMAGE SERVICE: Tiles extracted successfully\n")
	fmt.Printf("⏱️  IMAGE SERVICE: Tile extraction completed in %v\n", time.Since(extractStart))

	// Step 3: Filter tiles (skip output_0.png which is compatibility image)
	filterStart := time.Now()
	fmt.Printf("🔧 IMAGE SERVICE: Step 3 - Filtering tiles\n")

	files, err := filepath.Glob(filepath.Join(tempDir, "output_*.jpg"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no tiles extracted, found %d files", len(files))
	}

	fmt.Printf("📊 IMAGE SERVICE: Found %d extracted files\n", len(files))

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
			fmt.Printf("⚠️ IMAGE SERVICE: Skipping invalid tile: output_%d.jpg\n", i)
		}
	}

	actualTiles := tileIndex - 1
	fmt.Printf("📊 IMAGE SERVICE: Using %d valid tiles (expected %d) with grid %dx%d\n", actualTiles, expectedTiles, gridCols, gridRows)
	fmt.Printf("⏱️  IMAGE SERVICE: Tile filtering completed in %v\n", time.Since(filterStart))

	// Step 4: Merge tiles back into final image with proper quality and rotation
	mergeStart := time.Now()
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
		fmt.Printf("🎯 IMAGE SERVICE: Using ORIGINAL size - no scaling, natural orientation\n")
	} else {
		// This is a resize request - apply scaling with proper aspect handling
		filterChain = fmt.Sprintf("tile=%dx%d:nb_frames=%d:padding=0:margin=0%s,scale=%d:%d:force_original_aspect_ratio=decrease:eval=frame",
			gridCols, gridRows, actualTiles, orientationFilter, targetWidth, targetHeight)
		fmt.Printf("🔽 IMAGE SERVICE: Scaling to %dx%d with natural orientation\n", targetWidth, targetHeight)
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
	fmt.Printf("⏱️  IMAGE SERVICE: Tile merging completed in %v\n", time.Since(mergeStart))

	// Step 5: Read final output
	readStart := time.Now()
	fmt.Printf("📖 IMAGE SERVICE: Reading final merged JPEG file: %s\n", outputFile)
	jpegBytes, err := os.ReadFile(outputFile)
	if err != nil {
		fmt.Printf("❌ IMAGE SERVICE: Failed to read merged JPEG: %v\n", err)
		return nil, fmt.Errorf("failed to read merged JPEG: %w", err)
	}

	fmt.Printf("✅ IMAGE SERVICE: Successfully read %d bytes from merged JPEG\n", len(jpegBytes))
	fmt.Printf("⏱️  IMAGE SERVICE: File read completed in %v\n", time.Since(readStart))
	fmt.Printf("🎉 IMAGE SERVICE: TOTAL CONVERSION TIME: %v\n", time.Since(overallStart))
	return jpegBytes, nil
}
