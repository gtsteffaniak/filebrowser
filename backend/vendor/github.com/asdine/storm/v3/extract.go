package storm

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/asdine/storm/v3/index"
	bolt "go.etcd.io/bbolt"
)

// Storm tags
const (
	tagID        = "id"
	tagIdx       = "index"
	tagUniqueIdx = "unique"
	tagInline    = "inline"
	tagIncrement = "increment"
	indexPrefix  = "__storm_index_"
)

type fieldConfig struct {
	Name           string
	Index          string
	IsZero         bool
	IsID           bool
	Increment      bool
	IncrementStart int64
	IsInteger      bool
	Value          *reflect.Value
	ForceUpdate    bool
}

// structConfig is a structure gathering all the relevant informations about a model
type structConfig struct {
	Name   string
	Fields map[string]*fieldConfig
	ID     *fieldConfig
}

func extract(s *reflect.Value, mi ...*structConfig) (*structConfig, error) {
	if s.Kind() == reflect.Ptr {
		e := s.Elem()
		s = &e
	}
	if s.Kind() != reflect.Struct {
		return nil, ErrBadType
	}

	typ := s.Type()

	var child bool

	var m *structConfig
	if len(mi) > 0 {
		m = mi[0]
		child = true
	} else {
		m = &structConfig{}
		m.Fields = make(map[string]*fieldConfig)
	}

	if m.Name == "" {
		m.Name = typ.Name()
	}

	numFields := s.NumField()
	for i := 0; i < numFields; i++ {
		field := typ.Field(i)
		value := s.Field(i)

		if field.PkgPath != "" {
			continue
		}

		err := extractField(&value, &field, m, child)
		if err != nil {
			return nil, err
		}
	}

	if child {
		return m, nil
	}

	if m.ID == nil {
		return nil, ErrNoID
	}

	if m.Name == "" {
		return nil, ErrNoName
	}

	return m, nil
}

func extractField(value *reflect.Value, field *reflect.StructField, m *structConfig, isChild bool) error {
	var f *fieldConfig
	var err error

	tag := field.Tag.Get("storm")
	if tag != "" {
		f = &fieldConfig{
			Name:           field.Name,
			IsZero:         isZero(value),
			IsInteger:      isInteger(value),
			Value:          value,
			IncrementStart: 1,
		}

		tags := strings.Split(tag, ",")

		for _, tag := range tags {
			switch tag {
			case "id":
				f.IsID = true
				f.Index = tagUniqueIdx
			case tagUniqueIdx, tagIdx:
				f.Index = tag
			case tagInline:
				if value.Kind() == reflect.Ptr {
					e := value.Elem()
					value = &e
				}
				if value.Kind() == reflect.Struct {
					a := value.Addr()
					_, err := extract(&a, m)
					if err != nil {
						return err
					}
				}
				// we don't need to save this field
				return nil
			default:
				if strings.HasPrefix(tag, tagIncrement) {
					f.Increment = true
					parts := strings.Split(tag, "=")
					if parts[0] != tagIncrement {
						return ErrUnknownTag
					}
					if len(parts) > 1 {
						f.IncrementStart, err = strconv.ParseInt(parts[1], 0, 64)
						if err != nil {
							return err
						}
					}
				} else {
					return ErrUnknownTag
				}
			}
		}

		if _, ok := m.Fields[f.Name]; !ok || !isChild {
			m.Fields[f.Name] = f
		}
	}

	if m.ID == nil && f != nil && f.IsID {
		m.ID = f
	}

	// the field is named ID and no ID field has been detected before
	if m.ID == nil && field.Name == "ID" {
		if f == nil {
			f = &fieldConfig{
				Index:          tagUniqueIdx,
				Name:           field.Name,
				IsZero:         isZero(value),
				IsInteger:      isInteger(value),
				IsID:           true,
				Value:          value,
				IncrementStart: 1,
			}
			m.Fields[field.Name] = f
		}
		m.ID = f
	}

	return nil
}

func extractSingleField(ref *reflect.Value, fieldName string) (*structConfig, error) {
	var cfg structConfig
	cfg.Fields = make(map[string]*fieldConfig)

	f, ok := ref.Type().FieldByName(fieldName)
	if !ok || f.PkgPath != "" {
		return nil, fmt.Errorf("field %s not found", fieldName)
	}

	v := ref.FieldByName(fieldName)
	err := extractField(&v, &f, &cfg, false)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func getIndex(bucket *bolt.Bucket, idxKind string, fieldName string) (index.Index, error) {
	var idx index.Index
	var err error

	switch idxKind {
	case tagUniqueIdx:
		idx, err = index.NewUniqueIndex(bucket, []byte(indexPrefix+fieldName))
	case tagIdx:
		idx, err = index.NewListIndex(bucket, []byte(indexPrefix+fieldName))
	default:
		err = ErrIdxNotFound
	}

	return idx, err
}

func isZero(v *reflect.Value) bool {
	zero := reflect.Zero(v.Type()).Interface()
	current := v.Interface()
	return reflect.DeepEqual(current, zero)
}

func isInteger(v *reflect.Value) bool {
	kind := v.Kind()
	return v != nil && kind >= reflect.Int && kind <= reflect.Uint64
}
