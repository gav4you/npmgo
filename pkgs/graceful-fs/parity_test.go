package gracefulfs

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/gav4you/npmgo/internal/jsloop"
	"github.com/gav4you/npmgo/internal/jsnum"
	"github.com/gav4you/npmgo/internal/jsvalue"
)

// This file replays parity/gracefulfs/cases.json against the Go port the way
// parity/oracle_gracefulfs.cjs runs them against the original: a scripted fake
// fs module, a jsloop.Virtual clock, and the same trace encoding. Every
// function and convention here mirrors one in the oracle.

type paritySpec struct {
	Scheduler []program    `json:"scheduler"`
	Cases     []parityCase `json:"cases"`
}

type program struct {
	Name  string `json:"name"`
	Start int64  `json:"start"`
	Ops   []op   `json:"ops"`
}

type op struct {
	Timeout *int64  `json:"timeout"`
	After   *int64  `json:"after"`
	Tick    *string `json:"tick"`
	Clear   *string `json:"clear"`
	ID      string  `json:"id"`
	Panic   bool    `json:"panic"`
	Then    []op    `json:"then"`
}

type parityCase struct {
	Name    string               `json:"name"`
	Env     map[string]any       `json:"env"`
	Missing []string             `json:"missing"`
	Script  map[string][]outcome `json:"script"`
	Bases   []baseSpec           `json:"bases"`
	Load    []int                `json:"load"`
	Steps   []stepSpec           `json:"steps"`
}

type baseSpec struct {
	Missing []string             `json:"missing"`
	Script  map[string][]outcome `json:"script"`
}

type outcome struct {
	Err   string          `json:"err"`
	Ok    json.RawMessage `json:"ok"`
	Ms    int64           `json:"ms"`
	Times int             `json:"times"`
	Throw string          `json:"throw"`
}

type stepSpec struct {
	At   int64             `json:"at"`
	On   string            `json:"on"`
	Call string            `json:"call"`
	Args []json.RawMessage `json:"args"`
}

type cbSpec struct {
	Cb     int        `json:"cb"`
	Then   []stepSpec `json:"then"`
	Throws string     `json:"throws"`
}

type caseResult struct {
	Trace     []string   `json:"trace"`
	End       int64      `json:"end"`
	Timers    int        `json:"timers"`
	TimerHash string     `json:"timerHash"`
	Queue     int        `json:"queue"`
	Has       [][]string `json:"has"`
}

// fakeError is the oracle's FakeError: a code and the number of the call
// that failed (or "cb<n>" for an error a callback throws).
type fakeError struct{ code, id string }

func (e *fakeError) Error() string { return e.code + " #" + e.id }
func (e *fakeError) Code() string  { return e.code }

type fakeDirent struct{ name string }

// jsonValue is an options object, kept as its compact JSON (JSON.stringify
// of the object the oracle parses from the same text).
type jsonValue string

// brokenCase is a harness failure (a script that ran out), never a result.
type brokenCase struct{ msg string }

// harness runs one case.
type harness struct {
	t         *testing.T
	name      string
	v         *jsloop.Virtual
	start     int64
	out       []string
	calls     int
	streams   int
	instances []*FS
	fakes     []*fakeFS
	broken    string
	fired     int
	hash      uint32
}

func (h *harness) trace(line string) {
	h.out = append(h.out, strconv.FormatInt(h.v.Now()-h.start, 10)+" "+line)
}

func (h *harness) breakCase(msg string) {
	if h.broken == "" {
		h.broken = msg
	}
	panic(&brokenCase{"case " + h.name + ": " + msg})
}

// Now, SetTimeout, ClearTimeout and NextTick make the harness the Host: the
// virtual loop, with every timer fire counted and hashed as the oracle does.
func (h *harness) Now() int64 { return h.v.Now() }
func (h *harness) SetTimeout(fn func(), ms int64) any {
	return h.v.SetTimeout(func() {
		h.fired++
		h.hash = fnv(h.hash, strconv.FormatInt(h.v.Now()-h.start, 10)+";")
		fn()
	}, ms)
}
func (h *harness) ClearTimeout(t any) { h.v.ClearTimeout(t) }
func (h *harness) NextTick(fn func()) { h.v.NextTick(fn) }

func fnv(h uint32, s string) uint32 {
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// enc is the oracle's enc.
func enc(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case jsvalue.Undefined:
		return "undefined"
	case bool:
		return strconv.FormatBool(x)
	case float64:
		if x == 0 && math.Signbit(x) {
			return "-0"
		}
		return jsnum.String(x)
	case int:
		return strconv.Itoa(x)
	case string:
		return jsvalue.JSONQuote(x)
	case []byte:
		return "buf:" + hex.EncodeToString(x)
	case []any:
		if x == nil {
			return "undefined"
		}
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = enc(e)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case *fakeError:
		return "err:" + x.code + "#" + x.id
	case *fakeDirent:
		return "dirent:" + jsvalue.JSONQuote(x.name)
	case *Stats:
		if x == nil {
			return "undefined"
		}
		return fmt.Sprintf("stats:%d,%d", x.Uid, x.Gid)
	case *fakeStream:
		return "stream" + strconv.Itoa(x.sid)
	case jsonValue:
		return string(x)
	}
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Func {
		if rv.IsNil() {
			return "undefined"
		}
		return "fn"
	}
	panic(fmt.Sprintf("enc: unexpected value %T", v))
}

// undefinedArg reports a value the oracle sees as undefined: Undefined{} or a
// nil callback.
func undefinedArg(v any) bool {
	if _, ok := v.(jsvalue.Undefined); ok {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Func && rv.IsNil()
}

// thrown is the oracle's thrown: a fake error, or the name of the JavaScript
// error a Go panic stands for (calling a nil function is a TypeError).
func thrown(r any) string {
	switch x := r.(type) {
	case *brokenCase:
		panic(x)
	case *fakeError:
		return enc(x)
	case runtime.Error:
		if strings.Contains(x.Error(), "nil pointer dereference") {
			return "TypeError"
		}
	}
	return fmt.Sprintf("panic %v", r)
}

// val is the oracle's val: a scripted result.
func val(raw json.RawMessage) any {
	raw = bytes.TrimSpace(raw)
	switch raw[0] {
	case '[':
		var items []json.RawMessage
		json.Unmarshal(raw, &items)
		out := make([]any, len(items))
		for i, it := range items {
			out[i] = val(it)
		}
		return out
	case '{':
		var o struct {
			Buf    *string
			Dirent *string
			Stats  []int64
			Undef  bool
		}
		json.Unmarshal(raw, &o)
		switch {
		case o.Buf != nil:
			b, _ := hex.DecodeString(*o.Buf)
			return b
		case o.Dirent != nil:
			return &fakeDirent{*o.Dirent}
		case o.Stats != nil:
			return &Stats{Uid: o.Stats[0], Gid: o.Stats[1]}
		case o.Undef:
			return jsvalue.Undefined{}
		}
		panic("bad value " + string(raw))
	}
	var v any
	json.Unmarshal(raw, &v)
	return v
}

// fakeFS is the oracle's makeFake; fsv is the module it builds.
type fakeFS struct {
	h       *harness
	name    string
	missing map[string]bool
	script  map[string][]outcome
	cursors map[string]*[2]int
	fsv     *FS
}

func (f *fakeFS) record(name string, args ...any) string {
	f.h.calls++
	n := len(args)
	for n > 0 && undefinedArg(args[n-1]) {
		n--
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = enc(args[i])
	}
	f.h.trace(f.name + "." + name + "(" + strings.Join(parts, ",") + ")")
	return strconv.Itoa(f.h.calls)
}

func (f *fakeFS) next(name string, arg0 any, kind string) outcome {
	key := name
	var k0 string
	switch x := arg0.(type) {
	case string:
		k0 = name + ":" + x
	case float64:
		k0 = name + ":" + jsnum.String(x)
	}
	if _, ok := f.script[k0]; k0 != "" && ok {
		key = k0
	}
	list, ok := f.script[key]
	if !ok {
		if kind != "void" {
			f.h.breakCase("unscripted " + name)
		}
		return outcome{}
	}
	c := f.cursors[key]
	if c == nil {
		c = &[2]int{}
		f.cursors[key] = c
	}
	if c[0] >= len(list) {
		f.h.breakCase("script " + key + " exhausted")
	}
	o := list[c[0]]
	c[1]++
	if c[1] >= max(o.Times, 1) {
		c[0]++
		c[1] = 0
	}
	if o.Ok != nil && kind == "void" {
		f.h.breakCase(key + ": a 'void' function has no result")
	}
	if o.Ok == nil && o.Err == "" && o.Throw == "" && kind != "void" {
		f.h.breakCase(key + ": no result")
	}
	return o
}

// async records an asynchronous call, checks its callback as Node does,
// takes the next outcome and completes the call o.ms later.
func (f *fakeFS) async(name, kind string, cbNil bool, args []any, complete func(o outcome, err error)) {
	id := f.record(name, args...)
	if cbNil {
		panic(&fakeError{"ERR_INVALID_ARG_TYPE", id})
	}
	o := f.next(name, args[0], kind)
	if o.Throw != "" {
		panic(&fakeError{o.Throw, id})
	}
	f.h.v.After(o.Ms, func() {
		var err error
		if o.Err != "" {
			err = &fakeError{o.Err, id}
		}
		complete(o, err)
	})
}

// sync records a synchronous call and returns its outcome's result.
func (f *fakeFS) sync(name, kind string, args ...any) (any, error) {
	id := f.record(name, args...)
	o := f.next(name, args[0], kind)
	if o.Err != "" {
		return nil, &fakeError{o.Err, id}
	}
	if kind == "void" {
		return jsvalue.Undefined{}, nil
	}
	return val(o.Ok), nil
}

func (f *fakeFS) voidDone(cb func(error)) func(outcome, error) {
	return func(o outcome, err error) { cb(err) }
}

func (f *fakeFS) valDone(cb func(error, any)) func(outcome, error) {
	return func(o outcome, err error) {
		if err != nil && o.Ok == nil {
			cb(err, nil)
			return
		}
		cb(err, val(o.Ok))
	}
}

func statsOf(v any) *Stats {
	st, _ := v.(*Stats)
	return st
}

// statArgs is the argument list the module receives: options only when
// graceful-fs passed them.
func statArgs(target, options, cb any) []any {
	if options == nil {
		return []any{target, cb}
	}
	return []any{target, options, cb}
}

func (f *fakeFS) build() *FS {
	fs := &FS{}
	has := func(name string) bool { return !f.missing[name] }
	voidAsync := func(name string) func(args []any, cb func(error)) {
		return func(args []any, cb func(error)) {
			f.async(name, "void", cb == nil, append(args, cb), f.voidDone(cb))
		}
	}
	if has("open") {
		fs.Open = func(path, flags, mode any, cb func(error, any)) {
			f.async("open", "val", cb == nil, []any{path, flags, mode, cb}, f.valDone(cb))
		}
	}
	if has("close") {
		do := voidAsync("close")
		fs.Close = func(fd any, cb func(error)) { do([]any{fd}, cb) }
	}
	if has("readFile") {
		fs.ReadFile = func(path, options any, cb func(error, any)) {
			f.async("readFile", "val", cb == nil, []any{path, options, cb}, f.valDone(cb))
		}
	}
	if has("writeFile") {
		do := voidAsync("writeFile")
		fs.WriteFile = func(path, data, options any, cb func(error)) { do([]any{path, data, options}, cb) }
	}
	if has("appendFile") {
		do := voidAsync("appendFile")
		fs.AppendFile = func(path, data, options any, cb func(error)) { do([]any{path, data, options}, cb) }
	}
	if has("copyFile") {
		do := voidAsync("copyFile")
		fs.CopyFile = func(src, dest, flags any, cb func(error)) { do([]any{src, dest, flags}, cb) }
	}
	if has("readdir") {
		fs.Readdir = func(path, options any, cb func(error, []any)) {
			f.async("readdir", "val", cb == nil, []any{path, options, cb}, func(o outcome, err error) {
				if err != nil && o.Ok == nil {
					cb(err, nil)
					return
				}
				cb(err, val(o.Ok).([]any))
			})
		}
	}
	if has("read") {
		fs.Read = func(fd, buffer, offset, length, position any, cb func(error, any, any)) {
			f.async("read", "read", cb == nil, []any{fd, buffer, offset, length, position, cb}, func(o outcome, err error) {
				if err != nil {
					cb(err, nil, nil)
					return
				}
				cb(nil, val(o.Ok), buffer)
			})
		}
	}
	if has("rename") {
		do := voidAsync("rename")
		fs.Rename = func(from, to any, cb func(error)) { do([]any{from, to}, cb) }
	}
	for _, x := range []struct {
		name string
		fn   *func(target, uid, gid any, cb func(error))
	}{{"chown", &fs.Chown}, {"fchown", &fs.Fchown}, {"lchown", &fs.Lchown}} {
		if has(x.name) {
			do := voidAsync(x.name)
			*x.fn = func(target, uid, gid any, cb func(error)) { do([]any{target, uid, gid}, cb) }
		}
	}
	for _, x := range []struct {
		name string
		fn   *func(target, mode any, cb func(error))
	}{{"chmod", &fs.Chmod}, {"fchmod", &fs.Fchmod}, {"lchmod", &fs.Lchmod}} {
		if has(x.name) {
			do := voidAsync(x.name)
			*x.fn = func(target, mode any, cb func(error)) { do([]any{target, mode}, cb) }
		}
	}
	for _, x := range []struct {
		name string
		fn   *func(target, options any, cb func(error, *Stats))
	}{{"stat", &fs.Stat}, {"fstat", &fs.Fstat}, {"lstat", &fs.Lstat}} {
		if has(x.name) {
			name := x.name
			*x.fn = func(target, options any, cb func(error, *Stats)) {
				f.async(name, "val", cb == nil, statArgs(target, options, cb), func(o outcome, err error) {
					if err != nil && o.Ok == nil {
						cb(err, nil)
						return
					}
					cb(err, statsOf(val(o.Ok)))
				})
			}
		}
	}
	for _, x := range []struct {
		name string
		fn   *func(target, at, mt any, cb func(error))
	}{{"futimes", &fs.Futimes}, {"lutimes", &fs.Lutimes}} {
		if has(x.name) {
			do := voidAsync(x.name)
			*x.fn = func(target, at, mt any, cb func(error)) { do([]any{target, at, mt}, cb) }
		}
	}

	if has("openSync") {
		fs.OpenSync = func(path, flags any) (any, error) { return f.sync("openSync", "val", path, flags) }
	}
	if has("closeSync") {
		fs.CloseSync = func(fd any) error { _, err := f.sync("closeSync", "void", fd); return err }
	}
	if has("readSync") {
		fs.ReadSync = func(fd, buffer, offset, length, position any) (any, error) {
			return f.sync("readSync", "val", fd, buffer, offset, length, position)
		}
	}
	for _, x := range []struct {
		name string
		fn   *func(target, uid, gid any) error
	}{{"chownSync", &fs.ChownSync}, {"fchownSync", &fs.FchownSync}, {"lchownSync", &fs.LchownSync}} {
		if has(x.name) {
			name := x.name
			*x.fn = func(target, uid, gid any) error { _, err := f.sync(name, "void", target, uid, gid); return err }
		}
	}
	for _, x := range []struct {
		name string
		fn   *func(target, mode any) error
	}{{"chmodSync", &fs.ChmodSync}, {"fchmodSync", &fs.FchmodSync}, {"lchmodSync", &fs.LchmodSync}} {
		if has(x.name) {
			name := x.name
			*x.fn = func(target, mode any) error { _, err := f.sync(name, "void", target, mode); return err }
		}
	}
	for _, x := range []struct {
		name string
		fn   *func(target, options any) (*Stats, error)
	}{{"statSync", &fs.StatSync}, {"fstatSync", &fs.FstatSync}, {"lstatSync", &fs.LstatSync}} {
		if has(x.name) {
			name := x.name
			*x.fn = func(target, options any) (*Stats, error) {
				args := []any{target, options}
				if options == nil {
					args = args[:1]
				}
				v, err := f.sync(name, "val", args...)
				if err != nil {
					return nil, err
				}
				return statsOf(v), nil
			}
		}
	}
	for _, x := range []struct {
		name string
		fn   *func(target, at, mt any) error
	}{{"futimesSync", &fs.FutimesSync}, {"lutimesSync", &fs.LutimesSync}} {
		if has(x.name) {
			name := x.name
			*x.fn = func(target, at, mt any) error { _, err := f.sync(name, "void", target, at, mt); return err }
		}
	}
	if has("ReadStream") {
		fs.ReadStream = func(path, options any, open func(Stream)) Stream { return f.h.newStream("Read", path, options, open) }
	}
	if has("WriteStream") {
		fs.WriteStream = func(path, options any, open func(Stream)) Stream { return f.h.newStream("Write", path, options, open) }
	}
	return fs
}

// fakeStream is the oracle's stream class S.
type fakeStream struct {
	h                *harness
	sid              int
	path, flags, mod any
	autoClose        any
	fd               any
}

func (h *harness) newStream(kind string, path, options any, open func(Stream)) Stream {
	h.streams++
	s := &fakeStream{h: h, sid: h.streams, path: path, flags: "r", mod: float64(0o666), autoClose: true, fd: nil}
	if kind == "Write" {
		s.flags = "w"
	}
	if o, ok := options.(jsonValue); ok {
		var m map[string]any
		json.Unmarshal([]byte(o), &m)
		if v, ok := m["flags"]; ok {
			s.flags = v
		}
		if v, ok := m["mode"]; ok {
			s.mod = v
		}
		if v, ok := m["autoClose"]; ok {
			s.autoClose = v
		}
		if v, ok := m["fd"]; ok {
			s.fd = v
		}
	}
	h.trace(fmt.Sprintf("stream%d = new %sStream(%s)", s.sid, kind, enc(path)))
	if s.fd == nil {
		if open != nil {
			open(s)
		} else {
			h.trace(fmt.Sprintf("stream%d.open()", s.sid))
		}
	}
	return s
}

func (s *fakeStream) OpenArgs() (path, flags, mode any) { return s.path, s.flags, s.mod }
func (s *fakeStream) AutoClose() bool                   { return jsvalue.Truthy(s.autoClose) }
func (s *fakeStream) SetFD(fd any)                      { s.fd = fd }
func (s *fakeStream) Destroy()                          { s.h.trace(fmt.Sprintf("stream%d.destroy()", s.sid)) }
func (s *fakeStream) Read()                             { s.h.trace(fmt.Sprintf("stream%d.read()", s.sid)) }
func (s *fakeStream) Emit(event string, arg any) {
	s.h.trace(fmt.Sprintf("stream%d.emit(%s,%s)", s.sid, jsvalue.JSONQuote(event), enc(arg)))
}

// cbKind is the callback type of an operation: the Go type of its callback,
// and of a callback a case passes in another of its argument positions.
func cbKind(op string) string {
	switch op {
	case "open", "readFile":
		return "val"
	case "readdir":
		return "files"
	case "stat", "fstat", "lstat":
		return "stats"
	case "read":
		return "read"
	}
	return "void"
}

// recorder is the oracle's recording callback, typed for op. A nil error is
// null (JavaScript's null, undefined or no argument, which the oracle records
// alike); a nil result is an argument that was not passed (undefined), since
// the fake fs never passes null as a result.
func (h *harness) recorder(spec cbSpec, op string) any {
	rec := func(err error, vals ...any) {
		parts := []string{"null"}
		if err != nil {
			parts[0] = enc(err)
		}
		for _, v := range vals {
			if v == nil {
				v = jsvalue.Undefined{}
			}
			parts = append(parts, enc(v))
		}
		h.trace(fmt.Sprintf("cb%d(%s)", spec.Cb, strings.Join(parts, ",")))
		for j, s := range spec.Then {
			h.runStep(s, fmt.Sprintf("cb%d.%d", spec.Cb, j))
		}
		if spec.Throws != "" {
			panic(&fakeError{spec.Throws, fmt.Sprintf("cb%d", spec.Cb)})
		}
	}
	switch cbKind(op) {
	case "val":
		return func(err error, v any) { rec(err, v) }
	case "files":
		return func(err error, files []any) { rec(err, files) }
	case "stats":
		return func(err error, st *Stats) { rec(err, st) }
	case "read":
		return func(err error, n, buf any) { rec(err, n, buf) }
	}
	return func(err error) { rec(err) }
}

// arg is the oracle's arg.
func (h *harness) arg(raw json.RawMessage, op string) any {
	raw = bytes.TrimSpace(raw)
	if raw[0] == '{' {
		var o struct {
			Undef bool
			Buf   *string
			Opts  json.RawMessage
			Cb    *int
		}
		json.Unmarshal(raw, &o)
		switch {
		case o.Undef:
			return jsvalue.Undefined{}
		case o.Buf != nil:
			b, _ := hex.DecodeString(*o.Buf)
			return b
		case o.Opts != nil:
			var b bytes.Buffer
			json.Compact(&b, o.Opts)
			return jsonValue(b.String())
		case o.Cb != nil:
			var spec cbSpec
			json.Unmarshal(raw, &spec)
			return h.recorder(spec, op)
		}
		h.t.Fatalf("case %s: bad argument %s", h.name, raw)
	}
	var v any
	json.Unmarshal(raw, &v)
	return v
}

var reportsResult = regexp.MustCompile(`Sync$|^create.*Stream$`)

// runStep is the oracle's runStep.
func (h *harness) runStep(s stepSpec, label string) {
	on := s.On
	if on == "" {
		on = "g0"
	}
	idx, _ := strconv.Atoi(on[1:])
	var target *FS
	if on[0] == 'g' {
		target = h.instances[idx]
	} else {
		target = h.fakes[idx].fsv
	}
	args := make([]any, len(s.Args))
	for i, a := range s.Args {
		args[i] = h.arg(a, s.Call)
	}
	defer func() {
		if r := recover(); r != nil {
			h.trace(label + " throws " + thrown(r))
		}
	}()
	ret, err := h.call(target, s.Call, args)
	if err != nil {
		h.trace(label + " throws " + enc(err))
		return
	}
	if reportsResult.MatchString(s.Call) {
		h.trace(label + " = " + enc(ret))
	}
}

// call makes a step's call with JavaScript's argument positions: a missing
// argument is undefined (a nil callback).
func (h *harness) call(fs *FS, name string, args []any) (any, error) {
	a := func(i int) any {
		if i < len(args) {
			return args[i]
		}
		return jsvalue.Undefined{}
	}
	cb := func(i int) any {
		if i >= len(args) || undefinedArg(args[i]) {
			return nil
		}
		return args[i]
	}
	cbE := func(i int) func(error) {
		f, _ := cb(i).(func(error))
		if f == nil && cb(i) != nil {
			h.t.Fatalf("case %s: %s: argument %d is not a callback", h.name, name, i)
		}
		return f
	}
	cbV := func(i int) func(error, any) {
		f, _ := cb(i).(func(error, any))
		if f == nil && cb(i) != nil {
			h.t.Fatalf("case %s: %s: argument %d is not a callback", h.name, name, i)
		}
		return f
	}
	undefined := any(jsvalue.Undefined{})
	switch name {
	case "open":
		fs.Open(a(0), a(1), a(2), cbV(3))
	case "close":
		fs.Close(a(0), cbE(1))
	case "closeSync":
		return undefined, fs.CloseSync(a(0))
	case "readFile":
		fs.ReadFile(a(0), a(1), cbV(2))
	case "writeFile":
		fs.WriteFile(a(0), a(1), a(2), cbE(3))
	case "appendFile":
		fs.AppendFile(a(0), a(1), a(2), cbE(3))
	case "copyFile":
		fs.CopyFile(a(0), a(1), a(2), cbE(3))
	case "readdir":
		f, _ := cb(2).(func(error, []any))
		fs.Readdir(a(0), a(1), f)
	case "read":
		f, _ := cb(5).(func(error, any, any))
		fs.Read(a(0), a(1), a(2), a(3), a(4), f)
	case "readSync":
		return fs.ReadSync(a(0), a(1), a(2), a(3), a(4))
	case "rename":
		fs.Rename(a(0), a(1), cbE(2))
	case "chown":
		fs.Chown(a(0), a(1), a(2), cbE(3))
	case "fchown":
		fs.Fchown(a(0), a(1), a(2), cbE(3))
	case "lchown":
		fs.Lchown(a(0), a(1), a(2), cbE(3))
	case "chmod":
		fs.Chmod(a(0), a(1), cbE(2))
	case "fchmod":
		fs.Fchmod(a(0), a(1), cbE(2))
	case "lchmod":
		fs.Lchmod(a(0), a(1), cbE(2))
	case "chownSync":
		return undefined, fs.ChownSync(a(0), a(1), a(2))
	case "fchownSync":
		return undefined, fs.FchownSync(a(0), a(1), a(2))
	case "lchownSync":
		return undefined, fs.LchownSync(a(0), a(1), a(2))
	case "chmodSync":
		return undefined, fs.ChmodSync(a(0), a(1))
	case "fchmodSync":
		return undefined, fs.FchmodSync(a(0), a(1))
	case "lchmodSync":
		return undefined, fs.LchmodSync(a(0), a(1))
	case "stat", "fstat", "lstat":
		f, _ := cb(2).(func(error, *Stats))
		map[string]func(any, any, func(error, *Stats)){"stat": fs.Stat, "fstat": fs.Fstat, "lstat": fs.Lstat}[name](a(0), a(1), f)
	case "statSync", "fstatSync", "lstatSync":
		st, err := map[string]func(any, any) (*Stats, error){"statSync": fs.StatSync, "fstatSync": fs.FstatSync, "lstatSync": fs.LstatSync}[name](a(0), a(1))
		return st, err
	case "lutimes":
		fs.Lutimes(a(0), a(1), a(2), cbE(3))
	case "lutimesSync":
		return undefined, fs.LutimesSync(a(0), a(1), a(2))
	case "createReadStream":
		return fs.CreateReadStream(a(0), a(1)), nil
	case "createWriteStream":
		return fs.CreateWriteStream(a(0), a(1)), nil
	default:
		h.t.Fatalf("case %s: unknown call %s", h.name, name)
	}
	return undefined, nil
}

// hasNames is the oracle's HAS: the functions whose presence is recorded.
var hasNames = []string{"open", "openSync", "close", "closeSync", "readFile", "writeFile", "appendFile", "copyFile", "readdir",
	"read", "readSync", "rename", "chown", "fchown", "lchown", "chownSync", "fchownSync", "lchownSync",
	"chmod", "fchmod", "lchmod", "chmodSync", "fchmodSync", "lchmodSync", "stat", "fstat", "lstat",
	"statSync", "fstatSync", "lstatSync", "futimes", "futimesSync", "lutimes", "lutimesSync",
	"ReadStream", "WriteStream", "createReadStream", "createWriteStream"}

func hasList(fs *FS) []string {
	out := []string{}
	rv := reflect.ValueOf(fs).Elem()
	for _, n := range hasNames {
		if !rv.FieldByName(strings.ToUpper(n[:1]) + n[1:]).IsNil() {
			out = append(out, n)
		}
	}
	return out
}

func runCase(t *testing.T, c parityCase) caseResult {
	env := map[string]any{"platform": "linux", "uid": float64(1000), "osymlink": nil, "clock": float64(1700000000000)}
	for k, v := range c.Env {
		env[k] = v
	}
	start := int64(env["clock"].(float64))
	h := &harness{t: t, name: c.Name, v: jsloop.NewVirtual(start), start: start, hash: 2166136261}
	h.v.MaxTasks = 1000000
	h.v.Uncaught = func(r any) { h.trace("uncaught " + thrown(r)) }

	bases := c.Bases
	if bases == nil {
		bases = []baseSpec{{}}
	}
	if c.Missing != nil {
		bases[0].Missing = c.Missing
	}
	if c.Script != nil {
		bases[0].Script = c.Script
	}
	for i, b := range bases {
		f := &fakeFS{h: h, name: "fs", missing: map[string]bool{}, script: b.Script, cursors: map[string]*[2]int{}}
		if i > 0 {
			f.name = "fs" + strconv.Itoa(i)
		}
		for _, m := range b.Missing {
			f.missing[m] = true
		}
		f.fsv = f.build()
		h.fakes = append(h.fakes, f)
	}

	gfsPlatform, _ := env["gfsPlatform"].(string)
	rt := &Runtime{Host: h, Platform: platformOf(gfsPlatform, env["platform"].(string))}
	if uid, ok := env["uid"].(float64); ok {
		rt.Getuid = func() int { return int(uid) }
	}
	if o, ok := env["osymlink"].(float64); ok {
		rt.OSymlink = int(o)
	}
	load := c.Load
	if load == nil {
		load = []int{0}
	}
	for _, bi := range load {
		h.instances = append(h.instances, rt.Load(h.fakes[bi].fsv))
	}
	for i, s := range c.Steps {
		label := "s" + strconv.Itoa(i)
		h.v.After(s.At, func() { h.runStep(s, label) })
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if b, ok := r.(*brokenCase); ok {
					t.Fatal(b.msg)
				}
				panic(r)
			}
		}()
		h.v.Run()
	}()
	if h.broken != "" {
		t.Fatalf("case %s: %s", c.Name, h.broken)
	}
	res := caseResult{Trace: h.out, End: h.v.Now() - start, Timers: h.fired, TimerHash: fmt.Sprintf("%08x", h.hash)}
	if rt.global != nil {
		res.Queue = len(rt.global.items)
	}
	for _, g := range h.instances {
		res.Has = append(res.Has, hasList(g))
	}
	if res.Trace == nil {
		res.Trace = []string{}
	}
	return res
}

// runProgram is the oracle's runProgram, on jsloop.Virtual itself.
func runProgram(p program) []string {
	v := jsloop.NewVirtual(p.Start)
	log := []string{}
	v.Uncaught = func(r any) { log = append(log, fmt.Sprintf("%d uncaught %v", v.Now(), r)) }
	timers := map[string]any{}
	var exec func(o op)
	fire := func(o op) {
		id := o.ID
		if o.Tick != nil {
			id = *o.Tick
		}
		log = append(log, fmt.Sprintf("%d %s", v.Now(), id))
		if o.Panic {
			panic(id)
		}
		for _, t := range o.Then {
			exec(t)
		}
	}
	exec = func(o op) {
		switch {
		case o.Timeout != nil:
			timers[o.ID] = v.SetTimeout(func() { fire(o) }, *o.Timeout)
		case o.After != nil:
			v.After(*o.After, func() { fire(o) })
		case o.Tick != nil:
			v.NextTick(func() { fire(o) })
		case o.Clear != nil:
			v.ClearTimeout(timers[*o.Clear])
		}
	}
	for _, o := range p.Ops {
		exec(o)
	}
	v.Run()
	return log
}

func loadJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(path, err)
	}
}

// TestParity replays every case and diffs the whole trace with the one the
// original graceful-fs@4.2.11 produced (parity/oracle_gracefulfs.cjs).
func TestParity(t *testing.T) {
	var spec paritySpec
	var want struct {
		Node      string
		Scheduler [][]string
		Cases     []caseResult
	}
	loadJSON(t, "../../parity/gracefulfs/cases.json", &spec)
	loadJSON(t, "../../parity/gracefulfs/expected.json", &want)
	if len(want.Cases) != len(spec.Cases) || len(want.Scheduler) != len(spec.Scheduler) {
		t.Fatalf("expected.json has %d cases and %d programs for %d and %d: re-record it", len(want.Cases), len(want.Scheduler), len(spec.Cases), len(spec.Scheduler))
	}
	checks := map[string]int{}
	for i, p := range spec.Scheduler {
		got := runProgram(p)
		checks["scheduler"] += len(want.Scheduler[i])
		if !reflect.DeepEqual(got, want.Scheduler[i]) {
			t.Errorf("scheduler %q:\n--- js  %q\n+++ go  %q", p.Name, want.Scheduler[i], got)
		}
	}
	mismatched := 0
	for i, c := range spec.Cases {
		got, w := runCase(t, c), want.Cases[i]
		checks["trace"] += len(w.Trace)
		checks["summary"] += 5
		bad := false
		if n := firstDiff(w.Trace, got.Trace); n >= 0 {
			bad = true
			from := max(n-3, 0)
			t.Errorf("case %q: trace differs at line %d:\n--- js\n%s\n+++ go\n%s", c.Name, n,
				strings.Join(window(w.Trace, from, n+3), "\n"), strings.Join(window(got.Trace, from, n+3), "\n"))
		}
		if got.End != w.End || got.Timers != w.Timers || got.TimerHash != w.TimerHash || got.Queue != w.Queue {
			bad = true
			t.Errorf("case %q: end %d timers %d %s queue %d, want end %d timers %d %s queue %d", c.Name,
				got.End, got.Timers, got.TimerHash, got.Queue, w.End, w.Timers, w.TimerHash, w.Queue)
		}
		if !reflect.DeepEqual(got.Has, w.Has) {
			bad = true
			t.Errorf("case %q: functions\n--- js  %v\n+++ go  %v", c.Name, w.Has, got.Has)
		}
		if bad {
			mismatched++
		}
	}
	total := 0
	for _, n := range checks {
		total += n
	}
	t.Logf("%d cases, %d programs (Node %s): %d checks %v, %d mismatched cases", len(spec.Cases), len(spec.Scheduler), want.Node, total, checks, mismatched)
}

func firstDiff(a, b []string) int {
	for i := 0; i < max(len(a), len(b)); i++ {
		if i >= len(a) || i >= len(b) || a[i] != b[i] {
			return i
		}
	}
	return -1
}

func window(lines []string, from, to int) []string {
	if from > len(lines) {
		return nil
	}
	return lines[from:min(to, len(lines))]
}
