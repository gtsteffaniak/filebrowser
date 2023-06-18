package rifs

import (
	"io"
	"os"
	"path"

	"github.com/dsoprea/go-logging"
)

// FileListFilterPredicate is the callback predicate used for filtering.
type FileListFilterPredicate func(parent string, child os.FileInfo) (hit bool, err error)

// VisitedFile is one visited file.
type VisitedFile struct {
	Filepath string
	Info     os.FileInfo
	Index    int
}

// ListFiles feeds a continuous list of files from a recursive folder scan. An
// optional predicate can be provided in order to filter. When done, the
// `filesC` channel is closed. If there's an error, the `errC` channel will
// receive it.
func ListFiles(rootPath string, cb FileListFilterPredicate) (filesC chan VisitedFile, count int, errC chan error) {
	defer func() {
		if state := recover(); state != nil {
			err := log.Wrap(state.(error))
			log.Panic(err)
		}
	}()

	// Make sure the path exists.

	f, err := os.Open(rootPath)
	log.PanicIf(err)

	f.Close()

	// Do our thing.

	filesC = make(chan VisitedFile, 100)
	errC = make(chan error, 1)
	index := 0

	go func() {
		defer func() {
			if state := recover(); state != nil {
				err := log.Wrap(state.(error))
				errC <- err
			}
		}()

		queue := []string{rootPath}
		for len(queue) > 0 {
			// Pop the next folder to process off the queue.
			var thisPath string
			thisPath, queue = queue[0], queue[1:]

			// Skip path if a symlink.

			fi, err := os.Lstat(thisPath)
			log.PanicIf(err)

			if (fi.Mode() & os.ModeSymlink) > 0 {
				continue
			}

			// Read information.

			folderF, err := os.Open(thisPath)
			if err != nil {
				errC <- log.Wrap(err)
				return
			}

			// Iterate through children.

			for {
				children, err := folderF.Readdir(1000)
				if err == io.EOF {
					break
				} else if err != nil {
					errC <- log.Wrap(err)
					return
				}

				for _, child := range children {
					filepath := path.Join(thisPath, child.Name())

					// Skip if a file symlink.

					fi, err := os.Lstat(filepath)
					log.PanicIf(err)

					if (fi.Mode() & os.ModeSymlink) > 0 {
						continue
					}

					// If a predicate was given, determine if this child will be
					// left behind.
					if cb != nil {
						hit, err := cb(thisPath, child)

						if err != nil {
							errC <- log.Wrap(err)
							return
						}

						if hit == false {
							continue
						}
					}

					index++

					// Push file to channel.

					vf := VisitedFile{
						Filepath: filepath,
						Info:     child,
						Index:    index,
					}

					filesC <- vf

					// If a folder, queue for later processing.

					if child.IsDir() == true {
						queue = append(queue, filepath)
					}
				}
			}

			folderF.Close()
		}

		close(filesC)
		close(errC)
	}()

	return filesC, index, errC
}
