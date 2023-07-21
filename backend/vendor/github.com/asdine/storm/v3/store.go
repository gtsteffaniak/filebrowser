package storm

import (
	"bytes"
	"reflect"

	"github.com/asdine/storm/v3/index"
	"github.com/asdine/storm/v3/q"
	bolt "go.etcd.io/bbolt"
)

// TypeStore stores user defined types in BoltDB.
type TypeStore interface {
	Finder
	// Init creates the indexes and buckets for a given structure
	Init(data interface{}) error

	// ReIndex rebuilds all the indexes of a bucket
	ReIndex(data interface{}) error

	// Save a structure
	Save(data interface{}) error

	// Update a structure
	Update(data interface{}) error

	// UpdateField updates a single field
	UpdateField(data interface{}, fieldName string, value interface{}) error

	// Drop a bucket
	Drop(data interface{}) error

	// DeleteStruct deletes a structure from the associated bucket
	DeleteStruct(data interface{}) error
}

// Init creates the indexes and buckets for a given structure
func (n *node) Init(data interface{}) error {
	v := reflect.ValueOf(data)
	cfg, err := extract(&v)
	if err != nil {
		return err
	}

	return n.readWriteTx(func(tx *bolt.Tx) error {
		return n.init(tx, cfg)
	})
}

func (n *node) init(tx *bolt.Tx, cfg *structConfig) error {
	bucket, err := n.CreateBucketIfNotExists(tx, cfg.Name)
	if err != nil {
		return err
	}

	// save node configuration in the bucket
	_, err = newMeta(bucket, n)
	if err != nil {
		return err
	}

	for fieldName, fieldCfg := range cfg.Fields {
		if fieldCfg.Index == "" {
			continue
		}
		switch fieldCfg.Index {
		case tagUniqueIdx:
			_, err = index.NewUniqueIndex(bucket, []byte(indexPrefix+fieldName))
		case tagIdx:
			_, err = index.NewListIndex(bucket, []byte(indexPrefix+fieldName))
		default:
			err = ErrIdxNotFound
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (n *node) ReIndex(data interface{}) error {
	ref := reflect.ValueOf(data)

	if !ref.IsValid() || ref.Kind() != reflect.Ptr || ref.Elem().Kind() != reflect.Struct {
		return ErrStructPtrNeeded
	}

	cfg, err := extract(&ref)
	if err != nil {
		return err
	}

	return n.readWriteTx(func(tx *bolt.Tx) error {
		return n.reIndex(tx, data, cfg)
	})
}

func (n *node) reIndex(tx *bolt.Tx, data interface{}, cfg *structConfig) error {
	root := n.WithTransaction(tx)
	nodes := root.From(cfg.Name).PrefixScan(indexPrefix)
	bucket := root.GetBucket(tx, cfg.Name)
	if bucket == nil {
		return ErrNotFound
	}

	for _, node := range nodes {
		buckets := node.Bucket()
		name := buckets[len(buckets)-1]
		err := bucket.DeleteBucket([]byte(name))
		if err != nil {
			return err
		}
	}

	total, err := root.Count(data)
	if err != nil {
		return err
	}

	for i := 0; i < total; i++ {
		err = root.Select(q.True()).Skip(i).First(data)
		if err != nil {
			return err
		}

		err = root.Update(data)
		if err != nil {
			return err
		}
	}

	return nil
}

// Save a structure
func (n *node) Save(data interface{}) error {
	ref := reflect.ValueOf(data)

	if !ref.IsValid() || ref.Kind() != reflect.Ptr || ref.Elem().Kind() != reflect.Struct {
		return ErrStructPtrNeeded
	}

	cfg, err := extract(&ref)
	if err != nil {
		return err
	}

	if cfg.ID.IsZero {
		if !cfg.ID.IsInteger || !cfg.ID.Increment {
			return ErrZeroID
		}
	}

	return n.readWriteTx(func(tx *bolt.Tx) error {
		return n.save(tx, cfg, data, false)
	})
}

func (n *node) save(tx *bolt.Tx, cfg *structConfig, data interface{}, update bool) error {
	bucket, err := n.CreateBucketIfNotExists(tx, cfg.Name)
	if err != nil {
		return err
	}

	// save node configuration in the bucket
	meta, err := newMeta(bucket, n)
	if err != nil {
		return err
	}

	if cfg.ID.IsZero {
		err = meta.increment(cfg.ID)
		if err != nil {
			return err
		}
	}

	id, err := toBytes(cfg.ID.Value.Interface(), n.codec)
	if err != nil {
		return err
	}

	for fieldName, fieldCfg := range cfg.Fields {
		if !update && !fieldCfg.IsID && fieldCfg.Increment && fieldCfg.IsInteger && fieldCfg.IsZero {
			err = meta.increment(fieldCfg)
			if err != nil {
				return err
			}
		}

		if fieldCfg.Index == "" {
			continue
		}

		idx, err := getIndex(bucket, fieldCfg.Index, fieldName)
		if err != nil {
			return err
		}

		if update && fieldCfg.IsZero && !fieldCfg.ForceUpdate {
			continue
		}

		if fieldCfg.IsZero {
			err = idx.RemoveID(id)
			if err != nil {
				return err
			}
			continue
		}

		value, err := toBytes(fieldCfg.Value.Interface(), n.codec)
		if err != nil {
			return err
		}

		var found bool
		idsSaved, err := idx.All(value, nil)
		if err != nil {
			return err
		}
		for _, idSaved := range idsSaved {
			if bytes.Compare(idSaved, id) == 0 {
				found = true
				break
			}
		}

		if found {
			continue
		}

		err = idx.RemoveID(id)
		if err != nil {
			return err
		}

		err = idx.Add(value, id)
		if err != nil {
			if err == index.ErrAlreadyExists {
				return ErrAlreadyExists
			}
			return err
		}
	}

	raw, err := n.codec.Marshal(data)
	if err != nil {
		return err
	}

	return bucket.Put(id, raw)
}

// Update a structure
func (n *node) Update(data interface{}) error {
	return n.update(data, func(ref *reflect.Value, current *reflect.Value, cfg *structConfig) error {
		numfield := ref.NumField()
		for i := 0; i < numfield; i++ {
			f := ref.Field(i)
			if ref.Type().Field(i).PkgPath != "" {
				continue
			}
			zero := reflect.Zero(f.Type()).Interface()
			actual := f.Interface()
			if !reflect.DeepEqual(actual, zero) {
				cf := current.Field(i)
				cf.Set(f)
				idxInfo, ok := cfg.Fields[ref.Type().Field(i).Name]
				if ok {
					idxInfo.Value = &cf
				}
			}
		}
		return nil
	})
}

// UpdateField updates a single field
func (n *node) UpdateField(data interface{}, fieldName string, value interface{}) error {
	return n.update(data, func(ref *reflect.Value, current *reflect.Value, cfg *structConfig) error {
		f := current.FieldByName(fieldName)
		if !f.IsValid() {
			return ErrNotFound
		}
		tf, _ := current.Type().FieldByName(fieldName)
		if tf.PkgPath != "" {
			return ErrNotFound
		}
		v := reflect.ValueOf(value)
		if v.Kind() != f.Kind() {
			return ErrIncompatibleValue
		}
		f.Set(v)
		idxInfo, ok := cfg.Fields[fieldName]
		if ok {
			idxInfo.Value = &f
			idxInfo.IsZero = isZero(idxInfo.Value)
			idxInfo.ForceUpdate = true
		}
		return nil
	})
}

func (n *node) update(data interface{}, fn func(*reflect.Value, *reflect.Value, *structConfig) error) error {
	ref := reflect.ValueOf(data)
	if !ref.IsValid() || ref.Kind() != reflect.Ptr || ref.Elem().Kind() != reflect.Struct {
		return ErrStructPtrNeeded
	}

	cfg, err := extract(&ref)
	if err != nil {
		return err
	}

	if cfg.ID.IsZero {
		return ErrNoID
	}

	current := reflect.New(reflect.Indirect(ref).Type())

	return n.readWriteTx(func(tx *bolt.Tx) error {
		err = n.WithTransaction(tx).One(cfg.ID.Name, cfg.ID.Value.Interface(), current.Interface())
		if err != nil {
			return err
		}

		ref := reflect.ValueOf(data).Elem()
		cref := current.Elem()
		err = fn(&ref, &cref, cfg)
		if err != nil {
			return err
		}

		return n.save(tx, cfg, current.Interface(), true)
	})
}

// Drop a bucket
func (n *node) Drop(data interface{}) error {
	var bucketName string

	v := reflect.ValueOf(data)
	if v.Kind() != reflect.String {
		info, err := extract(&v)
		if err != nil {
			return err
		}

		bucketName = info.Name
	} else {
		bucketName = v.Interface().(string)
	}

	return n.readWriteTx(func(tx *bolt.Tx) error {
		return n.drop(tx, bucketName)
	})
}

func (n *node) drop(tx *bolt.Tx, bucketName string) error {
	bucket := n.GetBucket(tx)
	if bucket == nil {
		return tx.DeleteBucket([]byte(bucketName))
	}

	return bucket.DeleteBucket([]byte(bucketName))
}

// DeleteStruct deletes a structure from the associated bucket
func (n *node) DeleteStruct(data interface{}) error {
	ref := reflect.ValueOf(data)

	if !ref.IsValid() || ref.Kind() != reflect.Ptr || ref.Elem().Kind() != reflect.Struct {
		return ErrStructPtrNeeded
	}

	cfg, err := extract(&ref)
	if err != nil {
		return err
	}

	id, err := toBytes(cfg.ID.Value.Interface(), n.codec)
	if err != nil {
		return err
	}

	return n.readWriteTx(func(tx *bolt.Tx) error {
		return n.deleteStruct(tx, cfg, id)
	})
}

func (n *node) deleteStruct(tx *bolt.Tx, cfg *structConfig, id []byte) error {
	bucket := n.GetBucket(tx, cfg.Name)
	if bucket == nil {
		return ErrNotFound
	}

	for fieldName, fieldCfg := range cfg.Fields {
		if fieldCfg.Index == "" {
			continue
		}

		idx, err := getIndex(bucket, fieldCfg.Index, fieldName)
		if err != nil {
			return err
		}

		err = idx.RemoveID(id)
		if err != nil {
			if err == index.ErrNotFound {
				return ErrNotFound
			}
			return err
		}
	}

	raw := bucket.Get(id)
	if raw == nil {
		return ErrNotFound
	}

	return bucket.Delete(id)
}
