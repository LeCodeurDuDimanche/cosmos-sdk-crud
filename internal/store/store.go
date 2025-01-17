package store

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/iov-one/cosmos-sdk-crud/internal/store/indexes"
	"github.com/iov-one/cosmos-sdk-crud/internal/store/metadata"
	"github.com/iov-one/cosmos-sdk-crud/internal/store/objects"
	"github.com/iov-one/cosmos-sdk-crud/internal/store/types"
)

// DefaultVerifyType asserts that the type is not verified when
// interacting with the store
const DefaultVerifyType = false

// ObjectsPrefix defines at which prefix of the kv store
// we are actually saving the concrete objects
const ObjectsPrefix = 0x0

// IndexesPrefix defines the prefix of the kv store
// in which we are storing indexes data
const IndexesPrefix = 0x1

// MetadataPrefix defines the prefix of the kv store
// in which we are storing objects metadata
const MetadataPrefix = 0x2

type OptionFunc func(s *Store)

func VerifyTypes(s *Store) {
	s.verifyType = true
}

func DoNotVerifyTypes(s *Store) {
	s.verifyType = false
}

type Store struct {
	cdc codec.Marshaler

	verifyType bool

	objects  objects.Store
	indexes  indexes.Store
	metadata metadata.Store
}

func NewStore(cdc codec.Marshaler, db sdk.KVStore, pfx []byte, options ...OptionFunc) Store {
	prefixedStore := prefix.NewStore(db, pfx)
	s := Store{
		cdc:        cdc,
		verifyType: DefaultVerifyType,
		objects:    objects.NewStore(cdc, prefix.NewStore(prefixedStore, []byte{ObjectsPrefix})),
		indexes:    indexes.NewStore(cdc, prefix.NewStore(prefixedStore, []byte{IndexesPrefix})),
		metadata:   metadata.NewStore(cdc, prefix.NewStore(prefixedStore, []byte{MetadataPrefix})),
	}
	for _, opt := range options {
		opt(&s)
	}
	return s
}

func (s Store) Create(o types.Object) error {
	err := s.objects.Create(o)
	if err != nil {
		return err
	}
	// create indexes
	err = s.indexes.Index(o)
	if err != nil {
		err2 := s.objects.Delete(o.PrimaryKey())
		if err2 != nil {
			panic(fmt.Errorf("state corruption unable to rollback delete after error %s: %s", err, err2))
		}
		return err
	}
	// done
	return nil
}

func (s Store) Read(primaryKey []byte, o types.Object) error {
	return s.objects.Read(primaryKey, o)
}

func (s Store) Update(o types.Object) error {
	// update indexes
	err := s.indexes.Delete(o.PrimaryKey())
	if err != nil {
		return err
	}
	err = s.indexes.Index(o)
	if err != nil {
		// state corruption, cannot rollback TODO make rollback possible
		panic(err)
	}
	err = s.objects.Update(o)
	if err != nil {
		// state corruption panic
		panic(err)
	}
	return nil
}

func (s Store) Delete(primaryKey []byte) error {
	err := s.indexes.Delete(primaryKey)
	if err != nil {
		return err
	}
	err = s.objects.Delete(primaryKey)
	if err != nil {
		// state corruption, cannot rollback. todo make rollback possible
		panic(err)
	}
	return nil
}

func (s Store) RegisterObject(o types.Object) {
}

func (s Store) Query(sks []types.SecondaryKey, start, end uint64) (*Cursor, error) {
	var err error
	var pks [][]byte
	if len(sks) == 0 {
		pks, err = s.objects.GetAllKeys(start, end)
	} else {
		pks, err = s.indexes.Filter(sks, start, end)
	}
	if err != nil {
		return nil, err
	}
	return newFilter(pks, s), nil
}

func newFilter(primaryKeys [][]byte, store Store) *Cursor {
	return &Cursor{
		keys:     primaryKeys,
		store:    store,
		maxKeys:  len(primaryKeys),
		keyIndex: 0,
	}
}

type Cursor struct {
	maxKeys  int
	keyIndex int
	keys     [][]byte
	store    Store
}

func (c *Cursor) Next() {
	if c.keyIndex < c.maxKeys {
		c.keyIndex += 1
	}
}

func (c *Cursor) Read(o types.Object) error {
	return c.store.Read(c.currKey(), o)
}

func (c *Cursor) Delete() error {
	return c.store.Delete(c.currKey())
}

func (c *Cursor) Update(o types.Object) error {
	return c.store.Update(o)
}

func (c *Cursor) Valid() bool {
	return c.keyIndex < c.maxKeys
}

func (c *Cursor) currKey() []byte {
	return c.keys[c.keyIndex]
}
