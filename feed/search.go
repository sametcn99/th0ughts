package feed

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"github.com/sametcn99/th0ughts/storage"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type searchEntry struct {
	id    string
	rank  int64
	terms map[string]string
	tags  []string
}

func indexPost(p Post) searchEntry {
	r := searchEntry{id: p.ID, rank: p.Created, terms: map[string]string{}, tags: p.Tags}
	if p.Pinned {
		r.rank += 1 << 54
	}
	for _, l := range []string{"en", "tr"} {
		_, t := p.Select(l)
		r.terms[l] = cases.Lower(language.Make(l)).String(t.Body + " " + t.Title)
	}
	return r
}
func (s *Service) updateIndex(p Post) {
	if s.index == nil {
		return
	}
	pub, e := p.Public()
	if e != nil {
		delete(s.index, p.ID)
		return
	}
	s.index[p.ID] = indexPost(pub)
}
func (s *Service) search(ctx context.Context, f Filter, q storage.Query, limit int) (Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Page{Posts: []Post{}}
	if s.index == nil {
		idx := map[string]searchEntry{}
		all := storage.Query{Kind: "post", Public: true, Limit: 256}
		for {
			rows, e := s.DB.List(ctx, all)
			if e != nil {
				return out, e
			}
			for _, r := range rows {
				var p Post
				if e = json.Unmarshal(r.Data, &p); e != nil {
					return out, e
				}
				pub, e := p.Public()
				if e == nil {
					idx[p.ID] = indexPost(pub)
				}
			}
			if len(rows) < all.Limit {
				break
			}
			r := rows[len(rows)-1]
			all.Before = &storage.Cursor{Rank: r.Rank, ID: r.ID}
		}
		s.index = idx
	}
	l := f.Lang
	if !Locale(l) {
		l = "en"
	}
	text := cases.Lower(language.Make(l)).String(f.Search)
	var hits []searchEntry
	for _, r := range s.index {
		if q.Before != nil && (r.rank > q.Before.Rank || r.rank == q.Before.Rank && r.id >= q.Before.ID) {
			continue
		}
		if text != "" && !strings.Contains(r.terms[l], text) || f.Tag != "" && !slices.Contains(r.tags, f.Tag) {
			continue
		}
		hits = append(hits, r)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].rank == hits[j].rank {
			return hits[i].id > hits[j].id
		}
		return hits[i].rank > hits[j].rank
	})
	if len(hits) > limit {
		r := hits[limit-1]
		out.Next = cursor(storage.Record{ID: r.id, Rank: r.rank})
		hits = hits[:limit]
	}
	for _, hit := range hits {
		p, e := s.Get(ctx, Actor{}, hit.id, false)
		if e != nil {
			return out, e
		}
		out.Posts = append(out.Posts, p)
	}
	return out, nil
}
