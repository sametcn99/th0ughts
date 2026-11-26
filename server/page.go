package server

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"slices"

	"regexp"

	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	"github.com/sametcn99/th0ughts/feed"
	"github.com/sametcn99/th0ughts/storage"
	theme "github.com/sametcn99/th0ughts/themes"
	"github.com/yuin/goldmark"
)

var esc = html.EscapeString
var cleaner = bluemonday.UGCPolicy()
var markdown = goldmark.New()
var markup = regexp.MustCompile("<[^>]*>")

func bodyHTML(src string) string {
	var buf bytes.Buffer
	markdown.Convert([]byte(src), &buf)
	return cleaner.Sanitize(buf.String())
}
func plain(src string, n int) string {
	return excerpt(plainHTML(bodyHTML(src)), n)
}
func plainHTML(rendered string) string {
	return strings.Join(strings.Fields(html.UnescapeString(markup.ReplaceAllString(rendered, " "))), " ")
}
func excerpt(s string, n int) string {
	if utf8.RuneCountInString(s) > n {
		return string([]rune(s)[:n]) + "…"
	}
	return s
}
func jsonText(x any) string { b, _ := json.Marshal(x); return string(b) }
func (h *Server) page(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	site, e := h.S.Settings(ctx)
	if e != nil {
		fail(w, e)
		return
	}
	lang := h.locale(r, site)
	q := r.URL.Query()
	if feed.Locale(q.Get("lang")) {
		h.setCookie(w, r, "lang", lang, 31536000)
	}
	words := h.catalog[lang]
	a := h.actor(r)
	base := h.origin(r)
	if r.URL.Path == "/robots.txt" {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "User-agent: *\nDisallow: /login\nDisallow: /auth/\nSitemap: %s/sitemap.xml\n", base)
		return
	}
	if r.URL.Path == "/sitemap.xml" {
		h.sitemap(w, r, base)
		return
	}
	id := q.Get("post")
	isMD := r.URL.Path == "/feed.md" || strings.HasPrefix(r.URL.Path, "/post/") || strings.Contains(r.Header.Get("Accept"), "text/markdown")
	if strings.HasPrefix(r.URL.Path, "/post/") {
		if !strings.HasSuffix(r.PathValue("id"), ".md") {
			fail(w, storage.ErrMissing)
			return
		}
		id = strings.TrimSuffix(r.PathValue("id"), ".md")
	}
	login := r.URL.Path == "/login"
	rss := r.URL.Path == "/feed.xml"
	private := a.Owner && !login && !rss && !isMD && q.Get("view") != "public"
	preview := a.Owner && q.Has("preview")
	screen := ""
	if preview {
		screen = q.Get("screen")
	}
	sample := preview && !login && (q.Get("sample") == "1" || screen == "short" || screen == "article" || screen == "article-list")
	if screen == "composer" || screen == "settings" {
		private = true
	}
	page := feed.Page{}
	f := feed.Filter{Search: q.Get("q"), Tag: q.Get("tag"), Cursor: q.Get("cursor"), Lang: lang, Private: private, Limit: 20}
	if sample {
		title := words["sample_title"]
		if screen == "short" {
			title = ""
		}
		now := time.Now().UnixMilli()
		page.Posts = []feed.Post{{ID: "sample", Original: lang, Created: now, Updated: now, Text: map[string]feed.Translation{lang: {Body: words["sample_body"], Title: title, Published: true}}, Tags: []string{"th0ughts"}}}
		if screen == "short" || screen == "article" {
			id = "sample"
		}
	} else if !login {
		if id != "" {
			p, e := h.S.Get(ctx, a, id, private)
			if e != nil {
				fail(w, e)
				return
			}
			page.Posts = []feed.Post{p}
		} else {
			page, e = h.S.List(ctx, a, f)
			if e != nil {
				fail(w, e)
				return
			}
		}
	}
	w.Header().Set("Vary", "Accept, Cookie, Authorization")
	w.Header().Set("Content-Language", lang)
	if isMD && !login {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		fmt.Fprintf(w, "# %s\n\n", site.Name)
		for _, p := range page.Posts {
			l, t := p.Select(lang)
			fmt.Fprintf(w, "## [%s](%s)\n\n%s\n\n%s\n\n", time.UnixMilli(p.Created).UTC().Format(time.RFC3339), link(base, p.ID, l), t.Title, t.Body)
			for _, m := range p.Attachments(t) {
				fmt.Fprintf(w, "[%s](%s/media/%s)\n", strings.ReplaceAll(t.Alt[m], "]", ""), base, m)
			}
		}
		if page.Next != "" {
			q.Set("cursor", page.Next)
			fmt.Fprintf(w, "[%s](%s/feed.md?%s)\n", words["older"], base, q.Encode())
		}
		return
	}
	if rss {
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		fmt.Fprintf(w, "<rss version=\"2.0\"><channel><title>%s</title><link>%s</link><description>%s</description><language>%s</language>", esc(site.Name), esc(base), esc(site.Description), lang)
		for _, p := range page.Posts {
			l, t := p.Select(lang)
			title := t.Title
			if title == "" {
				title = plain(t.Body, 80)
			}
			u := esc(link(base, p.ID, l))
			fmt.Fprintf(w, "<item><guid>%s</guid><link>%s</link><title>%s</title><description>%s</description><pubDate>%s</pubDate></item>", u, u, esc(title), esc(bodyHTML(t.Body)), time.UnixMilli(p.Created).UTC().Format(time.RFC1123Z))
		}
		fmt.Fprint(w, "</channel></rss>")
		return
	}
	themeID := site.Theme
	if preview {
		themeID = q.Get("preview")
	}
	var design theme.Theme
	if a.Owner && q.Has("recover") {
		design = theme.Theme{ID: "recovery"}
	} else if preview && q.Has("draft") {
		value, ok := h.ticket(q.Get("draft"), "preview:"+a.Session, false)
		if !ok {
			fail(w, storage.ErrMissing)
			return
		}
		design = value.(theme.Theme)
	} else {
		design, e = h.T.LoadAt(ctx, themeID, site)
		if e != nil {
			fail(w, e)
			return
		}
	}
	title, description, cover := site.Name, site.Description, site.Cover
	canonical := link(base, id, lang)
	ogType := "website"
	seoLang := lang
	altlangs := []string{"en", "tr"}
	if id != "" && len(page.Posts) > 0 {
		p := page.Posts[0]
		l, t := p.Select(lang)
		canonical = link(base, p.ID, l)
		seoLang = l
		title = t.Title
		if title == "" {
			title = plain(t.Body, 72)
		}
		description = t.Description
		if description == "" {
			description = plain(t.Body, 180)
		}
		ogType = "article"
		altlangs = nil
		for k, tr := range p.Text {
			if tr.Published {
				altlangs = append(altlangs, k)
			}
		}
		sort.Strings(altlangs)
		candidates := p.Attachments(t)
		if t.Cover != "" {
			candidates = append([]string{t.Cover}, candidates...)
		}
		for _, mid := range candidates {
			m, err := feed.Read[feed.Media](ctx, h.S.DB, "media", mid)
			if err == nil && !m.Video() {
				cover = base + "/media/" + mid
				break
			}
		}
	}
	if id == "" && !login {
		canonical = listingURL(base, lang, f)
	}
	var head strings.Builder
	fmt.Fprintf(&head, "<!doctype html><html lang=\"%s\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><meta name=\"color-scheme\" content=\"light dark\"><title>%s</title>", lang, esc(title))
	meta := func(k, v string) {
		attribute := "property"
		if strings.HasPrefix(k, "twitter:") {
			attribute = "name"
		}
		fmt.Fprintf(&head, "<meta %s=\"%s\" content=\"%s\">", attribute, k, esc(v))
	}
	if login || preview || private || q.Get("q") != "" {
		head.WriteString("<meta name=\"robots\" content=\"noindex,nofollow\">")
	}
	if cover == "" {
		cover = base + "/social.png"
	}
	if strings.HasPrefix(cover, "/") {
		cover = base + cover
	}
	if !login {
		fmt.Fprintf(&head, "<link rel=\"canonical\" href=\"%s\"><meta name=\"description\" content=\"%s\">", esc(canonical), esc(description))
		for _, l := range altlangs {
			fmt.Fprintf(&head, "<link rel=\"alternate\" hreflang=\"%s\" href=\"%s\">", l, esc(alternateURL(base, id, l, f)))
		}
		fmt.Fprintf(&head, "<link rel=\"alternate\" hreflang=\"x-default\" href=\"%s\">", esc(alternateURL(base, id, firstAvailable(altlangs, site.Lang, seoLang), f)))
		for _, l := range altlangs {
			if l != seoLang {
				meta("og:locale:alternate", map[string]string{"en": "en_US", "tr": "tr_TR"}[l])
			}
		}
		if id != "" {
			meta("article:published_time", time.UnixMilli(page.Posts[0].Created).UTC().Format(time.RFC3339))
			meta("article:modified_time", time.UnixMilli(page.Posts[0].Updated).UTC().Format(time.RFC3339))
		}
		meta("og:type", ogType)
		meta("og:title", title)
		meta("og:description", description)
		meta("og:url", canonical)
		meta("og:site_name", site.Name)
		meta("og:locale", map[string]string{"en": "en_US", "tr": "tr_TR"}[seoLang])
		meta("twitter:card", "summary_large_image")
		meta("twitter:title", title)
		meta("twitter:description", description)
		if cover != "" {
			meta("og:image", cover)
			meta("twitter:image", cover)
		}
		structured := map[string]any{"@context": "https://schema.org", "@type": "CollectionPage", "name": title, "url": canonical, "inLanguage": seoLang, "description": description}
		if id != "" {
			structured["@type"] = "SocialMediaPosting"
			structured["datePublished"] = time.UnixMilli(page.Posts[0].Created).UTC().Format(time.RFC3339)
			structured["headline"] = title
			structured["author"] = map[string]string{"@type": "Person", "name": site.Author}
		}
		fmt.Fprintf(&head, "<script type=\"application/ld+json\">%s</script><link rel=\"alternate\" type=\"application/rss+xml\" href=\"/feed.xml?lang=%s\">", jsonText(structured), lang)
	}
	head.WriteString("<link rel=\"stylesheet\" href=\"/assets/app.css\">")
	nonce := feed.ID(16)
	w.Header().Set("Content-Security-Policy", strings.Replace(w.Header().Get("Content-Security-Policy"), "style-src 'self'", "style-src 'self' 'nonce-"+nonce+"'", 1))
	fmt.Fprintf(&head, "<style nonce=\"%s\">%s", nonce, design.Styles)
	for _, o := range design.Options {
		v := site.Options[design.ID][o.Key]
		if preview && q.Has("option."+o.Key) {
			v = q.Get("option." + o.Key)
		}
		v = o.CSS(v)
		fmt.Fprintf(&head, ":root{--%s:%s}", o.Key, v)
	}
	head.WriteString("</style></head><body>")
	var banner strings.Builder
	fmt.Fprintf(&banner, `<div class="mast"><a class="wordmark" href="/?lang=%s"><span class="mark" aria-hidden="true">◒</span>%s</a><details class="language-picker"><summary aria-label="%s · %s">%s</summary><nav aria-label="%s">`, lang, esc(site.Name), esc(words["language"]), esc(words["lang_"+lang]), esc(words["lang_"+lang]), esc(words["language"]))
	for _, l := range []string{"en", "tr"} {
		v := r.URL.Query()
		v.Set("lang", l)
		fmt.Fprintf(&banner, `<a lang="%s" href="?%s" aria-current="%t">%s</a>`, l, esc(v.Encode()), l == lang, esc(words["lang_"+l]))
	}
	banner.WriteString("</nav></details></div>")
	var posts strings.Builder
	posts.WriteString("<div class=\"entries\">")
	for _, p := range page.Posts {
		posts.WriteString(h.postHTML(r, p, lang, words, site, design, private, id != ""))
	}
	if len(page.Posts) == 0 {
		fmt.Fprintf(&posts, "<div class=\"empty\"><span aria-hidden=\"true\">◌</span><p>%s</p></div>", esc(words["empty"]))
	}
	posts.WriteString("</div>")
	parts := map[string]string{"header": banner.String(), "posts": posts.String()}
	parts["filters"] = fmt.Sprintf("<section class=\"intro\"><div><h1>%s</h1><p>%s</p></div><a class=\"rss\" href=\"/feed.xml?lang=%s\">RSS ↗</a></section><form class=\"search\" action=\"/\"><input type=hidden name=lang value=\"%s\"><input name=q type=search aria-label=\"%s\" placeholder=\"%s\" value=\"%s\"><button>%s</button></form>", esc(words["timeline"]), esc(site.Description), lang, lang, esc(words["search"]), esc(words["search_hint"]), esc(f.Search), esc(words["search"]))
	if page.Next != "" {
		q.Set("cursor", page.Next)
		parts["pagination"] = "<a class=\"older\" href=\"?" + esc(q.Encode()) + "\">" + esc(words["older"]) + " →</a>"
	}
	view := "feed"
	if id != "" {
		view = "post"
	}
	if login {
		view = "login"
		parts["login"] = "<section id=\"login\" class=\"secure-slot\"></section>"
	}
	_, setup := h.ticket(cookie(r, "bootstrap"), "setup", false)
	if private || setup {
		parts["composer"] = "<section id=\"composer\" class=\"secure-slot\"></section>"
		parts["settings"] = "<section id=\"settings\" class=\"secure-slot\"></section>"
	}
	if preview {
		for _, name := range []string{"composer", "settings", "login"} {
			parts[name] = ""
		}
		control := func(name string) string {
			markup, _ := assets.ReadFile("preview/" + lang + "-" + name + ".html")
			return `<section inert id="` + name + `" class="secure-slot"><template shadowrootmode="open"><link rel="stylesheet" href="` + assetURL("panel.css") + `">` + string(markup) + `</template></section>`
		}
		if login {
			parts["login"] = control("login")
		}
		if private && !login {
			parts["composer"] = control("composer")
			if screen == "settings" {
				parts["settings"] = control("settings")
			} else {
				parts["settings"] = control("launcher")
			}
		}
		if screen == "composer" || screen == "settings" {
			for _, k := range []string{"filters", "posts", "pagination"} {
				parts[k] = ""
			}
			if screen == "settings" {
				parts["composer"] = ""
			}
		}
	}
	var thread *threadPageResult
	var readonly bool
	if view == "post" && len(page.Posts) > 0 && (site.Comments.Enabled() || preview) && page.Posts[0].Visible() {
		if preview {
			parts["comments"] = `<section class="comment-thread" inert><h2>` + esc(words["comments"]) + `</h2><article class="thread-node"><strong>` + esc(words["unknown_user"]) + `</strong><p>` + esc(words["sample_comment"]) + `</p></article></section>`
		} else {
			thread, e = h.threadPage(ctx, a, feed.CommentQuery{Post: id}, false)
			if e != nil {
				fail(w, e)
				return
			}
			relay, e := h.relay(ctx)
			if e != nil {
				fail(w, e)
				return
			}
			readonly = page.Posts[0].CommentsClosed || relay.Host == ""
			var fallback strings.Builder
			fallback.WriteString(`<section id="comments" class="comment-thread"><h2>` + esc(words["comments"]) + "</h2>")
			for _, c := range thread.Items {
				fallback.WriteString(`<article class="thread-node">`)
				if c.Status == "deleted" {
					fallback.WriteString(esc(words["deleted_comment"]))
				} else {
					fallback.WriteString("<strong>" + esc(first(c.Name, words["unknown_user"])) + "</strong>")
					if c.Format == "markdown" {
						fallback.WriteString(c.HTML)
					} else {
						fallback.WriteString("<p>" + esc(c.Body) + "</p>")
					}
				}
				fallback.WriteString("</article>")
			}
			parts["comments"] = fallback.String() + "</section>"
		}
	}
	out := head.String() + "<div class=\"themed\">" + design.RenderLocale(view, parts, lang, map[string]bool{"owner": private, "detail": id != "", "list": id == "" && !login}) + "</div>"
	out += `<footer class="site-footer"><a href="https://github.com/sametcn99/th0ughts">` + esc(words["host_your_own"]) + `</a></footer>`
	tools := []Command{}
	for _, op := range commands {
		if visible(a, op) {
			tools = append(tools, op)
		}
	}
	if !preview {
		_, acctErr := h.A.Account(ctx)
		data := map[string]any{"site": site, "lang": lang, "owner": private, "setup": setup, "newSite": errors.Is(acctErr, storage.ErrMissing), "messages": words, "tools": tools}
		if thread != nil {
			data["thread"] = thread
			data["threadPost"] = id
			data["threadReadonly"] = readonly
		}
		if private || setup {
			data["driver"] = h.cfg.Driver
		}
		out += "<script id=\"state\" type=\"application/json\">" + jsonText(data) + "</script><script src=\"/assets/reader.js\" defer></script>"
		if private || setup || login {
			out += "<script type=\"module\" src=\"/assets/owner.js\"></script>"
		} else if thread != nil {
			out += `<script type="module" src="/assets/thread.js"></script>`
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	for _, name := range []string{"app.css", "reader.js", "owner.js", "thread.js"} {
		out = strings.ReplaceAll(out, "/assets/"+name+"\"", assetURL(name)+"\"")
	}
	fmt.Fprint(w, out+"</body></html>")
}
func first(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}
func dateLabel(dt time.Time, words map[string]string) string {
	return strings.NewReplacer("{day}", strconv.Itoa(dt.Day()), "{year}", strconv.Itoa(dt.Year()), "{month}", words["month_"+strconv.Itoa(int(dt.Month()))]).Replace(words["date_format"])
}
func (s *Server) sitemap(w http.ResponseWriter, r *http.Request, base string) {
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprint(w, "<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">")
	write := func(id, l string) {
		fmt.Fprint(w, "<url><loc>")
		xml.EscapeText(w, []byte(link(base, id, l)))
		fmt.Fprint(w, "</loc></url>")
	}
	write("", "en")
	write("", "tr")
	f := feed.Filter{Limit: 100}
	for {
		p, e := s.S.List(r.Context(), feed.Actor{}, f)
		if e != nil {
			return
		}
		for _, v := range p.Posts {
			for l := range v.Text {
				write(v.ID, l)
			}
		}
		if p.Next == "" {
			break
		}
		f.Cursor = p.Next
	}
	fmt.Fprint(w, "</urlset>")
}

func listingURL(base, lang string, f feed.Filter) string {
	q := url.Values{"lang": {lang}}
	if f.Tag != "" {
		q.Set("tag", f.Tag)
	}
	if f.Cursor != "" {
		q.Set("cursor", f.Cursor)
	}
	return base + "/?" + q.Encode()
}
func alternateURL(base, id, lang string, f feed.Filter) string {
	if id != "" {
		return link(base, id, lang)
	}
	return listingURL(base, lang, f)
}
func firstAvailable(locales []string, requested, fallback string) string {
	if slices.Contains(locales, requested) {
		return requested
	}
	return fallback
}
