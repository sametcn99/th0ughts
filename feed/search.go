package feed

import (
	"cmp"
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
	terms [2]string
	tags  []string
}

func indexPost(p Post) searchEntry {
	r := searchEntry{id: p.ID, rank: p.Created, tags: slices.Clone(p.Tags)}
	if p.Pinned {
		r.rank += 1 << 54
	}
	for i, l := range []string{"en", "tr"} {
		_, t := p.Select(l)
		r.terms[i] = cases.Lower(language.Make(l)).String(t.Body + " " + t.Title)
	}
	return r
}

type searchIndex struct {
	entries []searchEntry
	ranks   map[string]int64
}

func compareEntries(a, b searchEntry) int {
	if a.rank != b.rank {
		return cmp.Compare(b.rank, a.rank)
	}
	return strings.Compare(b.id, a.id)
}

func (idx *searchIndex) remove(id string) {
	rank, ok := idx.ranks[id]
	if !ok {
		return
	}
	at, _ := slices.BinarySearchFunc(idx.entries, searchEntry{id: id, rank: rank}, compareEntries)
	idx.entries = slices.Delete(idx.entries, at, at+1)
	delete(idx.ranks, id)
}
func (s *Service) updateIndex(p Post) {
	idx := s.index
	if idx == nil {
		return
	}
	pub, e := p.Public()
	if e != nil {
		idx.remove(p.ID)
		return
	}
	entry := indexPost(pub)
	if rank, ok := idx.ranks[p.ID]; ok && rank != entry.rank {
		idx.remove(p.ID)
	}
	at, found := slices.BinarySearchFunc(idx.entries, entry, compareEntries)
	if found {
		idx.entries[at] = entry
	} else {
		idx.entries = slices.Insert(idx.entries, at, entry)
	}
	idx.ranks[p.ID] = entry.rank
}

func (s *Service) buildIndex(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index != nil {
		return nil
	}
	idx := &searchIndex{ranks: make(map[string]int64)}
	all := storage.Query{Kind: "post", Public: true, Limit: 256}
	for {
		rows, e := s.DB.List(ctx, all)
		if e != nil {
			return e
		}
		for _, r := range rows {
			var p Post
			if e = json.Unmarshal(r.Data, &p); e != nil {
				return e
			}
			if pub, e := p.Public(); e == nil {
				entry := indexPost(pub)
				idx.entries = append(idx.entries, entry)
				idx.ranks[p.ID] = entry.rank
			}
		}
		if len(rows) < all.Limit {
			break
		}
		r := rows[len(rows)-1]
		all.Before = &storage.Cursor{Rank: r.Rank, ID: r.ID}
	}
	slices.SortFunc(idx.entries, compareEntries)
	s.index = idx
	return nil
}
func (s *Service) search(ctx context.Context, f Filter, q storage.Query, limit int) (Page, error) {
	out := Page{Posts: []Post{}}
	s.mu.RLock()
	for s.index == nil {
		s.mu.RUnlock()
		if err := s.buildIndex(ctx); err != nil {
			return out, err
		}
		s.mu.RLock()
	}
	defer s.mu.RUnlock()
	entries := s.index.entries
	if before := q.Before; before != nil {
		pos := sort.Search(len(entries), func(i int) bool {
			return compareEntries(entries[i], searchEntry{id: before.ID, rank: before.Rank}) > 0
		})
		entries = entries[pos:]
	}
	lang := language.English
	column := 0
	if f.Lang == "tr" {
		lang = language.Turkish
		column = 1
	}
	text := cases.Lower(lang).String(f.Search)
	var last storage.Record
	for i, r := range entries {
		if i%256 == 0 && ctx.Err() != nil {
			return out, ctx.Err()
		}
		if !strings.Contains(r.terms[column], text) || f.Tag != "" && !slices.Contains(r.tags, f.Tag) {
			continue
		}
		if len(out.Posts) == limit {
			out.Next = cursor(last)
			break
		}
		p, e := s.Get(ctx, Actor{}, r.id, false)
		if e != nil {
			return out, e
		}
		out.Posts = append(out.Posts, p)
		last = storage.Record{ID: r.id, Rank: r.rank}
	}
	return out, nil
}
