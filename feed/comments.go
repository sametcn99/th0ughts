package feed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/sametcn99/th0ughts/storage"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type CommentPolicy struct {
	RequireEmailCode bool   `json:"requireEmailCode"`
	Mode             string `json:"mode"`
	Markdown         bool   `json:"markdown"`
	Images           int    `json:"images"`
}

func (p CommentPolicy) Enabled() bool { return p.Mode == "review" || p.Mode == "open" }
func (p CommentPolicy) valid() bool {
	return slices.Contains([]string{"", "off", "review", "open"}, p.Mode) && p.Images >= 0 && p.Images <= 4
}

var CommentsClosed = errors.New("comments_closed")

type Comment struct {
	ID      string   `json:"id"`
	Post    string   `json:"post"`
	Parent  string   `json:"parent,omitempty"`
	Name    string   `json:"name,omitempty"`
	Body    string   `json:"body,omitempty"`
	Format  string   `json:"format,omitempty"`
	Images  []string `json:"images,omitempty"`
	Status  string   `json:"status"`
	Depth   int      `json:"depth"`
	Created int64    `json:"created"`
}
type CommentQuery struct {
	Status string `json:"status,omitempty"`
	Post   string `json:"post,omitempty"`
	Parent string `json:"parent,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}
type CommentPage struct {
	Items []Comment `json:"items"`
	Next  string    `json:"next,omitempty"`
}

func branchPrefix(post, parent string) string {
	branch := "root"
	if parent != "" {
		branch = Digest(parent)[:24]
	}
	return Digest(post)[:24] + "_" + branch + "_"
}
func (s *Service) CommentPolicy(ctx context.Context, post string, writing bool) (CommentPolicy, error) {
	site, e := s.Settings(ctx)
	if e != nil {
		return site.Comments, e
	}
	if !site.Comments.Enabled() {
		return site.Comments, CommentsClosed
	}
	p, e := s.Get(ctx, Actor{}, post, false)
	if e != nil {
		return site.Comments, e
	}
	if writing && p.CommentsClosed {
		return site.Comments, CommentsClosed
	}
	return site.Comments, nil
}
func (s *Service) CommentList(ctx context.Context, a Actor, f CommentQuery, queue bool) (CommentPage, error) {
	page := CommentPage{Items: []Comment{}}
	q := storage.Query{Kind: "comment", Limit: 21}
	if queue {
		if !a.Owner {
			return page, Denied
		}
		if f.Status == "published" {
			q.Public = true
		} else if f.Status == "" || f.Status == "pending" {
			q.Unpublished = true
		} else {
			return page, Invalid
		}
	} else {
		if _, e := s.CommentPolicy(ctx, f.Post, false); e != nil {
			return page, e
		}
		q.Public = true
		q.Prefix = branchPrefix(f.Post, f.Parent)
	}
	if f.Cursor != "" {
		position, e := parseCursor(f.Cursor)
		if e != nil || !strings.HasPrefix(position.ID, q.Prefix) {
			return page, Invalid
		}
		q.Before = position
	}
	rows, e := s.DB.List(ctx, q)
	if e != nil {
		return page, e
	}
	if len(rows) > 20 {
		page.Next = cursor(rows[19])
		rows = rows[:20]
	}
	for _, r := range rows {
		var c Comment
		if e = json.Unmarshal(r.Data, &c); e != nil {
			return page, e
		}
		page.Items = append(page.Items, c)
	}
	return page, nil
}

// SubmitVerifiedComment accepts a guest whose mailbox the transport has verified.
// The server supplies all identity, hierarchy and moderation fields.
func (l *Library) SubmitVerifiedComment(ctx context.Context, post, parent, name, body, format string, images [][]byte) (Comment, error) {
	return l.submitComment(ctx, post, parent, name, body, format, images, true)
}

// SubmitDomainCheckedComment accepts a guest address validated by the transport's
// DNS check. Recheck the current policy at commit time in case the owner enabled codes.
func (l *Library) SubmitDomainCheckedComment(ctx context.Context, post, parent, name, body, format string, images [][]byte) (Comment, error) {
	return l.submitComment(ctx, post, parent, name, body, format, images, false)
}

var EmailCodeRequired = errors.New("email_code_required")

func (l *Library) submitComment(ctx context.Context, post, parent, name, body, format string, images [][]byte, mailboxVerified bool) (Comment, error) {
	s := l.Service
	s.mu.Lock()
	defer s.mu.Unlock()
	var c Comment
	policy, e := s.CommentPolicy(ctx, post, true)
	if e != nil {
		return c, e
	}
	if policy.RequireEmailCode && !mailboxVerified {
		return c, EmailCodeRequired
	}
	name = strings.TrimSpace(name)
	body = strings.TrimSpace(body)
	if utf8.RuneCountInString(name) > 80 || utf8.RuneCountInString(body) > 6000 || (body == "" && len(images) == 0) || len(images) > policy.Images || !(format == "text" || format == "markdown" && policy.Markdown) {
		return c, Invalid
	}
	c = Comment{ID: branchPrefix(post, parent) + ID(12), Post: post, Parent: parent, Name: name, Body: body, Format: format, Status: "pending", Created: time.Now().UnixMilli()}
	if parent != "" {
		p, e := Read[Comment](ctx, s.DB, "comment", parent)
		if e != nil || p.Post != post || p.Status != "approved" || p.Depth >= 12 {
			return Comment{}, Invalid
		}
		c.Depth = p.Depth + 1
	}
	for range images {
		c.Images = append(c.Images, ID(16))
	}
	if e = s.trackImages(ctx, c); e != nil {
		return Comment{}, e
	}
	committed := false
	defer func() {
		cleanup := context.WithoutCancel(ctx)
		if len(c.Images) > 0 && (committed || l.removeCommentImages(cleanup, c.Images) == nil) {
			s.DB.Delete(cleanup, "comment_gc", c.ID)
		}
	}()
	for i, b := range images {
		if _, e = l.uploadBound(ctx, Actor{Scopes: []string{"media"}}, bytes.NewReader(b), c.Images[i], c.Post, c.ID); e != nil {
			return Comment{}, e
		}
	}
	// Recheck the current policy after slow object storage writes, while settings publication is excluded.
	s.Themes.mu.Lock()
	defer s.Themes.mu.Unlock()
	rec, e := s.Themes.record(ctx)
	if e != nil {
		return Comment{}, e
	}
	if rec.Comments.RequireEmailCode && !mailboxVerified {
		return Comment{}, EmailCodeRequired
	}
	if !rec.Comments.Enabled() {
		return Comment{}, CommentsClosed
	}
	if len(images) > rec.Comments.Images || format == "markdown" && !rec.Comments.Markdown {
		return Comment{}, Invalid
	}
	if rec.Comments.Mode == "open" {
		c.Status = "approved"
	}
	if e = l.stageCommentEvent(ctx, "comment.created", c); e != nil {
		return Comment{}, e
	}
	if e = Write(ctx, s.DB, "comment", c.ID, c, c.Created, c.Status == "approved"); e != nil {
		return Comment{}, e
	}
	committed = true
	s.Version.Add(1)
	return c, nil
}
func (l *Library) removeCommentImages(ctx context.Context, ids []string) error {
	for _, id := range ids {
		m, e := Read[Media](ctx, l.Service.DB, "media", id)
		if errors.Is(e, storage.ErrMissing) {
			continue
		}
		if e != nil {
			return e
		}
		if e = l.discard(ctx, m); e != nil {
			return e
		}
		if e = l.Service.DB.Delete(ctx, "media", id); e != nil {
			return e
		}
	}
	return nil
}
func (l *Library) Moderate(ctx context.Context, a Actor, id, action string) error {
	if !a.Owner {
		return Denied
	}
	s := l.Service
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := Read[Comment](ctx, s.DB, "comment", id)
	if e != nil {
		return e
	}
	defer s.Version.Add(1)
	switch action {
	case "approve":
		if c.Status == "deleted" {
			return Conflict
		}
		if c.Status == "approved" {
			return nil
		}
		c.Status = "approved"
		if e = l.stageCommentEvent(ctx, "comment.approved", c); e != nil {
			return e
		}
		return Write(ctx, s.DB, "comment", id, c, c.Created, true)
	case "delete":
		if c.Status == "deleted" {
			return nil
		}
		if e = s.trackImages(ctx, c); e != nil {
			return e
		}
		images := c.Images
		children, e := s.DB.List(ctx, storage.Query{Kind: "comment", Prefix: branchPrefix(c.Post, c.ID), Limit: 1})
		if e != nil {
			return e
		}
		c.Status = "deleted"
		c.Body = ""
		c.Name = ""
		c.Format = ""
		c.Images = nil
		if e = l.stageCommentEvent(ctx, "comment.deleted", c); e != nil {
			return e
		}
		if len(children) == 0 {
			e = s.DB.Delete(ctx, "comment", id)
		} else {
			e = Write(ctx, s.DB, "comment", id, c, c.Created, true)
		}
		if e != nil {
			return e
		}
		if len(children) == 0 {
			for parent := c.Parent; parent != ""; {
				ancestor, err := Read[Comment](ctx, s.DB, "comment", parent)
				if err != nil {
					return err
				}
				if ancestor.Status != "deleted" {
					break
				}
				siblings, err := s.DB.List(ctx, storage.Query{Kind: "comment", Prefix: branchPrefix(c.Post, parent), Limit: 1})
				if err != nil {
					return err
				}
				if len(siblings) > 0 {
					break
				}
				if err = s.DB.Delete(ctx, "comment", parent); err != nil {
					return err
				}
				parent = ancestor.Parent
			}
		}
		if e = l.removeCommentImages(ctx, images); e != nil {
			return e
		}
		if len(images) > 0 {
			return s.DB.Delete(ctx, "comment_gc", c.ID)
		}
		return nil
	default:
		return Invalid
	}
}

func (s *Service) trackImages(ctx context.Context, c Comment) error {
	if len(c.Images) == 0 {
		return nil
	}
	return Write(ctx, s.DB, "comment_gc", c.ID, c.Images, 0, false)
}

// Only unfinished operations need recovery, irrespective of the total thread size.
func (l *Library) RepairCommentMedia(ctx context.Context) error {
	s := l.Service
	rows, e := s.DB.List(ctx, storage.Query{Kind: "thread_gc"})
	if e != nil {
		return e
	}
	for _, r := range rows {
		_, e = s.DB.Get(ctx, "post", r.ID)
		if errors.Is(e, storage.ErrMissing) {
			if e = s.purgePostComments(ctx, r.ID, l.removeCommentImages); e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
		if e = s.DB.Delete(ctx, "thread_gc", r.ID); e != nil {
			return e
		}
	}
	rows, e = s.DB.List(ctx, storage.Query{Kind: "comment_gc"})
	if e != nil {
		return e
	}
	for _, r := range rows {
		var ids []string
		if e = json.Unmarshal(r.Data, &ids); e != nil {
			return e
		}
		c, e := Read[Comment](ctx, s.DB, "comment", r.ID)
		if e != nil && !errors.Is(e, storage.ErrMissing) {
			return e
		}
		for _, id := range ids {
			if !slices.Contains(c.Images, id) {
				if e = l.removeCommentImages(ctx, []string{id}); e != nil {
					return e
				}
			}
		}
		if e = s.DB.Delete(ctx, "comment_gc", r.ID); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) HasReplies(ctx context.Context, c Comment) (bool, error) {
	rows, e := s.DB.List(ctx, storage.Query{Kind: "comment", Public: true, Prefix: branchPrefix(c.Post, c.ID), Limit: 1})
	return len(rows) > 0, e
}

func (l *Library) DeletePost(ctx context.Context, a Actor, id string) error {
	return l.Service.deletePost(ctx, a, id, l.removeCommentImages)
}
func (s *Service) purgePostComments(ctx context.Context, post string, discard func(context.Context, []string) error) error {
	q := storage.Query{Kind: "comment", Prefix: Digest(post)[:24] + "_", Limit: 100}
	for {
		rows, e := s.DB.List(ctx, q)
		if e != nil {
			return e
		}
		for _, r := range rows {
			var c Comment
			if e = json.Unmarshal(r.Data, &c); e != nil {
				return e
			}
			if discard != nil {
				if e = discard(ctx, c.Images); e != nil {
					return e
				}
			} else if e = s.trackImages(ctx, c); e != nil {
				return e
			}
			if e = s.DB.Delete(ctx, "comment", c.ID); e != nil {
				return e
			}
		}
		if len(rows) < q.Limit {
			return nil
		}
	}
}

func (s *Service) LockThread(ctx context.Context, a Actor, id string, closed bool) error {
	if !a.Owner {
		return Denied
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, e := s.DB.Get(ctx, "post", id)
	if e != nil {
		return e
	}
	var p Post
	if e = json.Unmarshal(r.Data, &p); e != nil {
		return e
	}
	if p.CommentsClosed == closed {
		return nil
	}
	p.CommentsClosed = closed
	p.Revision++
	p.Updated = time.Now().UnixMilli()
	if e = Write(ctx, s.DB, "post", p.ID, p, r.Rank, r.Published); e != nil {
		return e
	}
	s.updateIndex(p)
	s.Version.Add(1)
	return nil
}
