// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/core 2.11.0 (build/src/propagation/composite.js).

package core

import (
	"reflect"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/opentelemetry/api"
)

// CompositePropagatorConfig is the configuration of a CompositePropagator.
type CompositePropagatorConfig struct {
	// Propagators run in order (nil: none).
	Propagators []api.TextMapPropagator
}

// CompositePropagator runs several propagators as one.
type CompositePropagator struct {
	propagators []api.TextMapPropagator
	fields      []string
}

// NewCompositePropagator is new CompositePropagator(config): the fields are those of every propagator, without
// repeats, in order.
func NewCompositePropagator(config CompositePropagatorConfig) *CompositePropagator {
	c := &CompositePropagator{propagators: config.Propagators}
	if c.propagators == nil {
		c.propagators = []api.TextMapPropagator{}
	}
	seen := map[string]bool{}
	c.fields = []string{}
	for _, p := range c.propagators {
		for _, f := range p.Fields() {
			if !seen[f] {
				seen[f] = true
				c.fields = append(c.fields, f)
			}
		}
	}
	return c
}

// Inject runs every propagator's Inject in order, so a later propagator's write to the same key wins. A propagator
// that panics is logged as a warning to api.Diag: "Failed to inject with <class>. Err: <message>".
func (c *CompositePropagator) Inject(context api.Context, carrier any, setter api.TextMapSetter) {
	for _, p := range c.propagators {
		func() {
			defer func() {
				if err := recover(); err != nil {
					api.Diag.Warn("Failed to inject with " + constructorName(p) + ". Err: " + errMessage(err))
				}
			}()
			p.Inject(context, carrier, setter)
		}()
	}
}

// Extract runs every propagator's Extract in order, each on the context the previous one returned, so a later
// propagator's value for the same key wins. A propagator that panics is logged as a warning ("Failed to extract with
// <class>. Err: <message>") and its input context passed on.
func (c *CompositePropagator) Extract(context api.Context, carrier any, getter api.TextMapGetter) api.Context {
	ctx := context
	for _, p := range c.propagators {
		ctx = func() (out api.Context) {
			defer func() {
				if err := recover(); err != nil {
					api.Diag.Warn("Failed to extract with " + constructorName(p) + ". Err: " + errMessage(err))
					out = ctx
				}
			}()
			return p.Extract(ctx, carrier, getter)
		}()
	}
	return ctx
}

// Fields returns a copy of the fields.
func (c *CompositePropagator) Fields() []string { return append([]string{}, c.fields...) }

// constructorName is propagator.constructor.name: the name of the Go type (behind any pointers), or "Object" for an
// unnamed type.
func constructorName(v any) string {
	t := reflect.TypeOf(v)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Name() == "" {
		return "Object"
	}
	return t.Name()
}

// errMessage is `${err.message}` for a thrown value: an error's message (a jsvalue error's Message, otherwise
// Error()), a plain object's message property, and undefined for anything else (a thrown string has no message).
// Reading the message of a thrown undefined throws, as in JavaScript.
func errMessage(err any) string {
	switch e := err.(type) {
	case jsvalue.Undefined:
		panic(&jsvalue.TypeError{Message: "Cannot read properties of undefined (reading 'message')"})
	case *jsvalue.TypeError:
		return e.Message
	case *jsvalue.RangeError:
		return e.Message
	case *jsvalue.URIError:
		return e.Message
	case error:
		return e.Error()
	case *jsvalue.Object:
		return jsvalue.ToString(e.Get("message"))
	}
	return "undefined"
}
