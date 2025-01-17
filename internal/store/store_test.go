package store

import (
	"errors"
	"reflect"
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/iov-one/cosmos-sdk-crud/internal/store/types"
	"github.com/iov-one/cosmos-sdk-crud/internal/test"
)

func TestStore(t *testing.T) {
	db, cdc, err := test.NewStore()
	if err != nil {
		t.Fatal("failed precondition", err)
	}
	s := NewStore(cdc, db, nil)
	obj := test.NewDeterministicObject()
	// test create
	err = s.Create(obj)
	if err != nil {
		t.Fatal(err)
	}
	// test read
	var expected = test.NewObject()
	err = s.Read(obj.PrimaryKey(), expected)
	if err != nil {
		t.Fatal(err)
	}
	if err := obj.Equals(expected); err != nil {
		t.Fatal(err)
	}
	// test update
	update := obj
	update.TestSecondaryKeyB = []byte("test-update")
	err = s.Update(update)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Read(obj.PrimaryKey(), expected)
	if err != nil {
		t.Fatal(err)
	}
	if err := update.Equals(expected); err != nil {
		t.Fatal(err)
	}
	// test cursor
	crs, err := s.Query([]types.SecondaryKey{
		update.FirstSecondaryKey(),
	}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// test read
	err = crs.Read(expected)
	if err != nil {
		t.Logf("%s", crs.currKey())
		t.Fatal(err)
	}
	if err := update.Equals(expected); err != nil {
		t.Fatal(err)
	}
	// test update
	update.TestSecondaryKeyA = []byte("another-update")
	err = crs.Update(update)
	if err != nil {
		t.Fatal(err)
	}
	expected.Reset()
	err = crs.Read(expected)
	if err := update.Equals(expected); err != nil {
		t.Fatal(err)
	}
	// test delete
	err = crs.Delete()
	if err != nil {
		t.Fatal(err)
	}
	err = crs.Read(expected)
	if !errors.Is(err, types.ErrNotFound) {
		t.Fatal("unexpected error", err)
	}

	t.Run("create/duplicate", func(t *testing.T) {
		if err = s.Create(test.NewDeterministicObject()); err != nil {
			t.Fatal("Unexpected error :", err)
		}
		err = s.Create(test.NewDeterministicObject())
		if !errors.Is(err, types.ErrAlreadyExists) {
			t.Fatal("Object should be a duplicate")
		}
	})
	t.Run("delete/existing", func(t *testing.T) {
		err = s.Delete(test.NewDeterministicObject().PrimaryKey())
		if err != nil {
			t.Fatal("Unexpected error :", err)
		}
		err = s.Read(test.NewDeterministicObject().PrimaryKey(), obj)
		if !errors.Is(err, types.ErrNotFound) {
			t.Fatal("Object should not exist anymore")
		}
	})
	t.Run("delete/non existing", func(t *testing.T) {
		err = s.Delete(test.NewDeterministicObject().PrimaryKey())
		if !errors.Is(err, types.ErrNotFound) {
			t.Fatal("Deleting an non existing object should result in a not found error")
		}
	})

	t.Run("indexes", func(t *testing.T) {
		obj = test.NewDeterministicObject()
		if err = s.Create(obj); err != nil {
			t.Fatal("Unexpected error :", err)
		}
		if err = s.indexes.Index(obj); !errors.Is(err, types.ErrAlreadyExists) {
			t.Fatal("The object indexes should have been added to the index store")
		}

		if err = s.Delete(obj.PrimaryKey()); err != nil {
			t.Fatal("Unexpected error :", err)
		}
		if err = s.indexes.Index(obj); err != nil {
			t.Fatal("The object indexes should have been removed from the index store, err :", err)
		}

	})
	t.Run("query", func(t *testing.T) {
		s := NewStore(cdc, db, []byte("query"))
		obj = test.NewDeterministicObject()
		if err = s.Create(obj); err != nil {
			t.Fatal("Unexpected error :", err)
		}
		cursor, err := s.Query(obj.SecondaryKeys(), 0, 0)
		if err != nil {
			t.Fatal("Unexpected error :", err)
		}

		if !cursor.Valid() {
			t.Fatal("Cursor should be valid at this point")
		}
		actual := test.NewObject()
		if err := cursor.Read(actual); err != nil {
			t.Fatal("Unexpected error :", err)
		}
		if actual.Equals(&obj) != nil {
			t.Fatal("Invalid object, expected = ", test.NewDeterministicObject(), ", actual = ", actual)
		}

		cursor.Next()
		if cursor.Valid() {
			t.Fatal("Cursor should be invalid at this point")
		}
	})

	t.Run("query all", func(t *testing.T) {
		s, objs := createStoreWithRandomObjects(cdc, db, t, 50, "queryall")

		results, err := s.Query(nil, 0, 0)
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		i := 0
		for ; results.Valid(); results.Next() {
			if i == len(objs) {
				t.Fatalf("Length mismatch, exepected %v elements but got more", len(objs))
			}
			var actual = test.NewObject()
			if err := results.Read(*actual); err != nil {
				t.Fatal("Unexpected error :", err)
			}
			if !reflect.DeepEqual(*actual, objs[i]) {
				t.Fatalf("Object mismatch at index %v : expected = %v(%[2]T), actual = %v(%[3]T)", i, objs[i], actual)
			}
			i++
		}

	})
}

func createStoreWithRandomObjects(cdc codec.Marshaler, db sdk.KVStore, t *testing.T, n int, uniqueID string) (Store, []types.Object) {
	store := NewStore(cdc, db, []byte(uniqueID))
	addToStore := func(obj types.Object) error {
		return store.Create(obj)
	}
	return store, test.CreateRandomObjects(addToStore, t, n)
}
