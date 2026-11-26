// Package theme compiles single-file themes into a small, data-only layout.
package theme

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"github.com/sametcn99/th0ughts/storage"
	"golang.org/x/net/html"
)

//go:embed *.html
var builtins embed.FS

type Option struct {
	Type    string   `json:"type,omitempty"`
	Min     float64  `json:"min,omitempty"`
	Max     float64  `json:"max,omitempty"`
	Unit    string   `json:"unit,omitempty"`
	Choices []Choice `json:"choices,omitempty"`

	Key     string            `json:"key"`
	Labels  map[string]string `json:"labels"`
	Default string            `json:"default"`
}
type Theme struct {
	Missing []string          `json:"missing,omitempty"`
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Styles  string            `json:"-"`
	Layouts map[string]string `json:"-"`
	Options []Option          `json:"options"`
}

var invalid = errors.New("invalid_theme")
var classes = regexp.MustCompile("^[a-zA-Z0-9 _-]{1,120}$")
var selector = regexp.MustCompile("^[.][a-zA-Z][a-zA-Z0-9_-]*( [.]?[a-zA-Z][a-zA-Z0-9_-]*)*$")
var value = regexp.MustCompile("^[#a-zA-Z0-9 ,.:%/()-]{1,180}$")
var functionName = regexp.MustCompile("[a-zA-Z-]+[(]")
var cssFunctions = regexp.MustCompile("^(repeat|minmax|clamp|min|max|var|rgb|rgba|linear-gradient)[(]$")
var mediaQuery = regexp.MustCompile("^[(](min|max)-width: [0-9]{1,4}px[)]$")
var color = regexp.MustCompile("^#[0-9a-fA-F]{6}$")

func Color(s string) bool { return color.MatchString(s) }

// CSS is intentionally a small positive grammar. It cannot reference URLs,
// pseudo selectors, attributes, custom functions, or document-level styles.
func css(src string) (string, error) {
	var out strings.Builder
	for strings.TrimSpace(src) != "" {
		sel, rest, ok := strings.Cut(strings.TrimSpace(src), "{")
		if !ok {
			return "", invalid
		}
		body, next, ok := strings.Cut(rest, "}")
		if !ok {
			return "", invalid
		}
		src = next
		for i, x := range strings.Split(sel, ",") {
			x = strings.TrimSpace(x)
			if !selector.MatchString(x) {
				return "", invalid
			}
			if i > 0 {
				out.WriteString(",")
			}
			out.WriteString(".themed " + x)
		}
		out.WriteString("{")
		for _, p := range strings.Split(body, ";") {
			if strings.TrimSpace(p) == "" {
				continue
			}
			key, v, ok := strings.Cut(p, ":")
			key = strings.TrimSpace(key)
			v = strings.TrimSpace(v)
			for _, fn := range functionName.FindAllString(v, -1) {
				if !cssFunctions.MatchString(fn) {
					return "", invalid
				}
			}
			if !ok || !value.MatchString(v) || !slices.Contains(strings.Fields("color background background-color border border-radius border-color border-width border-style font-family font-weight font-size font-style line-height letter-spacing text-align text-transform text-decoration padding padding-block padding-inline margin margin-block margin-inline gap display flex-direction flex-wrap grid-template-columns align-items justify-content max-width width grid-template-rows grid-column grid-row order align-self text-wrap box-shadow"), key) {
				return "", invalid
			}
			out.WriteString(key + ":" + v + ";")
		}
		out.WriteString("}")
	}
	return out.String(), nil
}
func attr(n *html.Node, k string) string {
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val
		}
	}
	return ""
}

var slots = map[string][]string{"feed": {"header", "filters", "posts", "composer", "settings", "pagination"}, "post": {"header", "posts", "composer", "settings", "pagination"}, "login": {"header", "login"}, "item": {"byline", "title", "body", "media", "tags", "actions"}}

func Compile(src string) (Theme, error) {
	var t Theme
	if len(src) > 65536 {
		return t, invalid
	}
	doc, e := html.Parse(strings.NewReader(src))
	if e != nil {
		return t, e
	}
	t.Layouts = map[string]string{}
	var visit func(*html.Node) error
	visit = func(n *html.Node) error {
		if n.Type == html.TextNode {
			if strings.TrimSpace(n.Data) != "" {
				return invalid
			}
			return nil
		}
		if n.Type == html.CommentNode || n.Type == html.DoctypeNode {
			return nil
		}
		switch n.Data {
		case "html", "head", "body", "":
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if e := visit(c); e != nil {
					return e
				}
			}
		case "meta":
			if attr(n, "name") != "theme" {
				return invalid
			}
			if version := attr(n, "data-version"); version != "" && version != "1" {
				return invalid
			}
			t.ID = attr(n, "content")
			t.Name = attr(n, "title")
		case "style":
			var raw string
			if n.FirstChild != nil {
				raw = n.FirstChild.Data
			}
			s, e := css(raw)
			if e != nil {
				return e
			}
			if media := attr(n, "media"); media != "" {
				if !mediaQuery.MatchString(media) {
					return invalid
				}
				s = "@media " + media + "{" + s + "}"
			}
			t.Styles += s
		case "script":
			if attr(n, "type") != "application/json" || attr(n, "id") != "options" || n.FirstChild == nil {
				return invalid
			}
			if e := json.Unmarshal([]byte(n.FirstChild.Data), &t.Options); e != nil {
				return invalid
			}
			seenOptions := map[string]bool{}
			for _, o := range t.Options {
				if seenOptions[o.Key] {
					return invalid
				}
				seenOptions[o.Key] = true
				for _, l := range []string{"en", "tr"} {
					if o.Labels[l] == "" {
						t.Missing = append(t.Missing, o.Key+" ("+l+")")
					}
					for _, c := range o.Choices {
						if c.Labels[l] == "" {
							t.Missing = append(t.Missing, o.Key+"/"+c.Value+" ("+l+")")
						}
					}
				}
				for _, c := range o.Choices {
					if !choiceValue.MatchString(c.Value) || len(c.Labels) == 0 {
						return invalid
					}
				}
				if !storage.ValidKey(o.Key) || !o.accept(o.Default) || len(o.Labels) == 0 {
					return invalid
				}
			}
		case "template":
			name := attr(n, "id")
			allowed, ok := slots[name]
			if !ok {
				return invalid
			}
			var buf bytes.Buffer
			seen := map[string]bool{}
			var layout func(*html.Node, int) error
			layout = func(x *html.Node, depth int) error {
				if depth > 12 {
					return invalid
				}
				if x.Type == html.TextNode && strings.TrimSpace(x.Data) == "" {
					return nil
				}
				if x.Type != html.ElementNode {
					return invalid
				}
				if x.Data == "theme-text" {
					en, tr := attr(x, "en"), attr(x, "tr")
					if en == "" || tr == "" || strings.ContainsAny(en+tr, "[]") || len(en)+len(tr) > 2000 {
						return invalid
					}
					buf.WriteString("[[?en]]" + html.EscapeString(en) + "[[/]][[?tr]]" + html.EscapeString(tr) + "[[/]]")
					return nil
				}
				if cond := attr(x, "data-if"); cond != "" {
					if !slices.Contains([]string{"pinned", "draft", "owner", "media", "tags", "title"}, cond) {
						return invalid
					}
					buf.WriteString("[[?" + cond + "]]")
					defer buf.WriteString("[[/]]")
				}
				if x.Data == "slot" {
					k := attr(x, "name")
					if !slices.Contains(allowed, k) || seen[k] {
						return invalid
					}
					seen[k] = true
					buf.WriteString("[[" + k + "]]")
					return nil
				}
				if !slices.Contains([]string{"div", "section", "aside", "header", "main", "footer", "article"}, x.Data) {
					return invalid
				}
				class := attr(x, "class")
				if class != "" && !classes.MatchString(class) {
					return invalid
				}
				fmt.Fprintf(&buf, "<%s class=\"%s\">", x.Data, class)
				for c := x.FirstChild; c != nil; c = c.NextSibling {
					if e := layout(c, depth+1); e != nil {
						return e
					}
				}
				fmt.Fprintf(&buf, "</%s>", x.Data)
				return nil
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if e := layout(c, 0); e != nil {
					return e
				}
			}
			for _, k := range allowed {
				if !seen[k] {
					return invalid
				}
			}
			t.Layouts[name] = buf.String()
		default:
			return invalid
		}
		return nil
	}
	e = visit(doc)
	if e != nil || !storage.ValidKey(t.ID) || t.Name == "" {
		return t, invalid
	}
	return t, nil
}
func Builtins() map[string]string {
	m := map[string]string{}
	fs.WalkDir(builtins, ".", func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			b, _ := builtins.ReadFile(p)
			t, e := Compile(string(b))
			if e != nil {
				panic(p + ": " + e.Error())
			}
			if _, duplicate := m[t.ID]; duplicate {
				panic("duplicate theme ID: " + t.ID)
			}
			for _, o := range t.Options {
				if o.Labels["en"] == "" || o.Labels["tr"] == "" {
					panic("missing built-in option translation: " + o.Key)
				}
			}
			m[t.ID] = string(b)
		}
		return nil
	})
	return m
}
func (t Theme) Render(view string, parts map[string]string) string {
	return t.RenderLocale(view, parts, "en", nil)
}
func (t Theme) RenderLocale(view string, parts map[string]string, lang string, flags map[string]bool) string {
	layout := t.Layouts[view]
	if layout == "" {
		for _, s := range slots[view] {
			layout += "[[" + s + "]]"
		}
	}
	if strings.Contains(layout, "[[?") {
		var out strings.Builder
		stack := []bool{true}
		for len(layout) > 0 {
			prefix, tokens, ok := strings.Cut(layout, "[[")
			if stack[len(stack)-1] {
				out.WriteString(prefix)
			}
			if !ok {
				break
			}
			tok, rest, ok := strings.Cut(tokens, "]]")
			if !ok {
				break
			}
			layout = rest
			if key, ok := strings.CutPrefix(tok, "?"); ok {
				stack = append(stack, stack[len(stack)-1] && (flags[key] || lang == key))
			} else if tok == "/" {
				stack = stack[:len(stack)-1]
			} else if stack[len(stack)-1] {
				out.WriteString("[[" + tok + "]]")
			}
		}
		layout = out.String()
	}
	pairs := []string{}
	for _, k := range slots[view] {
		pairs = append(pairs, "[["+k+"]]", parts[k])
	}
	return strings.NewReplacer(pairs...).Replace(layout)
}
