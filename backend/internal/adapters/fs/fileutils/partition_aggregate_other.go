//go:build !linux && !freebsd

package fileutils

// GetPartitionUsageVariants returns the aggregate and root-only capacity views
// of root. Without mount info only a single statfs of the source path is
// available, so both variants are identical.
func GetPartitionUsageVariants(root string) (aggregate, rootOnly PartitionUsage, err error) {
	u, err := singlePathPartitionUsage(root)
	if err != nil {
		return aggregate, rootOnly, err
	}
	return u, u, nil
}
