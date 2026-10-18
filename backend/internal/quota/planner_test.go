package quota

import "testing"

func TestUploadCommitDeltaUsesPriorFileSize(t *testing.T) {
	ctx := UploadContext{
		TotalSize:        100,
		HasKnownSize:     true,
		HasPriorFileSize: true,
		PriorFileSize:    40,
	}
	if delta := uploadCommitDelta(ctx); delta != 60 {
		t.Fatalf("delta = %d, want 60", delta)
	}
}
