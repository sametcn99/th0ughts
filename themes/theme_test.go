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
			out := design.Render(view, parts)
			for _, f := range fields {
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
