// Package ms is a Go translation of the npm package ms@2.1.3
// (https://github.com/vercel/ms). Original copyright notice and MIT
// license are preserved in LICENSE.
package ms

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/gav4you/npmgo/internal/jsnum"
)

const (
	s = 1000.0
	m = s * 60
	h = m * 60
	d = h * 24
	w = d * 7
	y = d * 365.25
)

var re = regexp.MustCompile(`(?i)^(-?(?:\d+)?\.?\d+) *(milliseconds?|msecs?|ms|seconds?|secs?|s|minutes?|mins?|m|hours?|hrs?|h|days?|d|weeks?|w|years?|yrs?|y)?$`)

// Parse converts a duration string to milliseconds. ok is false where the
// JavaScript original returns undefined.
func Parse(str string) (ms float64, ok bool) {
	if len([]rune(str)) > 100 {
		return 0, false
	}
	match := re.FindStringSubmatch(str)
	if match == nil {
		return 0, false
	}
	n, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, false
	}
	unit := strings.ToLower(match[2])
	if unit == "" {
		unit = "ms"
	}
	switch unit {
	case "years", "year", "yrs", "yr", "y":
		return n * y, true
	case "weeks", "week", "w":
		return n * w, true
	case "days", "day", "d":
		return n * d, true
	case "hours", "hour", "hrs", "hr", "h":
		return n * h, true
	case "minutes", "minute", "mins", "min", "m":
		return n * m, true
	case "seconds", "second", "secs", "sec", "s":
		return n * s, true
	case "milliseconds", "millisecond", "msecs", "msec", "ms":
		return n, true
	}
	return 0, false
}

// Format renders milliseconds like ms(n) or, with long, ms(n, {long: true}).
func Format(ms float64, long bool) (string, error) {
	if math.IsNaN(ms) || math.IsInf(ms, 0) {
		v, _ := json.Marshal(nil)
		return "", errors.New("val is not a non-empty string or a valid number. val=" + string(v))
	}
	if long {
		return fmtLong(ms), nil
	}
	return fmtShort(ms), nil
}

func fmtShort(ms float64) string {
	abs := math.Abs(ms)
	switch {
	case abs >= d:
		return jsnum.String(jsnum.Round(ms/d)) + "d"
	case abs >= h:
		return jsnum.String(jsnum.Round(ms/h)) + "h"
	case abs >= m:
		return jsnum.String(jsnum.Round(ms/m)) + "m"
	case abs >= s:
		return jsnum.String(jsnum.Round(ms/s)) + "s"
	}
	return jsnum.String(ms) + "ms"
}

func fmtLong(ms float64) string {
	abs := math.Abs(ms)
	switch {
	case abs >= d:
		return plural(ms, abs, d, "day")
	case abs >= h:
		return plural(ms, abs, h, "hour")
	case abs >= m:
		return plural(ms, abs, m, "minute")
	case abs >= s:
		return plural(ms, abs, s, "second")
	}
	return jsnum.String(ms) + " ms"
}

func plural(ms, abs, n float64, name string) string {
	suffix := ""
	if abs >= n*1.5 {
		suffix = "s"
	}
	return jsnum.String(jsnum.Round(ms/n)) + " " + name + suffix
}
