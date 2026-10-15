package jspromise

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gav4you/npmgo/internal/jsloop"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// thenable is an object with a then method.
type thenable func(resolve, reject func(any))

func (t thenable) Then(onFulfilled, onRejected func(any)) { t(onFulfilled, onRejected) }

// Each program's expected log is Node v25.9.0's for the same program written
// with native promises (the JavaScript is in the comment above it).
func TestOrder(t *testing.T) {
	und := jsvalue.Undefined{}
	programs := []struct {
		name string
		run  func(q jsloop.Microtasks, log func(string) any) // log returns the new log length, like push
		want string
	}{
		{
			// const p1 = Promise.resolve(1); const p2 = new Promise(r => r(p1));
			// p2.then(v => log('p2 ' + v));
			// p1.then(v => log('p1a ' + v)).then(() => log('p1b')).then(() => log('p1c')).then(() => log('p1d'));
			"adoption", func(q jsloop.Microtasks, log func(string) any) {
				p1 := Resolve(q, 1.0)
				p2 := New(q, func(res, _ func(any)) { res(p1) })
				p2.Then(func(v any) any { return log(fmt.Sprint("p2 ", v)) }, nil)
				p1.Then(func(v any) any { return log(fmt.Sprint("p1a ", v)) }, nil).
					Then(func(any) any { return log("p1b") }, nil).
					Then(func(any) any { return log("p1c") }, nil).
					Then(func(any) any { return log("p1d") }, nil)
			}, "p1a 1|p1b|p2 1|p1c|p1d",
		},
		{
			// const t = { then(res, rej) { log('then called'); res(5); res(6); rej(7); } };
			// Promise.resolve(t).then(v => log('p ' + v));
			// Promise.resolve().then(() => log('a')).then(() => log('b')).then(() => log('c'));
			// Promise.resolve({ then() { throw 'thrown in then'; } }).then(null, e => log('bad ' + e));
			// Promise.resolve({ then(res) { res(1); throw 'after resolve'; } }).then(v => log('late ' + v), e => log('late err ' + e));
			"thenable", func(q jsloop.Microtasks, log func(string) any) {
				t := thenable(func(res, rej func(any)) { log("then called"); res(5.0); res(6.0); rej(7.0) })
				Resolve(q, t).Then(func(v any) any { return log(fmt.Sprint("p ", v)) }, nil)
				Resolve(q, und).Then(func(any) any { return log("a") }, nil).
					Then(func(any) any { return log("b") }, nil).
					Then(func(any) any { return log("c") }, nil)
				Resolve(q, thenable(func(_, _ func(any)) { panic("thrown in then") })).
					Then(nil, func(e any) any { return log(fmt.Sprint("bad ", e)) })
				Resolve(q, thenable(func(res, _ func(any)) { res(1.0); panic("after resolve") })).
					Then(func(v any) any { return log(fmt.Sprint("late ", v)) }, func(e any) any { return log(fmt.Sprint("late err ", e)) })
			}, "then called|a|p 5|b|bad thrown in then|late 1|c",
		},
		{
			// let resolveA; const a = new Promise(r => { resolveA = r; });
			// Promise.race([a, 2, Promise.resolve(3)]).then(v => log('race ' + v));
			// Promise.race([]).then(() => log('never'));
			// Promise.race([Promise.reject('no'), 1]).then(v => log('r2 ' + v), e => log('r2 err ' + e));
			// resolveA(1);
			// Promise.resolve().then(() => log('x')).then(() => log('y'));
			"race", func(q jsloop.Microtasks, log func(string) any) {
				var resolveA func(any)
				a := New(q, func(res, _ func(any)) { resolveA = res })
				Race(q, []any{a, 2.0, Resolve(q, 3.0)}).Then(func(v any) any { return log(fmt.Sprint("race ", v)) }, nil)
				Race(q, nil).Then(func(any) any { return log("never") }, nil)
				Race(q, []any{Reject(q, "no"), 1.0}).Then(func(v any) any { return log(fmt.Sprint("r2 ", v)) },
					func(e any) any { return log(fmt.Sprint("r2 err ", e)) })
				resolveA(1.0)
				Resolve(q, und).Then(func(any) any { return log("x") }, nil).Then(func(any) any { return log("y") }, nil)
			}, "x|race 2|r2 err no|y",
		},
		{
			// new Promise((res) => { res('ok'); throw 'ignored'; }).then(v => log('e1 ' + v), e => log('e1 err ' + e));
			// new Promise(() => { throw 'boom'; }).then(v => log('e2 ' + v), e => log('e2 err ' + e));
			// new Promise((res, rej) => { rej('first'); res('second'); }).then(v => log('e3 ' + v), e => log('e3 err ' + e));
			"executor", func(q jsloop.Microtasks, log func(string) any) {
				both := func(name string) (func(any) any, func(any) any) {
					return func(v any) any { return log(fmt.Sprint(name, " ", v)) }, func(e any) any { return log(fmt.Sprint(name, " err ", e)) }
				}
				New(q, func(res, _ func(any)) { res("ok"); panic("ignored") }).Then(both("e1"))
				New(q, func(_, _ func(any)) { panic("boom") }).Then(both("e2"))
				New(q, func(res, rej func(any)) { rej("first"); res("second") }).Then(both("e3"))
			}, "e1 ok|e2 err boom|e3 err first",
		},
		{
			// Promise.resolve(1).then(() => { throw 'h1'; }).then(v => log('no ' + v)).catch(e => log('caught ' + e)).then(v => log('after ' + v));
			// Promise.reject('r').then(v => log('nope')).then(null, e => log('passed ' + e));
			// Promise.resolve(2).then(null, null).then(v => log('passthrough ' + v));
			// const p = Promise.resolve(3); p.then(v => log('one ' + v)); p.then(v => log('two ' + v));
			// Promise.resolve(Promise.resolve(4)).then(v => log('same ' + v));
			"handlers", func(q jsloop.Microtasks, log func(string) any) {
				Resolve(q, 1.0).Then(func(any) any { panic("h1") }, nil).
					Then(func(v any) any { return log(fmt.Sprint("no ", v)) }, nil).
					Catch(func(e any) any { return log(fmt.Sprint("caught ", e)) }).
					Then(func(v any) any { return log(fmt.Sprint("after ", v)) }, nil)
				Reject(q, "r").Then(func(any) any { return log("nope") }, nil).
					Then(nil, func(e any) any { return log(fmt.Sprint("passed ", e)) })
				Resolve(q, 2.0).Then(nil, nil).Then(func(v any) any { return log(fmt.Sprint("passthrough ", v)) }, nil)
				p := Resolve(q, 3.0)
				p.Then(func(v any) any { return log(fmt.Sprint("one ", v)) }, nil)
				p.Then(func(v any) any { return log(fmt.Sprint("two ", v)) }, nil)
				Resolve(q, Resolve(q, 4.0)).Then(func(v any) any { return log(fmt.Sprint("same ", v)) }, nil)
			}, "one 3|two 3|same 4|passed r|passthrough 2|caught h1|after 6",
		},
		{
			// let resolveSelf; const p = new Promise(r => { resolveSelf = r; }); resolveSelf(p);
			// p.then(null, e => log(e.name + ': ' + e.message));
			"self", func(q jsloop.Microtasks, log func(string) any) {
				var resolveSelf func(any)
				p := New(q, func(res, _ func(any)) { resolveSelf = res })
				resolveSelf(p)
				p.Then(nil, func(e any) any { return log("TypeError: " + e.(*jsvalue.TypeError).Message) })
			}, "TypeError: Chaining cycle detected for promise #<Promise>",
		},
		{
			// const outer = new Promise(r => r(new Promise(r2 => r2(Promise.resolve('deep')))));
			// outer.then(v => log('outer ' + v));
			// let n = 0; const step = () => { log('step ' + (++n)); if (n < 8) Promise.resolve().then(step); };
			// Promise.resolve().then(step);
			"nested", func(q jsloop.Microtasks, log func(string) any) {
				outer := New(q, func(res, _ func(any)) {
					res(New(q, func(res2, _ func(any)) { res2(Resolve(q, "deep")) }))
				})
				outer.Then(func(v any) any { return log(fmt.Sprint("outer ", v)) }, nil)
				n := 0
				var step func(any) any
				step = func(any) any {
					n++
					log(fmt.Sprint("step ", n))
					if n < 8 {
						Resolve(q, und).Then(step, nil)
					}
					return und
				}
				Resolve(q, und).Then(step, nil)
			}, "step 1|step 2|step 3|outer deep|step 4|step 5|step 6|step 7|step 8",
		},
	}
	for _, p := range programs {
		v := jsloop.NewVirtual(0)
		var log []string
		p.run(v, func(s string) any { log = append(log, s); return float64(len(log)) })
		v.Run()
		if got := strings.Join(log, "|"); got != p.want {
			t.Errorf("%s:\n got %s\nwant %s", p.name, got, p.want)
		}
	}
}

func TestStateAndResult(t *testing.T) {
	v := jsloop.NewVirtual(0)
	var resolve func(any)
	p := New(v, func(res, _ func(any)) { resolve = res })
	if p.State() != Pending || p.Result() != (jsvalue.Undefined{}) {
		t.Fatalf("pending: %v %v", p.State(), p.Result())
	}
	resolve("x")
	if p.State() != Fulfilled || p.Result() != "x" {
		t.Fatalf("fulfilled: %v %v", p.State(), p.Result())
	}
	r := Reject(v, "why")
	if r.State() != Rejected || r.Result() != "why" {
		t.Fatalf("rejected: %v %v", r.State(), r.Result())
	}
	// A nil Thenable pointer is null, not a thenable.
	var nt *nilThenable
	n := Resolve(v, nt)
	if n.State() != Fulfilled {
		t.Fatalf("nil thenable: %v", n.State())
	}
	v.Run()
}

type nilThenable struct{}

func (*nilThenable) Then(onFulfilled, onRejected func(any)) { panic("called") }
