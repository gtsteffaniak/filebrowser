package indexing

// OnSourceRootFullScanComplete is invoked after a source root ("/") full scan finishes.
// Optional; registered by the analytics collector when enabled in the build.
var OnSourceRootFullScanComplete func(sourceName string)
