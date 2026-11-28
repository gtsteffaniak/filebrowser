//go:build windows

package indexing

import "golang.org/x/sys/windows"

// volumeNameDOS is Win32's VOLUME_NAME_DOS flag (0x0), which is not exported
// by golang.org/x/sys/windows.
const volumeNameDOS = 0x0

// resolveLinkTarget resolves symlinks and NTFS junctions (both final and
// intermediate) by opening path without FILE_FLAG_OPEN_REPARSE_POINT and
// querying the final DOS path via GetFinalPathNameByHandle.
func resolveLinkTarget(path string) (string, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	h, err := windows.CreateFile(
		ptr,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, windows.MAX_PATH)
	for {
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), volumeNameDOS)
		if err != nil {
			return "", err
		}
		if n < uint32(len(buf)) {
			return stripFinalPathPrefix(windows.UTF16ToString(buf[:n])), nil
		}
		buf = make([]uint16, n+1)
	}
}
