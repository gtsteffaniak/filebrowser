package files

// GetFileMetadata retrieves the FileInfo from the specified directory in the index.
func (si *Index) GetFileMetadata(adjustedPath string) (FileInfo, bool) {
	si.mu.RLock()
	dir, exists := si.Directories[adjustedPath]
	si.mu.RUnlock()
	if exists {
		// Initialize the Metadata map if it is nil
		if dir.Metadata == nil {
			dir.Metadata = make(map[string]FileInfo)
			si.SetDirectoryInfo(adjustedPath, dir)
			return FileInfo{}, false
		} else {
			return dir.Metadata[adjustedPath], true
		}
	}
	return FileInfo{}, false
}

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
func (si *Index) UpdateFileMetadata(adjustedPath string, info FileInfo) bool {
	si.mu.RLock()
	dir, exists := si.Directories[adjustedPath]
	si.mu.RUnlock()
	if exists {
		// Initialize the Metadata map if it is nil
		if dir.Metadata == nil {
			dir.Metadata = make(map[string]FileInfo)
		}
		// Release the read lock before calling SetFileMetadata
		return si.SetFileMetadata(adjustedPath, info)
	}
	return false
}

// SetFileMetadata sets the FileInfo for the specified directory in the index.
func (si *Index) SetFileMetadata(adjustedPath string, info FileInfo) bool {
	si.mu.Lock()
	defer si.mu.Unlock()
	_, exists := si.Directories[adjustedPath]
	if !exists {
		return false
	}
	si.Directories[adjustedPath].Metadata[adjustedPath] = info
	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified directory in the index.
func (si *Index) GetMetadataInfo(adjustedPath string) (FileInfo, bool) {
	si.mu.RLock()
	dir, exists := si.Directories[adjustedPath]
	si.mu.RUnlock()
	if exists {
		// Initialize the Metadata map if it is nil
		if dir.Metadata == nil {
			dir.Metadata = make(map[string]FileInfo)
			si.SetDirectoryInfo(adjustedPath, dir)
		}
		info, metadataExists := dir.Metadata[adjustedPath]
		return info, metadataExists
	}
	return FileInfo{}, false
}

// SetDirectoryInfo sets the directory information in the index.
func (si *Index) SetDirectoryInfo(adjustedPath string, dir Directory) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.Directories[adjustedPath] = dir
}
