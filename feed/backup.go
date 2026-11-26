package feed

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"

	"github.com/sametcn99/th0ughts/storage"
	theme "github.com/sametcn99/th0ughts/themes"
)

type Manifest struct {
	Version int
	Site    Settings
	Posts   []Post
	Themes  []Version
	Media   []Media
}

func (l *Library) Export(ctx context.Context, a Actor, w io.Writer) error {
	if !a.Recent() {
		return Denied
	}
	z := zip.NewWriter(w)
	m := Manifest{Version: 1}
	var e error
	m.Site, e = l.Service.Settings(ctx)
	if e != nil {
		return e
	}
	catalog := l.Service.Themes
	ids, e := catalog.ids(ctx, m.Site.refs, false)
	if e != nil {
		return e
	}
	for _, id := range ids {
		v, e := catalog.sourceAt(ctx, id, m.Site.refs)
		if e != nil {
			return e
		}
		m.Themes = append(m.Themes, v)
	}
	for _, kind := range []string{"post", "media"} {
		rows, e := l.Service.DB.List(ctx, storage.Query{Kind: kind})
		if e != nil {
			return e
		}
		for _, r := range rows {
			switch kind {
			case "post":
				var p Post
				if e = json.Unmarshal(r.Data, &p); e != nil {
					return e
				}
				m.Posts = append(m.Posts, p)
			case "media":
				med, f, e := l.Open(ctx, a, r.ID)
				if e != nil {
					return e
				}
				dest, e := z.CreateHeader(&zip.FileHeader{Name: "media/" + med.ID, Method: zip.Store})
				if e == nil {
					_, e = io.Copy(dest, f)
				}
				f.Close()
				if e != nil {
					return e
				}
				med.Location = ""
				med.Object = ""
				m.Media = append(m.Media, med)
			}
		}
	}
	dst, e := z.Create("manifest.json")
	if e != nil {
		return e
	}
	if e = json.NewEncoder(dst).Encode(m); e != nil {
		return e
	}
	return z.Close()
}

type importLog struct {
	Posts, Themes, Media []string
	Site                 *siteRecord
}

func (l *Library) RollbackImport(ctx context.Context) error {
	j, e := Read[importLog](ctx, l.Service.DB, "system", "import")
	if errors.Is(e, storage.ErrMissing) {
		return nil
	}
	if e != nil {
		return e
	}
	for kind, ids := range map[string][]string{"post": j.Posts, "theme": j.Themes, "media": j.Media} {
		for _, id := range ids {
			if kind == "media" {
				m, e := Read[Media](ctx, l.Service.DB, "media", id)
				if e == nil {
					if e = l.discard(ctx, m); e != nil {
						return e
					}
				} else if !errors.Is(e, storage.ErrMissing) {
					return e
				}
			}
			if e = l.Service.DB.Delete(ctx, kind, id); e != nil {
				return e
			}
		}
	}
	if j.Site != nil {
		if e = Write(ctx, l.Service.DB, "site", "settings", *j.Site, 0, true); e != nil {
			return e
		}
	}
	l.Service.Themes.clearCache()
	return l.Service.DB.Delete(ctx, "system", "import")
}
func (l *Library) Import(ctx context.Context, a Actor, z *zip.Reader) (err error) {
	if !a.Recent() {
		return Denied
	}
	for _, k := range []string{"post", "media"} {
		r, e := l.Service.DB.List(ctx, storage.Query{Kind: k, Limit: 1})
		if e != nil {
			return e
		}
		if len(r) > 0 {
			return Conflict
		}
	}
	files := map[string]*zip.File{}
	var total uint64
	for _, f := range z.File {
		if len(files) > 100000 || files[f.Name] != nil || f.UncompressedSize64 > 64<<20 {
			return Invalid
		}
		total += f.UncompressedSize64
		if total > 4<<30 {
			return Invalid
		}
		files[f.Name] = f
	}
	mf := files["manifest.json"]
	if mf == nil {
		return Invalid
	}
	r, e := mf.Open()
	if e != nil {
		return e
	}
	var m Manifest
	e = json.NewDecoder(r).Decode(&m)
	r.Close()
	if e != nil || m.Version != 1 {
		return Invalid
	}
	old, e := l.Service.Themes.record(ctx)
	if e != nil {
		return e
	}
	ids, e := l.Service.Themes.ids(ctx, old.Refs, false)
	if e != nil {
		return e
	}
	if len(ids) > 0 {
		return Conflict
	}
	j := importLog{Site: &old}
	seen := map[string]bool{}
	for _, v := range m.Media {
		if !storage.ValidKey(v.ID) || seen[v.ID] || files["media/"+v.ID] == nil {
			return Invalid
		}
		seen[v.ID] = true
		j.Media = append(j.Media, v.ID)
	}
	for _, p := range m.Posts {
		if !storage.ValidKey(p.ID) || seen[p.ID] || p.Created <= 0 || p.Created > 1<<53 || p.Revision < 1 || !Locale(p.Original) {
			return Invalid
		}
		if _, ok := p.Text[p.Original]; !ok {
			return Invalid
		}
		for lang, t := range p.Text {
			if t.Cover != "" && !slices.Contains(p.Attachments(t), t.Cover) {
				return Invalid
			}
			if !Locale(lang) || len(t.Body) > 128000 {
				return Invalid
			}
		}
		for _, id := range p.AllMedia() {
			if !seen[id] || files["media/"+id] == nil {
				return Invalid
			}
		}
		seen[p.ID] = true
		j.Posts = append(j.Posts, p.ID)
	}
	for _, v := range m.Themes {
		t, e := theme.Compile(v.Source)
		if e != nil {
			return Invalid
		}
		if slices.Contains(j.Themes, t.ID) {
			return Invalid
		}
		for _, source := range sources(v)[1:] {
			past, e := theme.Compile(source)
			if e != nil || past.ID != t.ID {
				return Invalid
			}
		}
		j.Themes = append(j.Themes, t.ID)
	}
	if e = Write(ctx, l.Service.DB, "system", "import", j, 0, false); e != nil {
		return e
	}
	defer func() {
		if err != nil {
			if e := l.RollbackImport(context.WithoutCancel(ctx)); e != nil {
				err = errors.Join(err, e)
			}
		}
	}()
	for _, v := range m.Media {
		f, e := files["media/"+v.ID].Open()
		if e != nil {
			return e
		}
		med, e := l.upload(ctx, a, f, v.ID)
		f.Close()
		if e != nil {
			return e
		}
		med.Post = v.Post
		if e = Write(ctx, l.Service.DB, "media", med.ID, med, 0, false); e != nil {
			return e
		}
	}
	for _, p := range m.Posts {
		rank := p.Created
		if p.Pinned {
			rank += 1 << 54
		}
		if e = Write(ctx, l.Service.DB, "post", p.ID, p, rank, p.Visible()); e != nil {
			return e
		}
	}
	for _, v := range m.Themes {
		t, _ := theme.Compile(v.Source)
		if e = Write(ctx, l.Service.DB, "theme", t.ID, v, 0, false); e != nil {
			return e
		}
	}
	if len(old.Refs) > 0 {
		old.Refs = nil
		if e = Write(ctx, l.Service.DB, "site", "settings", old, 0, true); e != nil {
			return e
		}
	}
	l.Service.Themes.clearCache()
	if _, e = NewThemes(l.Service.DB).Load(ctx, m.Site.Theme); e != nil {
		return Invalid
	}
	if e = l.Service.Themes.normalize(ctx, &m.Site); e != nil {
		return e
	}
	if e = l.Service.SaveSettings(ctx, a, m.Site); e != nil {
		return e
	}
	if e = l.Service.DB.Delete(ctx, "system", "import"); e != nil {
		return e
	}
	l.Service.mu.Lock()
	l.Service.index = nil
	l.Service.mu.Unlock()
	return nil
}
