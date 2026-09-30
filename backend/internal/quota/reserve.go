package quota

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
)

// mapReserveError converts reserve failures to quota errors.
func mapReserveError(err error, checks []state.QuotaReserveCheck) error {
	if err == nil {
		return nil
	}
	for _, chk := range checks {
		if chk.LimitBytes <= 0 || chk.DeltaBytes <= 0 {
			continue
		}
		reserved := state.SessionReservedForQuota(chk.QuotaID)
		if chk.UsedBytes+reserved+chk.DeltaBytes > chk.LimitBytes {
			return newError(CodeExceeded, chk.Kind, chk.QuotaID, chk.LimitBytes, chk.UsedBytes, reserved, "")
		}
	}
	for _, chk := range checks {
		if chk.LimitBytes > 0 {
			reserved := state.SessionReservedForQuota(chk.QuotaID)
			return newError(CodeExceeded, chk.Kind, chk.QuotaID, chk.LimitBytes, chk.UsedBytes, reserved, "")
		}
	}
	return newError(CodeExceeded, "", "", 0, 0, 0, "")
}
