package feed

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sametcn99/th0ughts/storage"
	_ "golang.org/x/image/webp"
)

const MaxMedia = 32 << 20

type Media struct {
	Comment  string `json:"comment,omitempty"`
	Object   string `json:"object,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	Post     string `json:"post"`
	Location string `json:"location"`
}
type S3 struct {
	Endpoint string `json:"endpoint"`
	Bucket   string `json:"bucket"`
	Access   string `json:"access"`
	Secret   string `json:"secret"`
	TLS      bool   `json:"tls"`
}
type Library struct {
	Service *Service
	Dir     string
	Vault   Vault
}

func (l *Library) Configure(ctx context.Context, a Actor, c S3) error {
	if !a.Recent() {
		return Denied
	}
	id := "disk"
	if c.Endpoint != "" {
		client, e := s3client(c)
		if e != nil {
			return Invalid
		}
		ok, e := client.BucketExists(ctx, c.Bucket)
		if e != nil || !ok {
			return Invalid
		}
		id = ID(12)
		b, _ := json.Marshal(c)
		if e = Write(ctx, l.Service.DB, "location", id, l.Vault.Seal("s3", b), 0, false); e != nil {
			return e
		}
	}
	return Write(ctx, l.Service.DB, "config", "media", id, 0, false)
}
func s3client(c S3) (*minio.Client, error) {
	return minio.New(c.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(c.Access, c.Secret, ""), Secure: c.TLS})
}
func (l *Library) remote(ctx context.Context, id string) (*minio.Client, S3, error) {
	var c S3
	b, e := Read[[]byte](ctx, l.Service.DB, "location", id)
	if e != nil {
		return nil, c, e
	}
	raw, e := l.Vault.Open("s3", b)
	if e != nil {
		return nil, c, e
	}
	if e = json.Unmarshal(raw, &c); e != nil {
		return nil, c, e
	}
	m, e := s3client(c)
	return m, c, e
}
func (l *Library) Upload(ctx context.Context, a Actor, r io.Reader) (Media, error) {
	return l.upload(ctx, a, r, ID(16))
}
func (l *Library) upload(ctx context.Context, a Actor, r io.Reader, id string) (Media, error) {
	return l.uploadBound(ctx, a, r, id, "", "")
}
func (l *Library) uploadBound(ctx context.Context, a Actor, r io.Reader, id, post, comment string) (Media, error) {
	var m Media
	if !a.Can("media") {
		return m, Denied
	}
	if !storage.ValidKey(id) {
		return m, Invalid
	}
	if e := os.MkdirAll(l.Dir, 0700); e != nil {
		return m, e
	}
	f, e := os.CreateTemp(l.Dir, ".upload-")
	if e != nil {
		return m, e
	}
	defer os.Remove(f.Name())
	defer f.Close()
	n, e := io.Copy(f, io.LimitReader(r, MaxMedia+1))
	if e != nil {
		return m, e
	}
	if n < 1 || n > MaxMedia {
		return m, Invalid
	}
	head := make([]byte, 512)
	k, _ := f.ReadAt(head, 0)
	typ := http.DetectContentType(head[:k])
	if !slices.Contains([]string{"image/jpeg", "image/webp", "image/png", "image/gif", "video/mp4", "video/webm"}, typ) {
		return m, Invalid
	}
	if _, e = f.Seek(0, 0); e != nil {
		return m, e
	}
	loc, e := Read[string](ctx, l.Service.DB, "config", "media")
	if errors.Is(e, storage.ErrMissing) {
		loc = "disk"
	} else if e != nil {
		return m, e
	}
	m = Media{Comment: comment, Post: post, ID: id, Object: ID(16), Type: typ, Size: n, Location: loc}
	if strings.HasPrefix(typ, "image/") {
		cfg, _, e := image.DecodeConfig(f)
		if e != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 16384 || cfg.Height > 16384 || int64(cfg.Width)*int64(cfg.Height) > 64000000 {
			return m, Invalid
		}
		m.Width = cfg.Width
		m.Height = cfg.Height
		if _, e = f.Seek(0, 0); e != nil {
			return m, e
		}
	}
	if e = Write(ctx, l.Service.DB, "pending_media", m.ID, m, 0, false); e != nil {
		return m, e
	}
	if loc == "disk" {
		if e = f.Sync(); e != nil {
			return m, e
		}
		if e = os.Rename(f.Name(), filepath.Join(l.Dir, m.objectKey())); e != nil {
			return m, e
		}
	} else {
		client, c, e := l.remote(ctx, loc)
		if e != nil {
			return m, e
		}
		_, e = client.PutObject(ctx, c.Bucket, m.objectKey(), f, n, minio.PutObjectOptions{ContentType: typ})
		if e != nil {
			return m, e
		}
	}
	if e = Write(ctx, l.Service.DB, "media", id, m, 0, false); e != nil {
		return m, e
	}
	return m, l.Service.DB.Delete(ctx, "pending_media", id)
}

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

func (l *Library) Open(ctx context.Context, a Actor, id string) (Media, ReadSeekCloser, error) {
	m, e := Read[Media](ctx, l.Service.DB, "media", id)
	if e != nil {
		return m, nil, e
	}
	if m.Comment != "" && !a.Owner {
		c, e := Read[Comment](ctx, l.Service.DB, "comment", m.Comment)
		if e != nil || c.Status != "approved" || !slices.Contains(c.Images, id) || c.Post != m.Post {
			return m, nil, storage.ErrMissing
		}
		if _, e = l.Service.CommentPolicy(ctx, c.Post, false); e != nil {
			return m, nil, storage.ErrMissing
		}
	} else if m.Comment == "" && !a.Can("media") && !a.Can("posts:read") {
		p, e := l.Service.Get(ctx, Actor{}, m.Post, false)
		if e != nil || !slices.Contains(p.AllMedia(), id) {
			return m, nil, storage.ErrMissing
		}
	}
	if m.Location == "disk" {
		if !storage.ValidKey(id) {
			return m, nil, Invalid
		}
		f, e := os.Open(filepath.Join(l.Dir, m.objectKey()))
		return m, f, e
	}
	client, c, e := l.remote(ctx, m.Location)
	if e != nil {
		return m, nil, e
	}
	obj, e := client.GetObject(ctx, c.Bucket, m.objectKey(), minio.GetObjectOptions{})
	return m, obj, e
}
func (m Media) Video() bool { return strings.HasPrefix(m.Type, "video/") }
