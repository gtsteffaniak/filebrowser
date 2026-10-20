package state

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/quota"
)

func withCleanQuotaReservations(t *testing.T) {
	oldReservations := reservationsBySession
	oldCounters := quotaCounters
	initQuotaMaps()
	t.Cleanup(func() {
		quotasMux.Lock()
		reservationsBySession = oldReservations
		quotaCounters = oldCounters
		quotasMux.Unlock()
	})
}

func TestReserveQuota_idempotentReplaceUsesNewTotalNotIncrementTwice(t *testing.T) {
	withCleanQuotaReservations(t)

	const quotaID = "folder-q1"
	check := func(delta int64) QuotaReserveCheck {
		return QuotaReserveCheck{
			QuotaID:    quotaID,
			LimitBytes: 100,
			UsedBytes:  0,
			DeltaBytes: delta,
		}
	}

	if err := ReserveQuota("upload-session", []QuotaReserveCheck{check(40)}); err != nil {
		t.Fatalf("initial reserve: %v", err)
	}
	if got := SessionReservedForQuota(quotaID); got != 40 {
		t.Fatalf("after 40: reserved=%d", got)
	}

	if err := ReserveQuota("upload-session", []QuotaReserveCheck{check(70)}); err != nil {
		t.Fatalf("replace with 70: %v", err)
	}
	if got := SessionReservedForQuota(quotaID); got != 70 {
		t.Fatalf("after 70: reserved=%d want 70", got)
	}

	if err := ReserveQuota("upload-session", []QuotaReserveCheck{check(101)}); err == nil {
		t.Fatal("expected quota exceeded when replacing with 101")
	}
	if got := SessionReservedForQuota(quotaID); got != 70 {
		t.Fatalf("after failed 101: reserved=%d want 70 unchanged", got)
	}

	if err := ReserveQuota("upload-session", []QuotaReserveCheck{check(100)}); err != nil {
		t.Fatalf("replace with 100 at limit: %v", err)
	}
	if got := SessionReservedForQuota(quotaID); got != 100 {
		t.Fatalf("after 100: reserved=%d", got)
	}
}

func TestReserveQuota_multiCheckAtomicOnFailure(t *testing.T) {
	withCleanQuotaReservations(t)

	ok := QuotaReserveCheck{QuotaID: "q-ok", LimitBytes: 1000, UsedBytes: 0, DeltaBytes: 200}
	if err := ReserveQuota("sess", []QuotaReserveCheck{ok}); err != nil {
		t.Fatalf("setup reserve: %v", err)
	}
	if got := SessionReservedForQuota("q-ok"); got != 200 {
		t.Fatalf("setup reserved=%d", got)
	}

	tight := QuotaReserveCheck{QuotaID: "q-tight", LimitBytes: 50, UsedBytes: 0, DeltaBytes: 60}
	batch := []QuotaReserveCheck{
		{QuotaID: "q-ok", LimitBytes: 1000, UsedBytes: 0, DeltaBytes: 500},
		tight,
	}
	if err := ReserveQuota("sess", batch); err == nil {
		t.Fatal("expected second quota in batch to fail")
	}
	if got := SessionReservedForQuota("q-ok"); got != 200 {
		t.Fatalf("first quota must be unchanged after failed batch, reserved=%d", got)
	}
	if got := SessionReservedForQuota("q-tight"); got != 0 {
		t.Fatalf("failed quota must not be reserved, got=%d", got)
	}
}

// Accounted meters run counter ensure before any reservation mutation; a failed batch must
// leave session reservations unchanged (two-pass ReserveQuota).
func TestReserveQuota_accountedBatchAtomicOnFailure(t *testing.T) {
	withCleanQuotaReservations(t)
	if sqlDb == nil {
		t.Skip("requires initialized SQL store for accounted quota counters")
	}

	first := QuotaReserveCheck{
		QuotaID:    "q-accounted-a",
		Kind:       "share",
		Meter:      quota.MeterAccounted,
		LimitBytes: 1000,
		UsedBytes:  0,
		DeltaBytes: 100,
	}
	if err := ReserveQuota("sess-acct", []QuotaReserveCheck{first}); err != nil {
		t.Fatalf("setup reserve: %v", err)
	}
	if got := SessionReservedForQuota("q-accounted-a"); got != 100 {
		t.Fatalf("setup reserved=%d", got)
	}

	batch := []QuotaReserveCheck{
		{QuotaID: "q-accounted-a", Kind: "share", Meter: quota.MeterAccounted, LimitBytes: 1000, UsedBytes: 0, DeltaBytes: 500},
		{QuotaID: "q-accounted-b", Kind: "share", Meter: quota.MeterAccounted, LimitBytes: 50, UsedBytes: 0, DeltaBytes: 60},
	}
	if err := ReserveQuota("sess-acct", batch); err == nil {
		t.Fatal("expected second accounted quota in batch to fail")
	}
	if got := SessionReservedForQuota("q-accounted-a"); got != 100 {
		t.Fatalf("first accounted reservation must be unchanged after failed batch, reserved=%d", got)
	}
	if got := SessionReservedForQuota("q-accounted-b"); got != 0 {
		t.Fatalf("failed accounted quota must not be reserved, got=%d", got)
	}
}
