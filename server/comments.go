package server

import (
	"bytes"
	"context"
	"github.com/microcosm-cc/bluemonday"
	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/storage"
	"golang.org/x/image/draw"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"slices"
)

const commentImageLimit = 4 << 20

var imageSlots = make(chan struct{}, 2)

func guestImage(ctx context.Context, r io.Reader) ([]byte, error) {
	select {
	case imageSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-imageSlots }()
	b, e := io.ReadAll(io.LimitReader(r, commentImageLimit+1))
	if e != nil || len(b) > commentImageLimit {
		return nil, feed.Invalid
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || !(format == "jpeg" || format == "png" || format == "webp") || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 12_000_000 {
		return nil, feed.Invalid
	}
	img, _, e := image.Decode(bytes.NewReader(b))
	if e != nil {
		return nil, feed.Invalid
	}
	longest := max(cfg.Width, cfg.Height, 1600)
	dst := image.NewRGBA(image.Rect(0, 0, max(1, cfg.Width*1600/longest), max(1, cfg.Height*1600/longest)))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	var out bytes.Buffer
	e = jpeg.Encode(&out, dst, &jpeg.Options{Quality: 84})
	return out.Bytes(), e
}
func (s *Server) submitComment(w http.ResponseWriter, r *http.Request) {
	post := r.PathValue("post")
	ctx := r.Context()
	p, e := s.S.CommentPolicy(ctx, post, true)
	if e != nil {
		fail(w, e)
		return
	}
	submit := s.L.SubmitDomainCheckedComment
	done := false
	if p.RequireEmailCode {
		finish, err := s.guest.reserve(r.Header.Get("X-Comment-Challenge"), post, r.Header.Get("X-Comment-Code"))
		if err != nil {
			fail(w, err)
			return
		}
		defer func() { finish(done) }()
		submit = s.L.SubmitVerifiedComment
	} else if e = s.guest.allowRequest(r.RemoteAddr); e != nil {
		fail(w, e)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(p.Images*commentImageLimit+65536))
	// Spool large file parts to disk and validate the email before decoding images.
	if e = r.ParseMultipartForm(64 << 10); e != nil {
		fail(w, feed.Invalid)
		return
	}
	form := r.MultipartForm
	defer form.RemoveAll()
	for key, values := range form.Value {
		if len(values) != 1 || len(values[0]) > 24000 || !(key == "name" || key == "format" || key == "body" || key == "parent" || key == "email" && !p.RequireEmailCode) {
			fail(w, feed.Invalid)
			return
		}
	}
	files := form.File["image"]
	if len(form.File) > 0 && (len(form.File) != 1 || len(files) == 0) || len(files) > p.Images {
		fail(w, feed.Invalid)
		return
	}
	if !p.RequireEmailCode {
		address, err := mailbox(r.PostFormValue("email"))
		if err == nil {
			err = checkMailDomain(ctx, address, s.guest.resolver)
		}
		if err != nil {
			fail(w, err)
			return
		}
	}
	var images [][]byte
	for _, file := range files {
		if file.Size > commentImageLimit {
			fail(w, feed.Invalid)
			return
		}
		f, err := file.Open()
		if err != nil {
			fail(w, err)
			return
		}
		b, err := guestImage(ctx, f)
		f.Close()
		if err != nil {
			fail(w, err)
			return
		}
		images = append(images, b)
	}
	c, e := submit(ctx, post, r.PostFormValue("parent"), r.PostFormValue("name"), r.PostFormValue("body"), first(r.PostFormValue("format"), "text"), images)
	if e != nil {
		fail(w, e)
		return
	}
	done = true
	respond(w, c)
}

type threadItem struct {
	feed.Comment
	HTML       string `json:"html,omitempty"`
	HasReplies bool   `json:"hasReplies,omitempty"`
}

type threadPageResult struct {
	Items []threadItem `json:"items"`
	Next  string       `json:"next"`
}

func (s *Server) threadPage(ctx context.Context, a feed.Actor, q feed.CommentQuery, queue bool) (*threadPageResult, error) {
	page, e := s.S.CommentList(ctx, a, q, queue)
	if e != nil {
		return nil, e
	}
	items := make([]threadItem, 0, len(page.Items))
	for _, c := range page.Items {
		item := threadItem{Comment: c}
		if c.Format == "markdown" {
			item.HTML = commentMarkdown(c.Body)
		}
		if !queue {
			children, e := s.S.HasReplies(ctx, c)
			if e != nil {
				return nil, e
			}
			item.HasReplies = children
		}
		items = append(items, item)
	}
	return &threadPageResult{items, page.Next}, nil
}

// Comment Markdown allows formatted text and links. Images use the moderated attachment path.
var commentCleaner = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "strong", "em", "a", "ul", "ol", "li", "blockquote", "pre", "code", "h1", "h2", "h3", "h4", "h5", "h6", "hr")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("https", "http", "mailto")
	p.RequireNoFollowOnLinks(true)
	return p
}()

func commentMarkdown(src string) string {
	var b bytes.Buffer
	markdown.Convert([]byte(src), &b)
	return commentCleaner.Sanitize(b.String())
}

func (s *Server) commentContext(ctx context.Context, id string) ([]threadItem, error) {
	var trail []threadItem
	for id != "" && len(trail) < 13 {
		c, e := feed.Read[feed.Comment](ctx, s.S.DB, "comment", id)
		if e != nil {
			return nil, e
		}
		if c.Status != "approved" && c.Status != "deleted" {
			return nil, storage.ErrMissing
		}
		if len(trail) == 0 {
			if _, e = s.S.CommentPolicy(ctx, c.Post, false); e != nil {
				return nil, e
			}
		} else if c.Post != trail[0].Post {
			return nil, storage.ErrMissing
		}
		item := threadItem{Comment: c}
		if c.Format == "markdown" {
			item.HTML = commentMarkdown(c.Body)
		}
		item.HasReplies, e = s.S.HasReplies(ctx, c)
		if e != nil {
			return nil, e
		}
		trail = append(trail, item)
		id = c.Parent
	}
	if id != "" {
		return nil, feed.Invalid
	}
	slices.Reverse(trail)
	return trail, nil
}
