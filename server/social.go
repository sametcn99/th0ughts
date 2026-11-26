package server

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type socialImage struct {
	key string
	png []byte
}

var socialFonts = sync.OnceValue(func() [2]*opentype.Font {
	a, e := opentype.Parse(goregular.TTF)
	if e != nil {
		panic(e)
	}
	b, e := opentype.Parse(gomedium.TTF)
	if e != nil {
		panic(e)
	}
	return [2]*opentype.Font{a, b}
})

func (s *Server) social(w http.ResponseWriter, r *http.Request) {
	site, e := s.S.Settings(r.Context())
	if e != nil {
		fail(w, e)
		return
	}
	key := site.Name + "\x00" + site.Description + "\x00" + site.Author
	s.imageMu.Lock()
	defer s.imageMu.Unlock()
	if key != s.image.key {
		canvas := image.NewRGBA(image.Rect(0, 0, 1200, 630))
		ink := color.RGBA{34, 48, 39, 255}
		accent := color.RGBA{67, 100, 78, 255}
		fill := func(box image.Rectangle, c color.RGBA) {
			draw.Draw(canvas, box, image.NewUniform(c), image.Point{}, draw.Src)
		}
		fill(canvas.Bounds(), color.RGBA{246, 247, 240, 255})
		fill(image.Rect(64, 80, 1136, 83), accent)
		fill(image.Rect(64, 540, 1136, 542), accent)
		text := func(v string, size float64, x, y int, c color.RGBA, weight int) {
			face, e := opentype.NewFace(socialFonts()[weight], &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
			if e != nil {
				return
			}
			defer face.Close()
			v = strings.Join(strings.Fields(v), " ")
			d := font.Drawer{Dst: canvas, Src: image.NewUniform(c), Face: face, Dot: fixed.P(x, y)}
			if d.MeasureString(v).Ceil() > 1060 {
				for len(v) > 0 && d.MeasureString(v+"…").Ceil() > 1060 {
					_, n := utf8.DecodeLastRuneInString(v)
					v = v[:len(v)-n]
				}
				v += "…"
			}
			d.DrawString(v)
		}
		text(site.Name, 90, 62, 300, ink, 1)
		text(site.Description, 34, 65, 385, accent, 0)
		text(site.Author, 28, 65, 510, ink, 0)
		var b bytes.Buffer
		if e = png.Encode(&b, canvas); e != nil {
			fail(w, e)
			return
		}
		s.image = socialImage{key, b.Bytes()}
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, no-cache")
	w.Write(s.image.png)
}
