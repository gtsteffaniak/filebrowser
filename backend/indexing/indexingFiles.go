package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
)

var (
	RealPathCache = cache.NewCache[string](48*time.Hour, 72*time.Hour)
	IsDirCache    = cache.NewCache[bool](48*time.Hour, 72*time.Hour)
)

// actionConfig holds all configuration options for indexing operations
type actionConfig struct {
	Quick         bool // whether to perform a quick scan (skip unchanged directories)
	Recursive     bool // whether to recursively index subdirectories
	CheckViewable bool // whether to check if the path has viewable:true (for API access checks)
	IsRoutineScan bool // whether this is a routine/scheduled scan (vs initial indexing)
}

// NewactionConfig creates a new actionConfig with common presets
func NewactionConfig() *actionConfig {
	return &actionConfig{
		Quick:     false,
		Recursive: true,
	}
}

// reduced index is json exposed to the client
type ReducedIndex struct {
	IdxName         string      `json:"name"`
	DiskUsed        uint64      `json:"used"`
	DiskTotal       uint64      `json:"total"`
	Status          IndexStatus `json:"status"`
	NumDirs         uint64      `json:"numDirs"`
	NumFiles        uint64      `json:"numFiles"`
	NumDeleted      uint64      `json:"numDeleted"`
	LastIndexed     time.Time   `json:"-"`
	LastIndexedUnix int64       `json:"lastIndexedUnixTime"`
	QuickScanTime   int         `json:"quickScanDurationSeconds"`
	FullScanTime    int         `json:"fullScanDurationSeconds"`
	Assessment      string      `json:"assessment"`
}
type Index struct {
	ReducedIndex
	CurrentSchedule            int `json:"-"`
	settings.Source            `json:"-"`
	Directories                map[string]*iteminfo.FileInfo `json:"-"`
	DirectoriesLedger          map[string]struct{}           `json:"-"`
	runningScannerCount        int                           `json:"-"`
	SmartModifier              time.Duration                 `json:"-"`
	FilesChangedDuringIndexing bool                          `json:"-"`
	mock                       bool
	mu                         sync.RWMutex
	wasIndexed                 bool
	FoundHardLinks             map[string]uint64   `json:"-"` // hardlink path -> size
	processedInodes            map[uint64]struct{} `json:"-"`
	totalSize                  uint64              `json:"-"`
}

var (
	indexes      map[string]*Index
	indexesMutex sync.RWMutex
)

type IndexStatus string

const (
	READY       IndexStatus = "ready"
	INDEXING    IndexStatus = "indexing"
	UNAVAILABLE IndexStatus = "unavailable"
)

// omitList contains directory names to skip during indexing
var omitList = map[string]bool{
	"$RECYCLE.BIN":              true,
	"System Volume Information": true,
	"@eaDir":                    true,
}

func init() {
	indexes = make(map[string]*Index)
}

func Initialize(source *settings.Source, mock bool) {
	indexesMutex.Lock()
	newIndex := Index{
		mock:              mock,
		Source:            *source,
		Directories:       make(map[string]*iteminfo.FileInfo),
		DirectoriesLedger: make(map[string]struct{}),
		processedInodes:   make(map[uint64]struct{}),
		FoundHardLinks:    make(map[string]uint64),
	}
	newIndex.ReducedIndex = ReducedIndex{
		Status:     "indexing",
		IdxName:    source.Name,
		Assessment: "unknown",
	}
	indexes[newIndex.Name] = &newIndex
	indexesMutex.Unlock()
	if !newIndex.Config.DisableIndexing {
		time.Sleep(time.Second)
		logger.Infof("initializing index: [%v]", newIndex.Name)
		newIndex.RunIndexing("/", false)
		go newIndex.setupIndexingScanners()
	} else {
		newIndex.Status = "ready"
		logger.Debug("indexing disabled for source: " + newIndex.Name)
	}
}

// indexDirectoryWithOptions wraps indexDirectory with actionConfig
func (idx *Index) indexDirectoryWithOptions(adjustedPath string, config *actionConfig) error {
	return idx.indexDirectory(adjustedPath, config)
}

// Define a function to recursively index files and directories
func (idx *Index) indexDirectory(adjustedPath string, config *actionConfig) error {
	// Normalize path to always have trailing slash (except for root which is just "/")
	if adjustedPath != "/" {
		adjustedPath = strings.TrimSuffix(adjustedPath, "/") + "/"
	}
	realPath := strings.TrimRight(idx.Path, "/") + adjustedPath
	// Open the directory
	dir, err := os.Open(realPath)
	if err != nil {
		// must have been deleted
		return err
	}
	defer dir.Close()

	dirInfo, err := dir.Stat()
	if err != nil {
		return err
	}

	// check if excluded from indexing
	hidden := isHidden(dirInfo, idx.Path+adjustedPath)
	if idx.shouldSkip(dirInfo.IsDir(), hidden, adjustedPath, dirInfo.Name(), config) {
		return errors.ErrNotIndexed
	}

	// if indexing, mark the directory as valid and indexed.
	if config.Recursive {
		// Prevent race conditions if scanning becomes concurrent in the future.
		idx.mu.Lock()
		idx.DirectoriesLedger[adjustedPath] = struct{}{}
		idx.mu.Unlock()
	}
	// adjustedPath is already normalized with trailing slash
	combinedPath := adjustedPath
	// get whats currently in cache
	idx.mu.RLock()
	cacheDirItems := []iteminfo.ItemInfo{}
	modChange := false
	cachedDir, exists := idx.Directories[adjustedPath]
	if exists {
		modChange = dirInfo.ModTime() != cachedDir.ModTime
		cacheDirItems = cachedDir.Folders
	}
	idx.mu.RUnlock()

	// If the directory has not been modified since the last index, skip expensive readdir
	// recursively check cached dirs for mod time changes as well
	if config.Recursive {
		if modChange {
			idx.mu.Lock()
			idx.FilesChangedDuringIndexing = true
			idx.mu.Unlock()
		} else if config.Quick {
			for _, item := range cacheDirItems {
				subConfig := &actionConfig{
					Quick:     config.Quick,
					Recursive: true,
				}
				err = idx.indexDirectory(combinedPath+item.Name, subConfig)
				if err != nil && err != errors.ErrNotIndexed {
					logger.Errorf("error indexing directory %v : %v", combinedPath+item.Name, err)
				}
			}
			return nil
		}
	}
	dirFileInfo, err2 := idx.GetDirInfo(dir, dirInfo, realPath, adjustedPath, combinedPath, config)
	if err2 != nil {
		return err2
	}
	// Update the current directory metadata in the index
	idx.UpdateMetadata(dirFileInfo)
	return nil
}

func (idx *Index) GetFsDirInfo(adjustedPath string) (*iteminfo.FileInfo, error) {
	realPath, isDir, err := idx.GetRealPath(adjustedPath)
	if err != nil {
		return nil, err
	}
	originalPath := realPath

	dir, err := os.Open(realPath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	dirInfo, err := dir.Stat()
	if err != nil {
		return nil, err
	}

	if !dirInfo.IsDir() {
		fileInfo := iteminfo.FileInfo{
			Path: adjustedPath,
			ItemInfo: iteminfo.ItemInfo{
				Name:    filepath.Base(originalPath),
				Size:    dirInfo.Size(),
				ModTime: dirInfo.ModTime(),
			},
		}
		fileInfo.DetectType(realPath, false)

		// Set HasPreview flags using consolidated helper
		setFilePreviewFlags(&fileInfo.ItemInfo, realPath)

		return &fileInfo, nil
	}

	// Normalize directory path to always have trailing slash
	if adjustedPath != "/" {
		adjustedPath = strings.TrimSuffix(adjustedPath, "/") + "/"
	}
	// adjustedPath is already normalized with trailing slash
	combinedPath := adjustedPath
	var response *iteminfo.FileInfo
	response, err = idx.GetDirInfo(dir, dirInfo, realPath, adjustedPath, combinedPath, &actionConfig{
		Quick:         false,
		Recursive:     false,
		CheckViewable: true,
	})
	if err != nil {
		return nil, err
	}
	if !isDir {
		baseName := filepath.Base(originalPath)
		idx.MakeIndexPath(realPath)
		found := false
		for _, item := range response.Files {
			if item.Name == baseName {
				// Clean path to remove trailing slashes before joining
				filePath := strings.TrimSuffix(adjustedPath, "/") + "/" + item.Name
				response = &iteminfo.FileInfo{
					Path:     filePath,
					ItemInfo: item,
				}
				found = true
				continue
			}
		}
		if !found {
			return nil, fmt.Errorf("file not found in directory: %s", adjustedPath)
		}

	}
	return response, nil

}

func (idx *Index) GetDirInfo(dirInfo *os.File, stat os.FileInfo, realPath, adjustedPath, combinedPath string, config *actionConfig) (*iteminfo.FileInfo, error) {
	// Ensure combinedPath has exactly one trailing slash to prevent double slashes in subdirectory paths
	combinedPath = strings.TrimRight(combinedPath, "/") + "/"
	// Read directory contents
	files, err := dirInfo.Readdir(-1)
	if err != nil {
		return nil, err
	}
	var totalSize int64
	fileInfos := []iteminfo.ItemInfo{}
	dirInfos := []iteminfo.ItemInfo{}
	hasPreview := false
	if !config.Recursive {
		realDirInfo, exists := idx.GetMetadataInfo(adjustedPath, true)
		if exists {
			hasPreview = realDirInfo.HasPreview
		}
	}

	// Process each file and directory in the current directory
	for _, file := range files {
		hidden := isHidden(file, idx.Path+combinedPath)
		isDir := iteminfo.IsDirectory(file)
		baseName := file.Name()
		fullCombined := combinedPath + baseName

		// Skip logic based on mode
		if config.CheckViewable {
			// When checking viewable items: skip if shouldSkip=true AND not viewable
			if idx.shouldSkip(isDir, hidden, fullCombined, baseName, config) && !idx.IsViewable(isDir, fullCombined) {
				continue
			}
		} else {
			// Normal indexing mode: skip if shouldSkip=true
			if idx.shouldSkip(isDir, hidden, fullCombined, baseName, config) {
				continue
			}
		}
		itemInfo := &iteminfo.ItemInfo{
			Name:    file.Name(),
			ModTime: file.ModTime(),
			Hidden:  hidden,
		}

		if isDir {
			dirPath := combinedPath + file.Name()
			if idx.wasIndexed && config.Recursive && len(idx.Config.NeverWatchPaths) > 0 {
				if slices.Contains(idx.Config.NeverWatchPaths, fullCombined) {
					realDirInfo, exists := idx.GetMetadataInfo(dirPath, true)
					if exists {
						itemInfo.Size = realDirInfo.Size
					}
					continue
				}
			}
			// skip non-indexable dirs.
			if omitList[file.Name()] {
				continue
			}

			if config.Recursive {
				// clear for garbage collection
				file = nil
				// Recursively index the subdirectory
				err = idx.indexDirectory(dirPath, config)
				if err != nil {
					logger.Errorf("Failed to index directory %s: %v", dirPath, err)
					continue
				}
			}
			realDirInfo, exists := idx.GetMetadataInfo(dirPath, true)
			if exists {
				itemInfo.Size = realDirInfo.Size
				itemInfo.HasPreview = realDirInfo.HasPreview
			}
			totalSize += itemInfo.Size
			itemInfo.Type = "directory"
			dirInfos = append(dirInfos, *itemInfo)
			if config.Recursive {
				idx.NumDirs++
			}
		} else {
			size, shouldCountSize := idx.handleFile(file, fullCombined)
			itemInfo.DetectType(realPath+"/"+file.Name(), false)
			// Set HasPreview flags - use cached metadata optimization only when indexing is enabled
			if !idx.Config.DisableIndexing && config.Recursive {
				// Optimization: For audio files during indexing, check if we can use cached album art info
				simpleType := strings.Split(itemInfo.Type, "/")[0]
				if simpleType == "audio" {
					previousInfo, exists := idx.GetReducedMetadata(fullCombined, false)
					if exists && time.Time.Equal(previousInfo.ModTime, file.ModTime()) {
						// File unchanged - use cached album art info
						itemInfo.HasPreview = previousInfo.HasPreview
					}
				}
			}
			// When indexing is disabled or CheckViewable mode, always check directly
			setFilePreviewFlags(itemInfo, realPath+"/"+file.Name())

			itemInfo.Size = int64(size)

			// Update parent folder preview status for images, videos, and audio with album art
			// Use shared function to determine if this file type should bubble up to folder preview
			if itemInfo.HasPreview && iteminfo.ShouldBubbleUpToFolderPreview(*itemInfo) {
				hasPreview = true
			}

			fileInfos = append(fileInfos, *itemInfo)
			if shouldCountSize {
				totalSize += itemInfo.Size
			}
			if config.Recursive {
				idx.NumFiles++
			}
		}
	}

	if totalSize == 0 && idx.Config.Conditionals.ZeroSizeFolders {
		return nil, errors.ErrNotIndexed
	}

	if adjustedPath == "/" {
		idx.mu.Lock()
		idx.DiskUsed = uint64(totalSize)
		idx.mu.Unlock()
	}

	// Create FileInfo for the current directory (adjustedPath is already normalized with trailing slash)
	dirFileInfo := &iteminfo.FileInfo{
		Path:    adjustedPath,
		Files:   fileInfos,
		Folders: dirInfos,
	}
	dirFileInfo.ItemInfo = iteminfo.ItemInfo{
		Name:       filepath.Base(dirInfo.Name()),
		Type:       "directory",
		Size:       totalSize,
		ModTime:    stat.ModTime(),
		HasPreview: hasPreview,
	}
	dirFileInfo.SortItems()

	// Metadata will be updated by the caller (indexDirectory or GetFsDirInfo)
	return dirFileInfo, nil
}

func (idx *Index) recursiveUpdateDirSizes(childInfo *iteminfo.FileInfo, previousSize int64) {
	parentDir := utils.GetParentDirectoryPath(childInfo.Path)

	parentInfo, exists := idx.GetMetadataInfo(parentDir, true)
	if !exists || parentDir == "" {
		return
	}

	// Calculate size delta and update parent
	previousParentSize := parentInfo.Size
	sizeDelta := childInfo.Size - previousSize
	parentInfo.Size = previousParentSize + sizeDelta

	idx.UpdateMetadata(parentInfo)

	// Recursively update grandparents
	idx.recursiveUpdateDirSizes(parentInfo, previousParentSize)
}

func (idx *Index) GetRealPath(relativePath ...string) (string, bool, error) {
	combined := append([]string{idx.Path}, relativePath...)
	joinedPath := filepath.Join(combined...)
	isDir, _ := IsDirCache.Get(joinedPath + ":isdir")
	cached, ok := RealPathCache.Get(joinedPath)
	if ok && cached != "" {
		return cached, isDir, nil
	}
	// Convert relative path to absolute path
	absolutePath, err := filepath.Abs(joinedPath)
	if err != nil {
		return absolutePath, false, fmt.Errorf("could not get real path: %v, %s", joinedPath, err)
	}
	// Resolve symlinks and get the real path
	realPath, isDir, err := iteminfo.ResolveSymlinks(absolutePath)
	if err == nil {
		RealPathCache.Set(joinedPath, realPath)
		IsDirCache.Set(joinedPath+":isdir", isDir)
	}
	return realPath, isDir, err
}

func (idx *Index) RefreshFileInfo(opts utils.FileOptions) error {
	config := &actionConfig{
		Quick:     false,
		Recursive: opts.Recursive,
	}

	targetPath := opts.Path
	if !opts.IsDir {
		targetPath = idx.MakeIndexPath(filepath.Dir(targetPath))
	}

	// Get PREVIOUS metadata BEFORE indexing
	previousInfo, previousExists := idx.GetMetadataInfo(targetPath, true)
	var previousSize int64
	if previousExists {
		previousSize = previousInfo.Size
	}

	// Re-index the directory
	err := idx.indexDirectoryWithOptions(targetPath, config)
	if err != nil {
		return err
	}

	// Get the NEW metadata after indexing
	newInfo, exists := idx.GetMetadataInfo(targetPath, true)
	if !exists {
		return fmt.Errorf("file/folder does not exist in metadata: %s", targetPath)
	}

	// If size changed, propagate to parents
	if previousSize != newInfo.Size {
		idx.recursiveUpdateDirSizes(newInfo, previousSize)
	}

	return nil
}

func isHidden(file os.FileInfo, srcPath string) bool {
	// Check if the file starts with a dot (common on Unix systems)
	if file.Name()[0] == '.' {
		return true
	}

	if runtime.GOOS == "windows" {
		return CheckWindowsHidden(filepath.Join(srcPath, file.Name()))
	}

	// Default behavior for non-Windows systems
	return false
}

// setFilePreviewFlags determines if a file should have a preview based on its type
// This consolidates the logic used in both GetFsDirInfo and GetDirInfo
func setFilePreviewFlags(fileInfo *iteminfo.ItemInfo, realPath string) {
	simpleType := strings.Split(fileInfo.Type, "/")[0]
	ext := strings.ToLower(filepath.Ext(fileInfo.Name))
	extWithoutPeriod := strings.TrimPrefix(ext, ".")

	// Check if it's an image
	if simpleType == "image" {
		fileInfo.HasPreview = true
	}

	// Check for HEIC/HEIF
	switch extWithoutPeriod {
	case "heic", "heif":
		if settings.CanConvertImage(extWithoutPeriod) {
			fileInfo.HasPreview = true
		}
	}

	// Check if it's a video
	if simpleType == "video" && settings.CanConvertVideo(extWithoutPeriod) {
		fileInfo.HasPreview = true
	}

	// Check for audio with album art (always check, don't rely on cache)
	if simpleType == "audio" {
		fileInfo.HasPreview = iteminfo.HasAlbumArt(realPath, ext)
	}

	// Check for office docs and PDFs
	if settings.Config.Integrations.OnlyOffice.Secret != "" && iteminfo.IsOnlyOffice(fileInfo.Name) {
		fileInfo.HasPreview = true
	}
	if iteminfo.HasDocConvertableExtension(fileInfo.Name, fileInfo.Type) {
		fileInfo.HasPreview = true
	}
}

// matchResult represents the outcome of checking conditional rules
type matchResult int

const (
	noMatch     matchResult = iota // No rule matched
	shouldIndex                    // Rule matched and should be indexed
	shouldSkip                     // Rule matched and should be skipped
)

// checkExactMatch checks if a value exists in the map and returns the appropriate action
func checkExactMatch(ruleMap map[string]settings.ConditionalIndexConfig, value string) matchResult {
	rule, exists := ruleMap[value]
	if !exists {
		return noMatch
	}
	if !rule.Index {
		return shouldSkip
	}
	return shouldIndex
}

// checkPrefixMatch checks if value starts with any rule in the slice
func checkPrefixMatch(rules []settings.ConditionalIndexConfig, value string) matchResult {
	for _, rule := range rules {
		if strings.HasPrefix(value, rule.Value) {
			if !rule.Index {
				return shouldSkip
			}
			return shouldIndex
		}
	}
	return noMatch
}

// checkSuffixMatch checks if value ends with any rule in the slice
func checkSuffixMatch(rules []settings.ConditionalIndexConfig, value string) matchResult {
	for _, rule := range rules {
		if strings.HasSuffix(value, rule.Value) {
			if !rule.Index {
				return shouldSkip
			}
			return shouldIndex
		}
	}
	return noMatch
}

// IsViewable checks if a path has viewable:true (allows FS access without indexing)
func (idx *Index) IsViewable(isDir bool, adjustedPath string) bool {
	maps := idx.Config.ConditionalsMap
	rules := &idx.Config.Conditionals
	if maps == nil {
		return false
	}

	baseName := filepath.Base(strings.TrimSuffix(adjustedPath, "/"))

	if isDir {
		// Exact match (O(1))
		if rule, exists := maps.FolderNamesMap[baseName]; exists && !rule.Index && rule.Viewable {
			return true
		}
		// Prefix/suffix (O(n))
		for _, rule := range rules.FolderPaths {
			if strings.HasPrefix(adjustedPath, rule.Value) && !rule.Index && rule.Viewable {
				return true
			}
		}
		for _, rule := range rules.FolderEndsWith {
			if strings.HasSuffix(baseName, rule.Value) && !rule.Index && rule.Viewable {
				return true
			}
		}
		for _, rule := range rules.FolderStartsWith {
			if strings.HasPrefix(baseName, rule.Value) && !rule.Index && rule.Viewable {
				return true
			}
		}
	} else {
		// Exact match (O(1))
		if rule, exists := maps.FileNamesMap[baseName]; exists && !rule.Index && rule.Viewable {
			return true
		}
		// Prefix/suffix (O(n))
		for _, rule := range rules.FilePaths {
			if strings.HasPrefix(adjustedPath, rule.Value) && !rule.Index && rule.Viewable {
				return true
			}
		}
		for _, rule := range rules.FileEndsWith {
			if strings.HasSuffix(baseName, rule.Value) && !rule.Index && rule.Viewable {
				return true
			}
		}
		for _, rule := range rules.FileStartsWith {
			if strings.HasPrefix(baseName, rule.Value) && !rule.Index && rule.Viewable {
				return true
			}
		}
		// Check if parent directory is viewable (recursively check all parent rules)
		parent := filepath.Dir(adjustedPath)
		parentBaseName := filepath.Base(strings.TrimSuffix(parent, "/"))

		// Check parent against all folder rules
		if rule, exists := maps.FolderNamesMap[parentBaseName]; exists && !rule.Index && rule.Viewable {
			return true
		}
		for _, rule := range rules.FolderPaths {
			if strings.HasPrefix(parent, rule.Value) && !rule.Index && rule.Viewable {
				return true
			}
		}
		for _, rule := range rules.FolderEndsWith {
			if strings.HasSuffix(parentBaseName, rule.Value) && !rule.Index && rule.Viewable {
				return true
			}
		}
		for _, rule := range rules.FolderStartsWith {
			if strings.HasPrefix(parentBaseName, rule.Value) && !rule.Index && rule.Viewable {
				return true
			}
		}
	}
	return false
}

func (idx *Index) shouldSkip(isDir bool, isHidden bool, fullCombined, baseName string, config *actionConfig) bool {
	// When indexing is disabled globally, behavior depends on the mode
	if idx.Config.DisableIndexing {
		// If checking viewable (filesystem access), don't skip - show everything from filesystem
		if config != nil && config.CheckViewable {
			return false
		}
		// If indexing mode, skip everything
		return true
	}

	// Use optimized maps for lookups
	maps := idx.Config.ConditionalsMap
	rules := &idx.Config.Conditionals

	if maps == nil {
		// Fallback: maps not initialized (shouldn't happen in production)
		return false
	}

	if isDir && config != nil && config.IsRoutineScan {
		// Check NeverWatch: Skip directories with index=true AND neverWatch=true during routine scans
		// This allows them to be indexed once but never re-scanned
		checkNeverWatchIndexed := func(rules []settings.ConditionalIndexConfig, value string, matchFunc func(string, string) bool) bool {
			for _, rule := range rules {
				if matchFunc(rule.Value, value) && rule.Index && rule.NeverWatch {
					return true
				}
			}
			return false
		}

		exactMatch := func(a, b string) bool { return a == b }
		prefixMatch := func(a, b string) bool { return strings.HasPrefix(b, a) }
		suffixMatch := func(a, b string) bool { return strings.HasSuffix(b, a) }

		if checkNeverWatchIndexed(rules.FolderNames, baseName, exactMatch) ||
			checkNeverWatchIndexed(rules.FolderPaths, fullCombined, prefixMatch) ||
			checkNeverWatchIndexed(rules.FolderEndsWith, baseName, suffixMatch) ||
			checkNeverWatchIndexed(rules.FolderStartsWith, baseName, prefixMatch) {
			return true // Skip this directory during routine scans
		}
	}

	if isDir {

		// Check FolderNames (exact match on base name) - O(1) lookup
		if len(maps.FolderNamesMap) > 0 {
			if result := checkExactMatch(maps.FolderNamesMap, baseName); result == shouldSkip {
				return true
			}
		}

		// Check FolderPaths (prefix match on full path) - use original slice
		if len(rules.FolderPaths) > 0 {
			if result := checkPrefixMatch(rules.FolderPaths, fullCombined); result == shouldSkip {
				return true
			}
		}

		// Check FolderEndsWith (suffix match on base name) - use original slice
		if len(rules.FolderEndsWith) > 0 {
			if result := checkSuffixMatch(rules.FolderEndsWith, baseName); result == shouldSkip {
				return true
			}
		}

		// Check FolderStartsWith (prefix match on base name) - use original slice
		if len(rules.FolderStartsWith) > 0 {
			if result := checkPrefixMatch(rules.FolderStartsWith, baseName); result == shouldSkip {
				return true
			}
		}
	} else {
		// Check FileNames (exact match on base name) - O(1) lookup
		if len(maps.FileNamesMap) > 0 {
			if result := checkExactMatch(maps.FileNamesMap, baseName); result == shouldSkip {
				return true
			}
		}

		// Check FilePaths (prefix match on full path) - use original slice
		if len(rules.FilePaths) > 0 {
			if result := checkPrefixMatch(rules.FilePaths, fullCombined); result == shouldSkip {
				return true
			}
		}

		// Check FileEndsWith (suffix match on base name) - use original slice
		if len(rules.FileEndsWith) > 0 {
			if result := checkSuffixMatch(rules.FileEndsWith, baseName); result == shouldSkip {
				return true
			}
		}

		// Check FileStartsWith (prefix match on base name) - use original slice
		if len(rules.FileStartsWith) > 0 {
			if result := checkPrefixMatch(rules.FileStartsWith, baseName); result == shouldSkip {
				return true
			}
		}

		// Exclude if parent directory matches FolderPaths - use original slice
		if len(rules.FolderPaths) > 0 {
			parent := filepath.Dir(fullCombined)
			if result := checkPrefixMatch(rules.FolderPaths, parent); result == shouldSkip {
				return true
			}
		}
	}

	if rules.Hidden && isHidden {
		return true
	}
	return false
}

type DiskUsage struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

func (idx *Index) SetUsage(totalBytes uint64) {
	if settings.Config.Frontend.DisableUsedPercentage {
		return
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.DiskTotal = totalBytes
}

func (idx *Index) SetStatus(status IndexStatus) error {
	idx.mu.Lock()
	idx.Status = status
	switch status {
	case INDEXING:
		idx.runningScannerCount++
	case READY, UNAVAILABLE:
		idx.runningScannerCount = 0
	}
	idx.mu.Unlock()
	return idx.SendSourceUpdateEvent()
}

func (idx *Index) handleFile(file os.FileInfo, fullCombined string) (size uint64, shouldCountSize bool) {
	var realSize uint64
	var nlink uint64 = 1
	var ino uint64 = 0
	canUseSyscall := false

	if sys := file.Sys(); sys != nil {
		realSize, nlink, ino, canUseSyscall = getFileDetails(sys)
	}

	if !canUseSyscall {
		// Fallback for non-unix systems or if syscall info is unavailable
		realSize = uint64(file.Size())
	}

	if nlink > 1 {
		// It's a hard link
		idx.mu.Lock()
		defer idx.mu.Unlock()
		if _, exists := idx.processedInodes[ino]; exists {
			// Already seen, don't count towards global total, or directory total.
			return realSize, false
		}
		// First time seeing this inode.
		idx.processedInodes[ino] = struct{}{}
		idx.FoundHardLinks[fullCombined] = realSize
		idx.totalSize += realSize
		return realSize, true // Count size for directory total.
	}

	// It's a regular file.
	idx.mu.Lock()
	idx.totalSize += realSize
	idx.mu.Unlock()
	return realSize, true // Count size.
}

// input should be non-index path.
func (idx *Index) MakeIndexPath(path string) string {
	if path == "." || strings.HasPrefix(path, "./") {
		path = strings.TrimPrefix(path, ".")
	}
	path = strings.TrimPrefix(path, idx.Path)
	path = idx.MakeIndexPathPlatform(path)
	path = strings.TrimSuffix(path, "/") + "/"
	return path
}
