package http

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	gopath "path"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/fileutils"
	"github.com/gtsteffaniak/filebrowser/users"
)

func slashClean(name string) string {
	if name == "" || name[0] != '/' {
		name = "/" + name
	}
	return gopath.Clean(name)
}

func parseQueryFiles(r *http.Request, f *files.FileInfo, _ *users.User) ([]string, error) {
	var fileSlice []string
	names := strings.Split(r.URL.Query().Get("files"), ",")

	if len(names) == 0 {
		fileSlice = append(fileSlice, f.Path)
	} else {
		for _, name := range names {
			name, err := url.QueryUnescape(strings.Replace(name, "+", "%2B", -1)) //nolint:govet
			if err != nil {
				return nil, err
			}

			name = slashClean(name)
			fileSlice = append(fileSlice, filepath.Join(f.Path, name))
		}
	}

	return fileSlice, nil
}

func setContentDisposition(w http.ResponseWriter, r *http.Request, file *files.FileInfo) {
	if r.URL.Query().Get("inline") == "true" {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		// As per RFC6266 section 4.3
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(file.Name))
	}
}

// rawHandler serves the raw content of a file, multiple files, or directory in various formats.
// @Summary Get raw content of a file, multiple files, or directory
// @Description Returns the raw content of a file, multiple files, or a directory. Supports downloading files as archives in various formats.
// @Tags Resources
// @Accept json
// @Produce json
// @Param path query string true "Path to the file or directory"
// @Param files query string false "Comma-separated list of specific files within the directory (optional)"
// @Param inline query bool false "If true, sets 'Content-Disposition' to 'inline'. Otherwise, defaults to 'attachment'."
// @Param algo query string false "Compression algorithm for archiving multiple files or directories. Options: 'zip', 'tar', 'targz', 'tarbz2', 'tarxz', 'tarlz4', 'tarsz'. Default is 'zip'."
// @Success 200 {file} file "Raw file or directory content, or archive for multiple files"
// @Failure 202 {object} map[string]string "Download permissions required"
// @Failure 400 {object} map[string]string "Invalid request path"
// @Failure 404 {object} map[string]string "File or directory not found"
// @Failure 415 {object} map[string]string "Unsupported file type for preview"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/raw [get]
func rawHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Perm.Download {
		return http.StatusAccepted, nil
	}
	path := r.URL.Query().Get("path")
	fileInfo, err := files.FileInfoFaster(files.FileOptions{
		Path:       filepath.Join(d.user.Scope, path),
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: config.Server.TypeDetectionByHeader,
		Checker:    d.user,
	})
	if err != nil {
		return errToStatus(err), err
	}

	// TODO, how to handle? we removed mode, is it needed?
	// maybe instead of mode we use bool only two conditions are checked
	//if files.IsNamedPipe(fileInfo.Mode) {
	//	setContentDisposition(w, r, file)
	//	return 0, nil
	//}

	if fileInfo.Type == "directory" {
		return rawDirHandler(w, r, d, fileInfo.FileInfo)
	}

	return rawFileHandler(w, r, fileInfo.FileInfo)
}

func addFile(path, commonPath string, d *requestContext, tarWriter *tar.Writer, zipWriter *zip.Writer) error {
	path, _, _ = files.GetRealPath(d.user.Scope, path)
	if !d.user.Check(path) {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !info.IsDir() && !info.Mode().IsRegular() {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	filename := strings.TrimPrefix(path, commonPath)
	filename = strings.TrimPrefix(filename, string(filepath.Separator))

	if tarWriter != nil {
		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return err
		}
		header.Name = filename
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
		header.Name = filename
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, file)
		return err
	}

	return nil
}

func rawDirHandler(w http.ResponseWriter, r *http.Request, d *requestContext, file *files.FileInfo) (int, error) {
	filenames, err := parseQueryFiles(r, file, d.user)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	algo := r.URL.Query().Get("algo")
	var extension string
	switch algo {
	case "zip", "true", "":
		extension = ".zip"
	case "tar":
		extension = ".tar"
	case "targz":
		extension = ".tar.gz"
	default:
		return http.StatusInternalServerError, errors.New("format not implemented")
	}

	// Determine common directory prefix for file paths
	commonDir := fileutils.CommonPrefix(filepath.Separator, filenames...)

	// Set filename for the archive
	name := filepath.Base(commonDir)
	if name == "." || name == "" || name == string(filepath.Separator) {
		name = file.Name
	}
	if len(filenames) > 1 {
		name = "_" + name
	}
	name += extension

	w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(name))

	// Create the archive and stream it directly to the response
	if extension == ".zip" {
		err = createZip(w, filenames, commonDir, d)
	} else if extension == ".tar.gz" {
		err = createTarGz(w, filenames, commonDir, d)
	} else {
		err = createTar(w, filenames, commonDir, d)
	}

	if err != nil {
		return http.StatusInternalServerError, err
	}

	return 0, nil
}

func rawFileHandler(w http.ResponseWriter, r *http.Request, file *files.FileInfo) (int, error) {
	realPath, _, _ := files.GetRealPath(file.Path)
	fd, err := os.Open(realPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	setContentDisposition(w, r, file)

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, fd)
	return 0, nil
}

func createZip(w io.Writer, filenames []string, commonDir string, d *requestContext) error {
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for _, fname := range filenames {
		err := addFile(fname, commonDir, d, nil, zipWriter)
		if err != nil {
			log.Printf("Failed to add %s to ZIP: %v", fname, err)
		}
	}

	return nil
}

func createTar(w io.Writer, filenames []string, commonDir string, d *requestContext) error {
	tarWriter := tar.NewWriter(w)
	defer tarWriter.Close()

	for _, fname := range filenames {
		err := addFile(fname, commonDir, d, tarWriter, nil)
		if err != nil {
			log.Printf("Failed to add %s to TAR: %v", fname, err)
		}
	}

	return nil
}

func createTarGz(w io.Writer, filenames []string, commonDir string, d *requestContext) error {
	gzWriter := gzip.NewWriter(w)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	for _, fname := range filenames {
		err := addFile(fname, commonDir, d, tarWriter, nil)
		if err != nil {
			log.Printf("Failed to add %s to TAR.GZ: %v", fname, err)
		}
	}

	return nil
}
