package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofrs/flock"
)

func Atomic(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".pending-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	return syncDir(dir)
}
func syncDir(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}

type files struct {
	mu   sync.RWMutex
	rows map[string]Record
	cfg  Config
	lock *flock.Flock
}

func openFiles(c Config) (Store, error) {
	if c.Address == "" {
		return nil, errors.New("missing storage path")
	}
	dir := filepath.Dir(c.Address)
	if c.Driver == "json-dir" {
		dir = c.Address
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	f := &files{cfg: c, rows: map[string]Record{}, lock: flock.New(c.Address + ".lock")}
	ok, e := f.lock.TryLock()
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, errors.New("store already open")
	}
	if c.Driver == "json" {
		var b []byte
		b, e = os.ReadFile(c.Address)
		if e == nil {
			e = json.Unmarshal(b, &f.rows)
		} else if errors.Is(e, os.ErrNotExist) {
			e = nil
		}
	} else {
		e = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(p) != ".json" {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			var r Record
			if err = json.Unmarshal(b, &r); err != nil {
				return err
			}
			if !ValidKey(r.Kind) || !ValidKey(r.ID) || p != filepath.Join(dir, r.Kind, r.ID+".json") {
				return fmt.Errorf("invalid record path %s", p)
			}
			f.rows[r.Kind+"/"+r.ID] = r
			return nil
		})
	}
	if e != nil {
		f.Close()
		return nil, e
	}
	if f.rows == nil {
		f.Close()
		return nil, errors.New("invalid JSON store")
	}
	return f, nil
}
func (f *files) Get(_ context.Context, k, id string) (Record, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	r, ok := f.rows[k+"/"+id]
	if !ok {
		return r, ErrMissing
	}
	return r, nil
}
func (f *files) List(_ context.Context, q Query) ([]Record, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	rows := []Record{}
	for _, r := range f.rows {
		if matches(r, q) {
			rows = append(rows, r)
		}
	}
	return ordered(rows, q), nil
}
func (f *files) change(r Record, remove bool) error {
	if !ValidKey(r.Kind) || !ValidKey(r.ID) {
		return errors.New("invalid record key")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := r.Kind + "/" + r.ID
	if f.cfg.Driver == "json" {
		next := make(map[string]Record, len(f.rows))
		for k, v := range f.rows {
			next[k] = v
		}
		if remove {
			delete(next, key)
		} else {
			next[key] = r
		}
		b, e := json.Marshal(next)
		if e != nil {
			return e
		}
		if e = Atomic(f.cfg.Address, b); e != nil {
			return e
		}
		f.rows = next
		return nil
	}
	p := filepath.Join(f.cfg.Address, r.Kind, r.ID+".json")
	if remove {
		e := os.Remove(p)
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
		delete(f.rows, key)
		if e == nil {
			return syncDir(filepath.Dir(p))
		}
		return nil
	}
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	if e = Atomic(p, b); e != nil {
		return e
	}
	f.rows[key] = r
	return nil
}
func (f *files) Put(_ context.Context, r Record) error { return f.change(r, false) }
func (f *files) Delete(_ context.Context, k, id string) error {
	return f.change(Record{Kind: k, ID: id}, true)
}
func (f *files) Close() error { return f.lock.Unlock() }
