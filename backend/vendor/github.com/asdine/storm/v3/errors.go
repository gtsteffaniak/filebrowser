package storm

import "errors"

// Errors
var (
	// ErrNoID is returned when no ID field or id tag is found in the struct.
	ErrNoID = errors.New("missing struct tag id or ID field")

	// ErrZeroID is returned when the ID field is a zero value.
	ErrZeroID = errors.New("id field must not be a zero value")

	// ErrBadType is returned when a method receives an unexpected value type.
	ErrBadType = errors.New("provided data must be a struct or a pointer to struct")

	// ErrAlreadyExists is returned uses when trying to set an existing value on a field that has a unique index.
	ErrAlreadyExists = errors.New("already exists")

	// ErrNilParam is returned when the specified param is expected to be not nil.
	ErrNilParam = errors.New("param must not be nil")

	// ErrUnknownTag is returned when an unexpected tag is specified.
	ErrUnknownTag = errors.New("unknown tag")

	// ErrIdxNotFound is returned when the specified index is not found.
	ErrIdxNotFound = errors.New("index not found")

	// ErrSlicePtrNeeded is returned when an unexpected value is given, instead of a pointer to slice.
	ErrSlicePtrNeeded = errors.New("provided target must be a pointer to slice")

	// ErrStructPtrNeeded is returned when an unexpected value is given, instead of a pointer to struct.
	ErrStructPtrNeeded = errors.New("provided target must be a pointer to struct")

	// ErrPtrNeeded is returned when an unexpected value is given, instead of a pointer.
	ErrPtrNeeded = errors.New("provided target must be a pointer to a valid variable")

	// ErrNoName is returned when the specified struct has no name.
	ErrNoName = errors.New("provided target must have a name")

	// ErrNotFound is returned when the specified record is not saved in the bucket.
	ErrNotFound = errors.New("not found")

	// ErrNotInTransaction is returned when trying to rollback or commit when not in transaction.
	ErrNotInTransaction = errors.New("not in transaction")

	// ErrIncompatibleValue is returned when trying to set a value with a different type than the chosen field
	ErrIncompatibleValue = errors.New("incompatible value")

	// ErrDifferentCodec is returned when using a codec different than the first codec used with the bucket.
	ErrDifferentCodec = errors.New("the selected codec is incompatible with this bucket")
)
