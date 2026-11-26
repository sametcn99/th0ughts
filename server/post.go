package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sametcn99/th0ughts/feed"
	theme "github.com/sametcn99/th0ughts/themes"
)

const timelineTextLimit = 280
const articleExcerptLength = 160

func (s *Server) postHTML(r *http.Request, p feed.Post, lang string, words map[string]string, site feed.Settings, design theme.Theme, private, detail bool, prioritizePhoto *bool) string {
	l, t := p.Select(lang)
	dt := time.UnixMilli(p.Created).UTC()
	parts := map[string]string{}
	author := first(site.Author, site.Name)
	initial, _ := utf8.DecodeRuneInString(strings.TrimSpace(author))
	parts["avatar"] = fmt.Sprintf(`<span class="avatar" aria-hidden="true">%s</span>`, esc(strings.ToUpper(string(initial))))
	parts["author"] = "<strong>" + esc(author) + "</strong>"
	parts["date"] = fmt.Sprintf(`<a href="%s"><time datetime="%s">%s</time></a>`, esc(link("", p.ID, l)), dt.Format(time.RFC3339), esc(dateLabel(dt, words)))
	if p.Pinned {
		parts["status"] += "<span>" + esc(words["pinned"]) + "</span>"
	}
	if !t.Published {
		parts["status"] += "<span>" + esc(words["draft"]) + "</span>"
	}
	parts["byline"] = `<div class="byline">` + parts["avatar"] + parts["author"] + parts["date"] + parts["status"] + "</div>"
	rendered := bodyHTML(t.Body)
	class := "entry"
	attachments := p.Attachments(t)
	permalink := esc(link("", p.ID, lang))
	if detail {
		class += " entry-detail"
	}
	if t.Title != "" {
		if detail {
			parts["title"] = "<h1>" + esc(t.Title) + "</h1>"
		} else {
			parts["title"] = `<h2><a href="` + permalink + `">` + esc(t.Title) + "</a></h2>"
		}
	}
	parts["body"] = `<div class="prose">` + rendered + "</div>"
	visible := plainHTML(rendered)
	article := t.Title != "" || utf8.RuneCountInString(visible) > timelineTextLimit
	if article && !detail {
		class += " article-preview"
		parts["body"] = `<div class="prose"><p>` + esc(excerpt(visible, articleExcerptLength)) + `</p></div>`
		parts["readmore"] = `<a class="read-post" href="` + permalink + `">` + esc(words["read_article"]) + ` <span aria-hidden="true">→</span></a>`
		if len(attachments) > 1 {
			attachments = attachments[:1]
		}
	}
	var media, tags strings.Builder
	for _, id := range attachments {
		m, e := feed.Read[feed.Media](r.Context(), s.S.DB, "media", id)
		if e != nil {
			continue
		}
		if m.Video() {
			fmt.Fprintf(&media, "<video controls preload=\"metadata\" aria-label=\"%s\" src=\"/media/%s\"></video>", esc(t.Alt[id]), esc(id))
		} else {
			loading := `loading="lazy"`
			if *prioritizePhoto {
				loading = `loading="eager" fetchpriority="high"`
				*prioritizePhoto = false
			}
			path := "/media/" + esc(id)
			set := ""
			if m.Width > 0 {
				set = fmt.Sprintf("%s?w=480 %dw", path, min(480, m.Width))
			}
			if m.Width > 480 {
				set += fmt.Sprintf(", %s?w=960 %dw", path, min(960, m.Width))
			}
			fmt.Fprintf(&media, `<a href="%s"><img class="attachment" %s width="%d" height="%d" src="%s" srcset="%s" sizes="(max-width: 760px) 92vw, 700px" alt="%s"></a>`, path, loading, m.Width, m.Height, path, set, esc(t.Alt[id]))
		}
	}
	parts["media"] = media.String()
	for _, tag := range p.Tags {
		fmt.Fprintf(&tags, "<a class=\"tag\" href=\"/?lang=%s&amp;tag=%s\">#%s</a>", lang, url.QueryEscape(tag), esc(tag))
	}
	parts["tags"] = tags.String()
	if private {
		parts["actions"] = fmt.Sprintf("<button class=\"edit\" data-edit=\"%s\">%s</button>", esc(p.ID), esc(words["edit"]))
	}
	if !detail && site.Comments.Enabled() && p.Visible() {
		parts["actions"] += `<a class="thread-link" href="` + permalink + `#comments">` + esc(words["comments"]) + `</a>`
	}
	return "<article class=\"" + class + "\" lang=\"" + esc(l) + "\">" + design.RenderLocale("item", parts, lang, map[string]bool{"article": article, "short": !article, "detail": detail, "list": !detail, "owner": private, "pinned": p.Pinned, "draft": !t.Published, "media": len(p.Attachments(t)) > 0, "tags": len(p.Tags) > 0, "title": t.Title != ""}) + "</article>"
}
