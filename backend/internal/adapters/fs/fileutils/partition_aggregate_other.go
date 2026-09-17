//go:build !linux

package fileutils

// GetAggregatedPartitionUsage returns total and used bytes for the filesystem
// that owns root. Non-Linux builds use a single statfs/GetDiskFreeSpaceEx path.
func GetAggregatedPartitionUsage(root string) (total, used uint64, err error) {
	total, err = GetPartitionSize(root)
	if err != nil {
		return 0, 0, err
	}
	used, err = GetPartitionUsed(root)
	if err != nil {
		return 0, 0, err
	}
	return total, used, nil
}
