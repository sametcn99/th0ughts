package theme

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

type Diagnostic struct {
	Line  int    `json:"line"`
	Kind  string `json:"kind"`
	Token string `json:"token"`
}

func (d *Diagnostic) Error() string {
	return fmt.Sprintf("theme line %d (%s): %s", d.Line, d.Kind, d.Token)
}
func problem(line int, kind, token string) error {
	r := []rune(token)
	if len(r) > 120 {
		token = string(r[:120]) + "…"
	}
	return &Diagnostic{line, kind, token}
}
func elementLines(src string) map[string][]int {
	z := html.NewTokenizer(strings.NewReader(src))
	line := 1
	out := map[string][]int{}
	for {
		kind := z.Next()
		if kind == html.ErrorToken {
			return out
		}
		count := strings.Count(string(z.Raw()), "\n")
		if kind == html.StartTagToken || kind == html.SelfClosingTagToken {
			tag, _ := z.TagName()
			k := string(tag)
			out[k] = append(out[k], line)
		}
		line += count
	}
}

var selector = regexp.MustCompile(`^\.[A-Za-z][\w-]*(?::(?:hover|focus|focus-visible|focus-within|active|disabled|checked))?(?:(?:\s+|\s*>\s*)\.?[A-Za-z][\w-]*(?::(?:hover|focus|focus-visible|focus-within|active|disabled|checked))?)*$`)
var customProperty = regexp.MustCompile(`^--[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)
var cssValue = regexp.MustCompile(`^[#a-zA-Z0-9_ ,.:%/()+*-]{1,240}$`)
var functionName = regexp.MustCompile(`[a-zA-Z0-9_-]+\(`)
var cssFunctions = regexp.MustCompile(`^(repeat|minmax|clamp|min|max|var|calc|rgb|rgba|hsl|hsla|linear-gradient)\($`)
var mediaQuery = regexp.MustCompile(`^\((?:(min|max)-width:\s*[0-9]{1,4}px|prefers-color-scheme:\s*(dark|light)|prefers-reduced-motion:\s*(reduce|no-preference))\)$`)
var properties = strings.Fields("color background background-color border border-radius border-color border-width border-style font-family font-weight font-size font-style line-height letter-spacing text-align text-transform text-decoration padding padding-block padding-inline margin margin-block margin-inline gap row-gap column-gap display flex-direction flex-wrap grid-template-columns align-items justify-content max-width min-width width grid-template-rows grid-column grid-row order align-self text-wrap box-shadow outline outline-color outline-offset opacity")

func css(raw string, firstLine int) (string, error) {
	// Replace each comment with whitespace, retaining line numbers and token boundaries.
	b := []byte(raw)
	for pos := 0; pos < len(b); {
		start := strings.Index(raw[pos:], "/*")
		if start < 0 {
			break
		}
		start += pos
		end := strings.Index(raw[start+2:], "*/")
		if end < 0 {
			return "", problem(firstLine+strings.Count(raw[:start], "\n"), "rule", "/*")
		}
		pos = start + end + 4
		for i := start; i < pos; i++ {
			if b[i] != '\n' {
				b[i] = ' '
			}
		}
	}
	src := string(b)
	var out strings.Builder
	pos := 0
	line := firstLine
	advance := func(n int) { line += strings.Count(src[pos:pos+n], "\n"); pos += n }
	skip := func() { advance(len(src[pos:]) - len(strings.TrimLeftFunc(src[pos:], unicode.IsSpace))) }
	var rules func(bool) error
	rules = func(nested bool) error {
		for {
			skip()
			if pos == len(src) {
				if nested {
					return problem(line, "rule", "}")
				}
				return nil
			}
			if src[pos] == '}' {
				if !nested {
					return problem(line, "rule", "}")
				}
				advance(1)
				out.WriteByte('}')
				return nil
			}
			n := strings.IndexByte(src[pos:], '{')
			if n < 0 {
				return problem(line, "rule", src[pos:])
			}
			head := strings.TrimSpace(src[pos : pos+n])
			at := line
			advance(n + 1)
			if q, ok := strings.CutPrefix(head, "@media "); ok {
				q = strings.TrimSpace(q)
				if nested || !mediaQuery.MatchString(q) {
					return problem(at, "media", q)
				}
				out.WriteString("@media " + q + "{")
				if e := rules(true); e != nil {
					return e
				}
				continue
			}
			root := head == ":root"
			for i, sel := range strings.Split(head, ",") {
				sel = strings.TrimSpace(sel)
				if i > 0 {
					out.WriteByte(',')
				}
				if root {
					out.WriteString(":root")
				} else {
					if !selector.MatchString(sel) {
						return problem(at, "selector", sel)
					}
					out.WriteString(".themed " + sel)
				}
			}
			out.WriteByte('{')
			for {
				skip()
				if pos >= len(src) {
					return problem(line, "rule", "}")
				}
				if src[pos] == '}' {
					advance(1)
					out.WriteByte('}')
					break
				}
				end := strings.IndexAny(src[pos:], ";}")
				if end < 0 {
					return problem(line, "rule", "}")
				}
				if end == 0 {
					advance(1)
					continue
				}
				key, val, ok := strings.Cut(src[pos:pos+end], ":")
				key = strings.TrimSpace(key)
				val = strings.TrimSpace(val)
				if !ok || !customProperty.MatchString(key) && (root || !slices.Contains(properties, key)) {
					return problem(line, "property", key)
				}
				if !cssValue.MatchString(val) || strings.Contains(val, "/*") {
					return problem(line, "value", key+" = "+val)
				}
				for _, fn := range functionName.FindAllString(val, -1) {
					if !cssFunctions.MatchString(fn) {
						return problem(line, "value", fn)
					}
				}
				out.WriteString(key + ":" + val + ";")
				advance(end)
				if src[pos] == ';' {
					advance(1)
				}
			}
		}
	}
	if e := rules(false); e != nil {
		return "", e
	}
	return out.String(), nil
}

func (d Diagnostic) Localize(words map[string]string) string {
	return strings.NewReplacer("{line}", fmt.Sprint(d.Line), "{kind}", words["theme_error_"+d.Kind], "{token}", d.Token).Replace(words["theme_diagnostic"])
}
