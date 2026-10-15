// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/api 1.9.0 (build/src/common/Time.d.ts, a types-only
// module).

package api

// HrTime is a high-resolution time, [seconds, nanoseconds] (JavaScript's [number, number]).
//
// A TimeInput (an HrTime, a number of milliseconds or a Date) is an any holding an HrTime, a Go number or a
// time.Time.
type HrTime [2]float64
