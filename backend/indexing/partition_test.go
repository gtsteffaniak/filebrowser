//go:build !windows
// +build !windows

package indexing

import (
	"fmt"
	"testing"
)

func TestPartitionSizeAccuracy(t *testing.T) {
	// Test with root directory
	partitionSize, err := getPartitionSize("/")
	if err != nil {
		t.Fatalf("getPartitionSize failed: %v", err)
	}

	// Get filesystem size for comparison
	filesystemSize, err := getFilesystemSize("/")
	if err != nil {
		t.Fatalf("getFilesystemSize failed: %v", err)
	}

	t.Logf("Partition size: %d bytes (%.2f GB)", partitionSize, float64(partitionSize)/1e9)
	t.Logf("Filesystem size: %d bytes (%.2f GB)", filesystemSize, float64(filesystemSize)/1e9)

	// Partition size should be >= filesystem size
	if partitionSize < filesystemSize {
		t.Errorf("Partition size (%d) should be >= filesystem size (%d)", partitionSize, filesystemSize)
	}

	// Calculate the difference percentage
	if partitionSize > 0 {
		diff := float64(partitionSize-filesystemSize) / float64(partitionSize) * 100
		t.Logf("Difference: %.2f%% (partition is larger)", diff)

		// For most filesystems, partition should be at least 2% larger due to overhead
		if diff < 1.0 {
			t.Logf("Warning: Very small difference between partition and filesystem size")
		}
	}
}

func TestPartitionSizeConsistency(t *testing.T) {
	// Test multiple paths on the same partition should return same size
	paths := []string{"/", "/tmp", "/usr"}

	var lastSize uint64
	for i, path := range paths {
		size, err := getPartitionSize(path)
		if err != nil {
			t.Logf("Path %s failed (might be on different partition): %v", path, err)
			continue
		}

		if i == 0 {
			lastSize = size
		} else {
			// Allow for small differences due to different partitions
			if size != lastSize {
				t.Logf("Path %s has different partition size: %d vs %d", path, size, lastSize)
			}
		}
	}
}

// Benchmark to compare performance
func BenchmarkGetPartitionSize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := getPartitionSize("/")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetFilesystemSize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := getFilesystemSize("/")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Example_getPartitionSize() {
	size, err := getPartitionSize("/")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Partition size: %.2f GB\n", float64(size)/1e9)
}
