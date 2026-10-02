package fileutils

// PartitionUsage is a capacity view of a source path: total and used bytes.
type PartitionUsage struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

// singlePathPartitionUsage statfs's a single path: the filesystem covering it.
func singlePathPartitionUsage(path string) (PartitionUsage, error) {
	total, err := GetPartitionSize(path)
	if err != nil {
		return PartitionUsage{}, err
	}
	used, err := GetPartitionUsed(path)
	if err != nil {
		return PartitionUsage{}, err
	}
	return PartitionUsage{Total: total, Used: used}, nil
}

// GetAggregatedPartitionUsage returns total and used bytes summed across distinct
// capacity groups mounted at or under root. See GetPartitionUsageVariants.
func GetAggregatedPartitionUsage(root string) (total, used uint64, err error) {
	aggregate, _, err := GetPartitionUsageVariants(root)
	if err != nil {
		return 0, 0, err
	}
	return aggregate.Total, aggregate.Used, nil
}
