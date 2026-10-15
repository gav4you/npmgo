// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/core 2.11.0 (build/src/common/time.js and
// platform/node/index.js's otperformance).

package core

import (
	"math"
	"strings"
	"time"

	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/opentelemetry/api"
)

// Performance is what core reads of Node's performance object (perf_hooks): timeOrigin, the time (in milliseconds
// since the epoch) the process started, and now(), the milliseconds since then on a monotonic clock.
type Performance interface {
	TimeOrigin() float64
	Now() float64
}

// realPerformance measures from the time the package was initialised (Go has no record of when the process
// started), on Go's monotonic clock.
type realPerformance struct {
	origin time.Time
}

func (p realPerformance) TimeOrigin() float64 { return float64(p.origin.UnixNano()) / 1e6 }

func (p realPerformance) Now() float64 { return float64(time.Since(p.origin).Nanoseconds()) / 1e6 }

// Otperformance is the performance object core uses (otperformance, deprecated in the original in favour of
// performance). Replace it before use, for example with a fake clock in tests.
var Otperformance Performance = realPerformance{origin: time.Now()}

const (
	nanosecondDigits          = 9
	millisecondsToNanoseconds = 1e6 // Math.pow(10, 6)
	secondToNanoseconds       = 1e9 // Math.pow(10, 9)
)

// MillisToHrTime converts milliseconds since the epoch to an HrTime: the whole seconds (truncated) and the rest in
// nanoseconds (rounded as Math.round rounds; both negative for a negative time).
func MillisToHrTime(epochMillis float64) api.HrTime {
	epochSeconds := epochMillis / 1000
	seconds := math.Trunc(epochSeconds)
	nanos := jsnum.Round(float64(math.Mod(epochMillis, 1000) * millisecondsToNanoseconds))
	return api.HrTime{seconds, nanos}
}

// GetTimeOrigin returns Otperformance's time origin (deprecated in the original).
func GetTimeOrigin() float64 { return Otperformance.TimeOrigin() }

// HrTime returns the time origin plus performanceNow (the first argument) or, without one, Otperformance.Now(), as
// an HrTime.
func HrTime(performanceNow ...float64) api.HrTime {
	timeOrigin := MillisToHrTime(Otperformance.TimeOrigin())
	var now float64
	if len(performanceNow) > 0 {
		now = performanceNow[0]
	} else {
		now = Otperformance.Now()
	}
	return AddHrTimes(timeOrigin, MillisToHrTime(now))
}

// asHrTime returns a value IsTimeInputHrTime accepts as an HrTime.
func asHrTime(value any) (api.HrTime, bool) {
	switch v := value.(type) {
	case api.HrTime:
		return v, true
	case [2]float64:
		return api.HrTime(v), true
	}
	if jsvalue.IsArray(value) {
		items := jsvalue.Items(value)
		if len(items) == 2 && jsvalue.IsNumber(items[0]) && jsvalue.IsNumber(items[1]) {
			return api.HrTime{jsvalue.Float(items[0]), jsvalue.Float(items[1])}, true
		}
	}
	return api.HrTime{}, false
}

// dateValue returns the time value (getTime()) of a Date: a time.Time's milliseconds since the epoch, rounded down.
func dateValue(value any) (float64, bool) {
	switch d := value.(type) {
	case time.Time:
		return float64(d.UnixMilli()), true
	case *time.Time:
		if d != nil {
			return float64(d.UnixMilli()), true
		}
	}
	return 0, false
}

// TimeInputToHrTime converts a time input to an HrTime: an HrTime as it is; a number below half the time origin as
// a performance.now() reading (added to the time origin), any other number as milliseconds since the epoch; a Date
// (time.Time) by its milliseconds. Anything else panics with TypeError("Invalid input type").
func TimeInputToHrTime(time any) api.HrTime {
	if h, ok := asHrTime(time); ok {
		return h
	}
	if jsvalue.IsNumber(time) {
		t := jsvalue.Float(time)
		if t < Otperformance.TimeOrigin()/2 {
			return HrTime(t)
		}
		return MillisToHrTime(t)
	}
	if ms, ok := dateValue(time); ok {
		return MillisToHrTime(ms)
	}
	panic(&jsvalue.TypeError{Message: "Invalid input type"})
}

// HrTimeDuration returns endTime - startTime, borrowing a second when the nanoseconds go negative.
func HrTimeDuration(startTime, endTime api.HrTime) api.HrTime {
	seconds := endTime[0] - startTime[0]
	nanos := endTime[1] - startTime[1]
	if nanos < 0 {
		seconds--
		nanos += secondToNanoseconds
	}
	return api.HrTime{seconds, nanos}
}

// HrTimeToTimeStamp renders the time as an ISO 8601 timestamp with nanoseconds, "2019-05-14T17:00:00.000123456Z":
// the date of the whole seconds (new Date(time[0] * 1000).toISOString(), which panics with RangeError("Invalid time
// value") beyond the Date range) with its first "000Z" replaced by the last nine digits of the nanoseconds and "Z".
func HrTimeToTimeStamp(time api.HrTime) string {
	tmp := strings.Repeat("0", nanosecondDigits) + jsnum.String(time[1]) + "Z"
	nanoString := tmp[len(tmp)-nanosecondDigits-1:]
	date := toISOString(float64(time[0] * 1000))
	return strings.Replace(date, "000Z", nanoString, 1)
}

// toISOString is new Date(ms).toISOString().
func toISOString(ms float64) string {
	// TimeClip: NaN beyond ±8.64e15 ms, otherwise the time truncated to an integer.
	if math.IsNaN(ms) || math.IsInf(ms, 0) || math.Abs(ms) > 8.64e15 {
		panic(&jsvalue.RangeError{Message: "Invalid time value"})
	}
	return jsnum.ISOString(math.Trunc(ms) + 0)
}

// HrTimeToNanoseconds converts the time to nanoseconds.
func HrTimeToNanoseconds(time api.HrTime) float64 {
	return float64(time[0]*secondToNanoseconds) + time[1]
}

// HrTimeToMicroseconds converts the time to microseconds.
func HrTimeToMicroseconds(time api.HrTime) float64 { return float64(time[0]*1e6) + time[1]/1e3 }

// HrTimeToMilliseconds converts the time to milliseconds.
func HrTimeToMilliseconds(time api.HrTime) float64 { return float64(time[0]*1e3) + time[1]/1e6 }

// HrTimeToSeconds converts the time to seconds.
func HrTimeToSeconds(time api.HrTime) float64 { return time[0] + time[1]/secondToNanoseconds }

// IsTimeInputHrTime reports whether value is an HrTime: an api.HrTime or [2]float64, or an array (Go slice) of two
// numbers.
func IsTimeInputHrTime(value any) bool {
	_, ok := asHrTime(value)
	return ok
}

// IsTimeInput reports whether value is a time input: an HrTime, a number or a Date (time.Time).
func IsTimeInput(value any) bool {
	if IsTimeInputHrTime(value) || jsvalue.IsNumber(value) {
		return true
	}
	_, ok := dateValue(value)
	return ok
}

// AddHrTimes returns the sum of two times, carrying a second when the nanoseconds reach 1e9.
func AddHrTimes(time1, time2 api.HrTime) api.HrTime {
	out := api.HrTime{time1[0] + time2[0], time1[1] + time2[1]}
	if out[1] >= secondToNanoseconds {
		out[1] -= secondToNanoseconds
		out[0]++
	}
	return out
}
