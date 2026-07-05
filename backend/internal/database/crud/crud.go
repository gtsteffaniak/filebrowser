package crud

// CrudBackend defines the basic CRUD operations for a type T.
type CrudBackend[T any] interface {
	GetByID(id any) (*T, error)
	GetAll() ([]*T, error)
	Save(obj *T) error
	DeleteByID(id any) error
}

// Storage provides generic CRUD operations for any type T.
type Storage[T any] struct {
	backend CrudBackend[T]
}

func NewStorage[T any](backend CrudBackend[T]) *Storage[T] {
	return &Storage[T]{backend: backend}
}

func (s *Storage[T]) Get(id any) (*T, error) {
	return s.backend.GetByID(id)
}

func (s *Storage[T]) GetAll() ([]*T, error) {
	return s.backend.GetAll()
}

func (s *Storage[T]) Save(obj *T) error {
	return s.backend.Save(obj)
}

func (s *Storage[T]) Delete(id any) error {
	return s.backend.DeleteByID(id)
}
