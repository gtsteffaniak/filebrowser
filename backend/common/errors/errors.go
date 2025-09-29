package errors

import "errors"

var (
	ErrEmptyKey             = errors.New("empty key")
	ErrExist                = errors.New("the resource already exists")
	ErrNotExist             = errors.New("the resource does not exist")
	ErrEmptyPassword        = errors.New("password is empty")
	ErrEmptyUsername        = errors.New("username is empty")
	ErrEmptyRequest         = errors.New("empty request")
	ErrScopeIsRelative      = errors.New("scope is a relative path")
	ErrInvalidDataType      = errors.New("invalid data type")
	ErrIsDirectory          = errors.New("file is directory")
	ErrInvalidOption        = errors.New("invalid option")
	ErrInvalidAuthMethod    = errors.New("invalid auth method")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrInvalidRequestParams = errors.New("invalid request params")
	ErrSourceIsParent       = errors.New("source is parent")
	ErrNoTotpProvided       = errors.New("OTP code is required for user")
	ErrNoTotpConfigured     = errors.New("OTP is enforced, but user is not yet configured")
	ErrUnauthorized         = errors.New("user unauthorized")
	ErrNotIndexed           = errors.New("directory or item excluded from indexing")
	ErrWrongLoginMethod     = errors.New("user attempted to login with wrong login method")
	ErrTimeout              = errors.New("API request timed out")
	ErrPreviewTimeout       = errors.New("Preview generation timed out after 30 seconds")
)
