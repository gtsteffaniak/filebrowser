package http

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/files"
	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
)

func setContentDisposition(w http.ResponseWriter, r *http.Request, fileName string) {
	if r.URL.Query().Get("inline") == "true" {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		// As per RFC6266 section 4.3
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(fileName))
	}
}

// rawHandler serves the raw content of a file, multiple files, or directory in various formats.
// @Summary Get raw content of a file, multiple files, or directory
// @Description Returns the raw content of a file, multiple files, or a directory. Supports downloading files as archives in various formats.
// @Tags Resources
// @Accept json
// @Produce json
// @Param files query string true "Comma-separated list of specific files within the directory (required)"
// @Param inline query bool false "If true, sets 'Content-Disposition' to 'inline'. Otherwise, defaults to 'attachment'."
// @Param algo query string false "Compression algorithm for archiving multiple files or directories. Options: 'zip' and 'tar.gz'. Default is 'zip'."
// @Success 200 {file} file "Raw file or directory content, or archive for multiple files"
// @Failure 202 {object} map[string]string "Download permissions required"
// @Failure 400 {object} map[string]string "Invalid request path"
// @Failure 404 {object} map[string]string "File or directory not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/raw [get]
func rawHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	filePrefix := ""
	file, ok := d.raw.(files.ExtendedFileInfo)
	if ok {
		filePrefix = file.Path
	}

	encodedFiles := r.URL.Query().Get("files")
	// Decode the URL-encoded path
	files, err := url.QueryUnescape(encodedFiles)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	fileList := strings.Split(files, ",|")
	for i, f := range fileList {
		fileList[i] = filepath.Join(filePrefix, f)
	}
	return rawFilesHandler(w, r, d, fileList)
}

func addFile(path string, d *requestContext, tarWriter *tar.Writer, zipWriter *zip.Writer, flatten bool) error {
	idx := files.GetIndex("default")
	realPath, _, _ := idx.GetRealPath(d.user.Scopes["default"], path)
	info, err := os.Stat(realPath)
	if err != nil {
		return err
	}

	// Get the base name of the top-level folder or file
	baseName := filepath.Base(realPath)

	if info.IsDir() {
		// Walk through directory contents
		return filepath.Walk(realPath, func(filePath string, fileInfo os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Calculate the relative path
			relPath, err := filepath.Rel(realPath, filePath) // Use realPath directly
			if err != nil {
				return err
			}

			// Normalize for tar: convert \ to /
			relPath = filepath.ToSlash(relPath)

			// Skip adding `.` (current directory)
			if relPath == "." {
				return nil
			}

			// Prepend base folder name unless flatten is true
			if !flatten {
				relPath = filepath.Join(baseName, relPath)
				relPath = filepath.ToSlash(relPath) // Ensure normalized separators
			}

			if fileInfo.IsDir() {
				if tarWriter != nil {
					header := &tar.Header{
						Name:     relPath + "/",
						Mode:     0755,
						Typeflag: tar.TypeDir,
						ModTime:  fileInfo.ModTime(),
					}
					return tarWriter.WriteHeader(header)
				}
				if zipWriter != nil {
					_, err := zipWriter.Create(relPath + "/")
					return err
				}
				return nil
			}
			return addSingleFile(filePath, relPath, zipWriter, tarWriter)
		})
	}

	// For a single file, use the base name as the archive path
	return addSingleFile(realPath, baseName, zipWriter, tarWriter)
}

func addSingleFile(realPath, archivePath string, zipWriter *zip.Writer, tarWriter *tar.Writer) error {
	file, err := os.Open(realPath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := os.Stat(realPath)
	if err != nil {
		return err
	}

	if tarWriter != nil {
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		// Ensure correct relative path
		header.Name = filepath.ToSlash(archivePath)
		if err = tarWriter.WriteHeader(header); err != nil {
			return err
		}
		_, err = io.Copy(tarWriter, file)
		return err
	}

	if zipWriter != nil {
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = archivePath
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, file)
		return err
	}

	return nil
}

func rawFilesHandler(w http.ResponseWriter, r *http.Request, d *requestContext, fileList []string) (int, error) {
	filePath := fileList[0]
	fileName := filepath.Base(filePath)
	source := r.URL.Query().Get("source")
	if source == "" {
		source = settings.Config.Server.DefaultSource.Name
	}
	idx := files.GetIndex(source)
	realPath, isDir, err := idx.GetRealPath(d.user.Scopes[source], filePath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if len(fileList) == 1 && !isDir {
		fd, err2 := os.Open(realPath)
		if err2 != nil {
			return http.StatusInternalServerError, err
		}
		defer fd.Close()

		// Get file information
		fileInfo, err3 := fd.Stat()
		if err3 != nil {
			return http.StatusInternalServerError, err
		}

		// Set headers and serve the file
		setContentDisposition(w, r, fileName)
		w.Header().Set("Cache-Control", "private")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Serve the content
		http.ServeContent(w, r, fileName, fileInfo.ModTime(), fd)
		return 0, nil
	}

	algo := r.URL.Query().Get("algo")
	var extension string
	switch algo {
	case "zip", "true", "":
		extension = ".zip"
	case "tar.gz":
		extension = ".tar.gz"
	default:
		return http.StatusInternalServerError, errors.New("format not implemented")
	}
	baseDirName := filepath.Base(filepath.Dir(realPath))
	if baseDirName == "" || baseDirName == "/" {
		baseDirName = "download"
	}
	if len(fileList) == 1 && isDir {
		baseDirName = filepath.Base(realPath)
	}
	downloadFileName := url.PathEscape(baseDirName + extension)
	w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+downloadFileName)
	// Create the archive and stream it directly to the response
	if extension == ".zip" {
		err = createZip(w, d, fileList...)
	} else {
		err = createTarGz(w, d, fileList...)
	}

	if err != nil {
		return http.StatusInternalServerError, err
	}

	return 0, nil
}

func createZip(w io.Writer, d *requestContext, filenames ...string) error {
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Check if we have exactly one directory
	//flatten := len(filenames) == 1
	for _, fname := range filenames {
		err := addFile(fname, d, nil, zipWriter, false)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to add %s to ZIP: %v", fname, err))
		}
	}

	return nil
}

func createTarGz(w io.Writer, d *requestContext, filenames ...string) error {
	gzWriter := gzip.NewWriter(w)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Check if we have exactly one directory
	//flatten := len(filenames) == 1
	for _, fname := range filenames {
		err := addFile(fname, d, tarWriter, nil, false)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to add %s to TAR.GZ: %v", fname, err))
		}
	}

	return nil
}
