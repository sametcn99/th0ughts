package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sametcn99/th0ughts/feed"
	theme "github.com/sametcn99/th0ughts/themes"
)

func (s *Server) postHTML(r *http.Request, p feed.Post, lang string, words map[string]string, site feed.Settings, design theme.Theme, private bool) string {
	l, t := p.Select(lang)
	dt := time.UnixMilli(p.Created).UTC()
	parts := map[string]string{}
	var by strings.Builder
	fmt.Fprintf(&by, "<div class=\"byline\"><span class=\"avatar\" aria-hidden=\"true\">◒</span><strong>%s</strong><a href=\"%s\"><time datetime=\"%s\">%s</time></a>", esc(first(site.Author, site.Name)), esc(link("", p.ID, l)), dt.Format(time.RFC3339), esc(dateLabel(dt, words)))
	if p.Pinned {
		fmt.Fprintf(&by, "<span>%s</span>", esc(words["pinned"]))
	}
	if !t.Published {
		fmt.Fprintf(&by, "<span>%s</span>", esc(words["draft"]))
	}
	parts["byline"] = by.String() + "</div>"
	if t.Title != "" {
		parts["title"] = "<h2>" + esc(t.Title) + "</h2>"
	}
	parts["body"] = "<div class=\"prose\">" + bodyHTML(t.Body) + "</div>"
	var media, tags strings.Builder
	for _, id := range p.Attachments(t) {
		m, e := feed.Read[feed.Media](r.Context(), s.S.DB, "media", id)
		if e != nil {
			continue
		}
		if m.Video() {
			fmt.Fprintf(&media, "<video controls preload=\"metadata\" aria-label=\"%s\" src=\"/media/%s\"></video>", esc(t.Alt[id]), esc(id))
		} else {
			fmt.Fprintf(&media, "<a href=\"/media/%s\"><img class=\"attachment\" loading=\"lazy\" width=\"%d\" height=\"%d\" src=\"/media/%s\" srcset=\"/media/%s?w=480 480w, /media/%s?w=960 960w\" sizes=\"(max-width: 760px) 92vw, 700px\" alt=\"%s\"></a>", esc(id), m.Width, m.Height, esc(id), esc(id), esc(id), esc(t.Alt[id]))
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
	return "<article class=\"entry\" lang=\"" + esc(l) + "\">" + design.RenderLocale("item", parts, lang, map[string]bool{"owner": private, "pinned": p.Pinned, "draft": !t.Published, "media": len(p.Attachments(t)) > 0, "tags": len(p.Tags) > 0, "title": t.Title != ""}) + "</article>"
}
