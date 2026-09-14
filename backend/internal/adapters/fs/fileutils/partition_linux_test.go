//go:build linux

package fileutils

import (
	"syscall"
	"testing"
)

func TestBlockSizePrefersFrsize(t *testing.T) {
	// virtiofs (Docker Desktop for Mac) reports Bsize as the 1 MiB optimal
	// transfer size while Blocks are still counted in the 4 KiB fragment unit.
	stat := &syscall.Statfs_t{
		Bsize:  1048576,
		Frsize: 4096,
		Blocks: 1465121792,
		Bavail: 1054717543,
	}

	got := blockSize(stat)
	if got != 4096 {
		t.Fatalf("blockSize = %d, want 4096 (Frsize)", got)
	}

	total := uint64(stat.Blocks) * blockSize(stat)
	const wantTotal = 1465121792 * 4096 // ~5.5 TiB, matches df
	if total != wantTotal {
		t.Fatalf("total = %d bytes, want %d (~5.5 TiB)", total, uint64(wantTotal))
	}
	// Sanity: the old Bsize-based math overstates by exactly 256x.
	if inflated := uint64(stat.Blocks) * uint64(stat.Bsize); inflated != total*256 {
		t.Fatalf("expected 256x inflation from Bsize, got %d vs %d", inflated, total)
	}
}

func TestBlockSizeFallsBackToBsize(t *testing.T) {
	// Normal filesystems report Frsize == Bsize (or leave Frsize unset on
	// exotic drivers); either way the accounting unit is recovered.
	stat := &syscall.Statfs_t{Bsize: 4096, Frsize: 4096}
	if got := blockSize(stat); got != 4096 {
		t.Fatalf("blockSize = %d, want 4096", got)
	}

	zero := &syscall.Statfs_t{Bsize: 4096, Frsize: 0}
	if got := blockSize(zero); got != 4096 {
		t.Fatalf("blockSize with Frsize=0 = %d, want 4096 (Bsize fallback)", got)
	}
}
