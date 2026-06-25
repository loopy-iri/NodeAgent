package tenant

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

var tenantsBucket = []byte("tenants")

// boltStore is a bbolt-backed Store: a single embedded file, pure Go, no CGO.
type boltStore struct {
	db *bolt.DB
}

// OpenBoltStore opens (creating if needed) a bbolt database at path.
func OpenBoltStore(path string) (Store, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bolt: %w", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, e := tx.CreateBucketIfNotExists(tenantsBucket)
		return e
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init bucket: %w", err)
	}
	return &boltStore{db: db}, nil
}

func (s *boltStore) LoadTenants() ([]*Tenant, error) {
	var out []*Tenant
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(tenantsBucket)
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, v []byte) error {
			var t Tenant
			if err := json.Unmarshal(v, &t); err != nil {
				return err
			}
			out = append(out, &t)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *boltStore) SaveTenant(t *Tenant) error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(tenantsBucket)
		if b == nil {
			return fmt.Errorf("bucket %q missing", tenantsBucket)
		}
		return b.Put([]byte(t.ID), data)
	})
}

func (s *boltStore) DeleteTenant(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(tenantsBucket)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(id))
	})
}

func (s *boltStore) Close() error { return s.db.Close() }
