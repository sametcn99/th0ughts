//go:build nativekv && cgo

package storage

/*
#cgo LDFLAGS: -L${SRCDIR}/../native/lib -lth0ughts_kv
#cgo linux LDFLAGS: -ldl -lpthread -lm
#include "nativekv_build.h"
#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>
typedef struct { uint8_t *ptr; size_t len; } kv_buffer;
int kv_open(kv_buffer, bool, void **, kv_buffer *);
void kv_close(void *);
void kv_free(kv_buffer);
int kv_call(void *, uint8_t, kv_buffer, kv_buffer, kv_buffer, uint8_t, size_t, kv_buffer *);
*/
import "C"
import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/gofrs/flock"
)

const NativeKVEnabled = true

type nativeStore struct {
	slots  chan struct{}
	mu     sync.RWMutex
	handle unsafe.Pointer
	lock   *flock.Flock
}

func buffer(b []byte) C.kv_buffer {
	return C.kv_buffer{ptr: (*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(b))), len: C.size_t(len(b))}
}

func nativeResult(status C.int, out C.kv_buffer) ([]byte, error) {
	defer C.kv_free(out)
	if status == 1 {
		return nil, ErrMissing
	}
	data := append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(out.ptr)), int(out.len))...)
	if status != 0 {
		return nil, errors.New(string(data))
	}
	return data, nil
}

func openNative(c Config) (Store, error) {
	if c.Address == "" {
		return nil, errors.New("missing database path")
	}
	dir := filepath.Dir(c.Address)
	lockPath := c.Address + ".lock"
	if c.Driver == "lmdb" {
		dir = c.Address
		lockPath = filepath.Join(dir, ".th0ughts.lock")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	db := &nativeStore{lock: flock.New(lockPath), slots: make(chan struct{}, 64)}
	ok, err := db.lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("database already open")
	}
	if c.Driver == "redb" {
		// Restrict permissions even when the process inherits a permissive umask.
		f, err := os.OpenFile(c.Address, os.O_CREATE|os.O_RDWR, 0600)
		if err == nil {
			err = f.Close()
		}
		if err != nil {
			db.lock.Unlock()
			return nil, err
		}
	}
	var handle unsafe.Pointer
	var out C.kv_buffer
	status := C.kv_open(buffer([]byte(c.Address)), C.bool(c.Driver == "lmdb"), &handle, &out)
	if _, err = nativeResult(status, out); err != nil {
		db.lock.Unlock()
		return nil, err
	}
	db.handle = handle
	return db, nil
}

func (s *nativeStore) call(ctx context.Context, op byte, a, b, c []byte, visibility byte, limit int) ([]byte, error) {
	// Bound native threads and LMDB reader slots during a traffic spike.
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.handle == nil {
		return nil, errors.New("closed database")
	}
	var out C.kv_buffer
	status := C.kv_call(s.handle, C.uint8_t(op), buffer(a), buffer(b), buffer(c), C.uint8_t(visibility), C.size_t(max(limit, 0)), &out)
	return nativeResult(status, out)
}

func primaryKey(kind, id string) []byte { return []byte("r" + kind + "\x00" + id) }

// Flipping the sign bit makes big-endian byte order match signed rank order.
func orderKey(start []byte, rank int64, id string) []byte {
	return append(binary.BigEndian.AppendUint64(start, uint64(rank)^(1<<63)), id...)
}

func (s *nativeStore) Get(ctx context.Context, kind, id string) (Record, error) {
	var r Record
	b, err := s.call(ctx, 0, primaryKey(kind, id), nil, nil, 0, 0)
	if err == nil {
		err = json.Unmarshal(b, &r)
	}
	return r, err
}
func (s *nativeStore) Put(ctx context.Context, r Record) error {
	if !ValidKey(r.ID) || !ValidKey(r.Kind) {
		return errors.New("invalid record key")
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	flag := byte(0)
	if r.Published {
		flag = 1
	}
	_, err = s.call(ctx, 1, primaryKey(r.Kind, r.ID), orderKey([]byte("s"+r.Kind+"\x00"), r.Rank, r.ID), append([]byte{flag}, data...), 0, 0)
	return err
}
func (s *nativeStore) Delete(ctx context.Context, kind, id string) error {
	_, err := s.call(ctx, 1, primaryKey(kind, id), nil, nil, 0, 0)
	return err
}
func (s *nativeStore) List(ctx context.Context, q Query) ([]Record, error) {
	if q.Public && q.Unpublished {
		return []Record{}, nil
	}
	low := []byte("s" + q.Kind + "\x00")
	high := []byte("s" + q.Kind + "\x01")
	if q.Before != nil {
		high = orderKey(append([]byte(nil), low...), q.Before.Rank, q.Before.ID)
	}
	visibility := byte(2)
	if q.Public {
		visibility = 1
	} else if q.Unpublished {
		visibility = 0
	}
	b, err := s.call(ctx, 2, low, high, []byte(q.Prefix), visibility, q.Limit)
	if err != nil {
		return nil, err
	}
	var rows []Record
	err = json.Unmarshal(b, &rows)
	return rows, err
}
func (s *nativeStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handle == nil {
		return nil
	}
	C.kv_close(s.handle)
	s.handle = nil
	return s.lock.Unlock()
}
