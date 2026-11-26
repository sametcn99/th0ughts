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

var decimal = regexp.MustCompile(`^-?(?:[0-9]+(?:\.[0-9]+)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

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
		if !decimal.MatchString(v) {
			return false
		}
		n, e := strconv.ParseFloat(v, 64)
		if o.Step != nil {
			if *o.Step <= 0 {
				return false
			}
			steps := (n - o.Min) / *o.Step
			if math.IsInf(steps, 0) || math.Abs(steps-math.Round(steps)) > 1e-8 {
				return false
			}
		}
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

func (t Theme) InvalidOption(values map[string]string) string {
	for key, v := range values {
		ok := false
		for _, o := range t.Options {
			if key == o.Key && o.accept(v) {
				ok = true
				break
			}
		}
		if !ok {
			if key == "" {
				return "(empty)"
			}
			return key
		}
	}
	return ""
}
func (t Theme) Normalize(values map[string]string) {
	for key, value := range values {
		if t.InvalidOption(map[string]string{key: value}) != "" {
			delete(values, key)
		}
	}
}
