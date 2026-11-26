package theme

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

type Choice struct {
	Value  string            `json:"value"`
	Labels map[string]string `json:"labels"`
}

var choiceValue = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9 _,-]{0,60}$")

func (o Option) accept(v string) bool {
	switch o.Type {
	case "", "color":
		return Color(v)
	case "toggle":
		return v == "0" || v == "1"
	case "select":
		for _, c := range o.Choices {
			if v == c.Value && choiceValue.MatchString(v) {
				return true
			}
		}
	case "range":
		n, e := strconv.ParseFloat(v, 64)
		return e == nil && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= o.Min && n <= o.Max && (o.Unit == "" || o.Unit == "px" || o.Unit == "rem" || o.Unit == "%")
	}
	return false
}
func (o Option) CSS(v string) string {
	if !o.accept(v) {
		v = o.Default
	}
	if o.Type == "range" {
		return strings.TrimSpace(v) + o.Unit
	}
	return v
}
