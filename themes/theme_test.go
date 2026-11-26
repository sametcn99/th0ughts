package theme

import (
	"strings"
	"testing"
)

func TestCatalog(t *testing.T) {
	for id, source := range Builtins() {
		design, e := Compile(source)
		if e != nil || design.ID != id {
			t.Fatal(id, e)
		}
		for view, fields := range slots {
			parts := map[string]string{}
			for _, f := range fields {
				parts[f] = "(" + f + ")"
			}
			if view == "item" {
				parts["author"] = "(author)"
				parts["date"] = "(date)"
			}
			out := design.RenderLocale(view, parts, "en", map[string]bool{"media": true})
			for _, f := range fields {
				if f == "byline" && !strings.Contains(out, parts[f]) {
					if strings.Count(out, parts["author"]) != 1 || strings.Count(out, parts["date"]) != 1 {
						t.Fatal(id, view, "missing author or date")
					}
					continue
				}
				if strings.Count(out, parts[f]) != 1 {
					t.Fatal(id, view, f)
				}
			}
		}
	}
}
func TestRestrictedTheme(t *testing.T) {
	base := "<meta name=theme content=custom title=Custom>"
	for _, s := range []string{"<script>active</script>", "<style>.entry{background-image:url(https://example.org)}</style>", "<style>body{color:red}</style>", "<template id=feed><form></form></template>"} {
		if _, e := Compile(base + s); e == nil {
			t.Fatal("accepted unsupported theme feature")
		}
	}
}
func FuzzCompile(f *testing.F) {
	for _, src := range Builtins() {
		f.Add(src)
	}
	f.Fuzz(func(t *testing.T, input string) {
		v, e := Compile(input)
		if e == nil {
			v.Render("feed", nil)
		}
	})
}

func TestThemeVariantsAndDiagnostics(t *testing.T) {
	const meta = "<meta name=theme content=test title=Test>\n"
	const fields = "<slot name=author></slot><slot name=date></slot><slot name=title></slot><slot name=body></slot><slot name=media></slot><slot name=tags></slot><slot name=actions></slot>"
	source := meta + "<template id=item-article-detail><header data-if=detail>" + fields + "</header></template>"
	design, e := Compile(source)
	if e != nil {
		t.Fatal(e)
	}
	parts := map[string]string{"author": "Ada", "date": "June", "body": "Text", "readmore": "Read"}
	full := design.RenderLocale("item", parts, "en", map[string]bool{"article": true, "detail": true})
	if !strings.Contains(full, "<header") || !strings.Contains(full, "AdaJuneTextRead") {
		t.Fatal(full)
	}
	short := design.RenderLocale("item", parts, "en", map[string]bool{"short": true, "detail": true})
	if strings.Contains(short, "<header") {
		t.Fatal(short)
	}
	_, e = Compile(meta + "<style>\n/* two\nlines */\n.entry:hover {\n mystery: red;\n}\n</style>")
	d, ok := e.(*Diagnostic)
	if !ok || d.Line != 6 || d.Kind != "property" || d.Token != "mystery" {
		t.Fatal(e)
	}
	for _, rule := range []string{":root{display:none}", ".entry:visited{color:red}", "@media print{.entry{color:blue}}", ":root{--surface:url(example)}"} {
		if _, e = Compile(meta + "<style>" + rule + "</style>"); e == nil {
			t.Fatal(rule)
		}
	}
}
