// Package storage defines the persistent record contract shared by all drivers.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var ErrMissing = errors.New("record not found")

type Config struct {
	Driver  string
	Address string
}
type Record struct {
	Kind      string
	ID        string
	Rank      int64
	Published bool
	Data      json.RawMessage
}
type Query struct {
	Prefix      string
	Unpublished bool
	Kind        string
	Public      bool
	Before      *Cursor
	Limit       int
}
type Cursor struct {
	Rank int64
	ID   string
}
type Store interface {
	Get(context.Context, string, string) (Record, error)
	Put(context.Context, Record) error
	Delete(context.Context, string, string) error
	List(context.Context, Query) ([]Record, error)
	Close() error
}

func Open(ctx context.Context, c Config) (Store, error) {
	switch c.Driver {
	case "json", "json-dir":
		return openFiles(c)
	case "sqlite", "postgres", "mysql", "mariadb":
		return openSQL(ctx, c)
	case "mongodb":
		return openMongo(ctx, c)
	}
	return nil, fmt.Errorf("unsupported driver %q", c.Driver)
}
func matches(r Record, q Query) bool {
	return r.Kind == q.Kind && strings.HasPrefix(r.ID, q.Prefix) && (!q.Unpublished || !r.Published) && (!q.Public || r.Published) && (q.Before == nil || r.Rank < q.Before.Rank || r.Rank == q.Before.Rank && r.ID < q.Before.ID)
}
func ordered(rows []Record, q Query) []Record {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Rank == rows[j].Rank {
			return rows[i].ID > rows[j].ID
		}
		return rows[i].Rank > rows[j].Rank
	})
	if q.Limit > 0 && len(rows) > q.Limit {
		return rows[:q.Limit]
	}
	return rows
}
func ValidKey(s string) bool {
	return s != "" && len(s) <= 128 && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-") == ""
}
