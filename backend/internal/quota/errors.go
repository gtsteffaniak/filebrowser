package quota

import (
	"errors"
	"fmt"
)

const (
	CodeExceeded                = "quota_exceeded"
	CodeMeasurementUnavailable  = "quota_measurement_unavailable"
	CodePathNotCountable        = "quota_path_not_countable"
	CodeUsageUnknown            = "quota_usage_unknown"
	CodeReservedCapacity        = "quota_reserved_capacity"
	CodeLengthRequired          = "quota_length_required"
	CodeRootImmutable           = "quota_root_immutable"
)

// Error is returned when a quota check fails.
type Error struct {
	Code          string
	QuotaKind     string
	QuotaID       string
	LimitBytes    int64
	UsedBytes     int64
	ReservedBytes int64
	Message       string
}

func (e *Error) Error() string {
	return e.DisplayMessage()
}

// DisplayMessage returns a user-facing error string (no internal IDs).
func (e *Error) DisplayMessage() string {
	if e.Message != "" {
		return e.Message
	}
	return e.defaultMessage()
}

func (e *Error) defaultMessage() string {
	switch e.Code {
	case CodeExceeded:
		switch e.QuotaKind {
		case "share":
			return "Share storage quota exceeded"
		case "scope":
			return "Source storage quota exceeded"
		case "folder":
			return "Folder storage quota exceeded"
		}
		return "Storage quota exceeded"
	case CodeMeasurementUnavailable:
		return "Storage measurement is not available yet"
	case CodePathNotCountable:
		return "Storage for this path cannot be measured"
	case CodeUsageUnknown:
		return "Storage usage is not available for quota check"
	case CodeReservedCapacity:
		return "Storage quota reservation failed"
	case CodeLengthRequired:
		return "Upload size is required when storage quota applies"
	case CodeRootImmutable:
		return "Root storage quota cannot be changed"
	default:
		return fmt.Sprintf("quota error: %s", e.Code)
	}
}

func newError(code, kind, id string, limit, used, reserved int64, msg string) *Error {
	return &Error{
		Code:          code,
		QuotaKind:     kind,
		QuotaID:       id,
		LimitBytes:    limit,
		UsedBytes:     used,
		ReservedBytes: reserved,
		Message:       msg,
	}
}

// IsQuotaError reports whether err is a quota Error.
func IsQuotaError(err error) bool {
	var qe *Error
	return errors.As(err, &qe)
}

// AsError unwraps a quota Error.
func AsError(err error) (*Error, bool) {
	var qe *Error
	if errors.As(err, &qe) {
		return qe, true
	}
	return nil, false
}
