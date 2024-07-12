// MIT License
//
// Copyright (c) 2024 chaunsin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package badger

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/dgraph-io/badger/v4"
)

type Badger struct {
	path string
	db   *badger.DB
}

func New(path string) (*Badger, error) {
	var opts = badger.DefaultOptions(path).WithLoggingLevel(badger.WARNING)
	// .WithSyncWrites(false)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	b := &Badger{
		path: path,
		db:   db,
	}
	return b, nil
}

func (b *Badger) Close(ctx context.Context) error {
	return b.db.Close()
}

func (b *Badger) Set(ctx context.Context, key, value string, ttl ...time.Duration) error {
	return b.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), []byte(value))
		if len(ttl) > 0 && ttl[0] > 0 {
			entry.WithTTL(ttl[0])
		}
		return txn.SetEntry(entry)
	})
}

func (b *Badger) Get(ctx context.Context, key string) (string, error) {
	var resp string
	if err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			resp = string(val)
			return nil
		})
	}); err != nil {
		return "", err
	}
	return resp, nil
}

func (b *Badger) Exists(ctx context.Context, key string) (bool, error) {
	_, err := b.Get(ctx, key)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (b *Badger) Increment(ctx context.Context, key string, value int64, ttl ...time.Duration) (int64, error) {
	var oldValue int64
	err := b.db.Update(func(txn *badger.Txn) error {
		var found = true
		item, err := txn.Get([]byte(key))
		if errors.Is(err, badger.ErrKeyNotFound) {
			found = false
			// continue
		} else if err != nil {
			return err
		} else {
			v, err := item.ValueCopy(nil)
			if err != nil {
				return fmt.Errorf("ValueCopy: %w", err)
			}
			oldValue, err = strconv.ParseInt(string(v), 10, 64)
			if err != nil {
				return fmt.Errorf("ParseInt: %w", err)
			}
			value += oldValue
		}

		var entry = badger.NewEntry([]byte(key), []byte(fmt.Sprintf("%v", value)))
		if len(ttl) > 0 && ttl[0] > 0 && !found {
			entry.WithTTL(ttl[0])
		}
		return txn.SetEntry(entry)
	})
	return oldValue, err
}
