package quota

import "testing"

func TestUploadCommitDeltaTreatsMissingDestAsZeroPrior(t *testing.T) {
	ctx := UploadContext{
		TotalSize:        50,
		HasKnownSize:     true,
		HasPriorFileSize: true,
		PriorFileSize:    0,
	}
	if delta := uploadCommitDelta(ctx); delta != 50 {
		t.Fatalf("delta = %d, want 50", delta)
	}
}

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
