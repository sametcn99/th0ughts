package feed

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/sametcn99/th0ughts/storage"
)

func (m Media) objectKey() string {
	if m.Object != "" {
		return m.Object
	}
	return m.ID
}
func (l *Library) discard(ctx context.Context, m Media) error {
	key := m.objectKey()
	if !storage.ValidKey(key) {
		return Invalid
	}
	if m.Location == "disk" {
		e := os.Remove(filepath.Join(l.Dir, key))
		if errors.Is(e, os.ErrNotExist) {
			return nil
		}
		return e
	}
	client, config, e := l.remote(ctx, m.Location)
	if e != nil {
		return e
	}
	return client.RemoveObject(ctx, config.Bucket, key, minio.RemoveObjectOptions{})
}
func (l *Library) RepairUploads(ctx context.Context) error {
	rows, e := l.Service.DB.List(ctx, storage.Query{Kind: "pending_media"})
	if e != nil {
		return e
	}
	for _, r := range rows {
		var m Media
		if e = json.Unmarshal(r.Data, &m); e != nil {
			return e
		}
		_, e = l.Service.DB.Get(ctx, "media", m.ID)
		if errors.Is(e, storage.ErrMissing) {
			if e = l.discard(ctx, m); e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
		if e = l.Service.DB.Delete(ctx, "pending_media", m.ID); e != nil {
			return e
		}
	}
	return nil
}
