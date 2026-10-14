package gracefulfs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unicode/utf16"
)

// These tests cover what the parity tests cannot: the blocking functions on
// the real file system, the real-time host, error codes of real system errors
// and Go strings that are not valid UTF-8.

func TestBlockingFiles(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	if err := WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendFile(p, []byte(", world"), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, err := ReadFile(p); err != nil || string(b) != "hello, world" {
		t.Fatalf("ReadFile = %q, %v", b, err)
	}
	if err := AppendFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := Open(p, os.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []byte
	buf := make([]byte, 5)
	for {
		n, err := Read(f, buf)
		got = append(got, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if string(got) != "hello, world" {
		t.Fatalf("Read = %q", got)
	}
	if fi, err := Fstat(f); err != nil || fi.Size() != 12 {
		t.Fatalf("Fstat = %v, %v", fi, err)
	}
	if err := Close(f); err != nil {
		t.Fatal(err)
	}
	if err := Close(f); err == nil {
		t.Fatal("closing twice succeeded")
	}

	_, err = Open(filepath.Join(dir, "missing"), os.O_RDONLY, 0)
	if !errors.Is(err, os.ErrNotExist) || errCode(err) != "ENOENT" {
		t.Fatalf("Open(missing) = %v (code %q)", err, errCode(err))
	}
	if _, err := ReadFile(filepath.Join(dir, "missing")); errCode(err) != "ENOENT" {
		t.Fatalf("ReadFile(missing) = %v", err)
	}

	if fi, err := Stat(p); err != nil || fi.Size() != 12 || fi.Name() != "f.txt" {
		t.Fatalf("Stat = %v, %v", fi, err)
	}
	if fi, err := Lstat(p); err != nil || fi.Size() != 12 {
		t.Fatalf("Lstat = %v, %v", fi, err)
	}

	// On Windows package os changes the attribute through the file's handle,
	// which must be open for writing (see Fchmod).
	f, err = Open(p, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := Fchmod(f, 0o444); err != nil {
		t.Fatal(err)
	}
	Close(f)
	if fi, _ := Stat(p); fi.Mode().Perm()&0o200 != 0 {
		t.Fatalf("Fchmod 0444 left mode %v", fi.Mode())
	}
	if err := Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if fi, _ := Stat(p); fi.Mode().Perm()&0o200 == 0 {
		t.Fatalf("Chmod 0644 left mode %v", fi.Mode())
	}
	// lchmod is graceful-fs's no-op: the module has none.
	if err := Lchmod(p, 0o400); err != nil {
		t.Fatal(err)
	}
	if fi, _ := Stat(p); fi.Mode().Perm()&0o200 == 0 {
		t.Fatalf("Lchmod changed the mode to %v", fi.Mode())
	}
	if err := Chmod(filepath.Join(dir, "missing"), 0o644); errCode(err) != "ENOENT" {
		t.Fatalf("Chmod(missing) = %v", err)
	}

	// chown to root succeeds as root, and its EPERM is ignored otherwise (on
	// Windows chown does nothing, as in Node).
	if err := Chown(p, 0, 0); err != nil {
		t.Fatalf("Chown = %v", err)
	}
	if err := Lchown(p, os.Getuid(), os.Getgid()); err != nil && runtime.GOOS != "windows" {
		t.Fatalf("Lchown = %v", err)
	}
	f, _ = Open(p, os.O_RDONLY, 0)
	if err := Fchown(f, 0, 0); err != nil {
		t.Fatalf("Fchown = %v", err)
	}
	Close(f)

	q := filepath.Join(dir, "g.txt")
	if err := Rename(p, q); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(p, []byte("again"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Rename(p, q); err != nil { // onto an existing file
		t.Fatal(err)
	}
	if b, _ := ReadFile(q); string(b) != "again" {
		t.Fatalf("after Rename: %q", b)
	}
	if err := Rename(p, q); errCode(err) != "ENOENT" {
		t.Fatalf("Rename(missing) = %v", err)
	}

	w, err := CreateWriteStream(filepath.Join(dir, "s.txt"))
	if err != nil {
		t.Fatal(err)
	}
	w.WriteString("stream")
	Close(w)
	r, err := CreateReadStream(filepath.Join(dir, "s.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := io.ReadAll(r); string(b) != "stream" {
		t.Fatalf("stream read %q", b)
	}
	Close(r)
	if _, err := CreateReadStream(filepath.Join(dir, "missing")); errCode(err) != "ENOENT" {
		t.Fatalf("CreateReadStream(missing) = %v", err)
	}
}

// ReadDir sorts names as JavaScript sorts strings, by UTF-16 code units:
// U+1F600 (a surrogate pair) comes before U+E000, unlike in Go's byte order.
func TestBlockingReadDir(t *testing.T) {
	dir := t.TempDir()
	// No two names differ only in case (Windows and macOS file systems fold
	// it), and none has a decomposable character (HFS+ decomposes them).
	names := []string{"b", "a", "😀", "", "C", "10", "9", "日"}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Skipf("file system rejects %q: %v", n, err)
		}
	}
	got, err := ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"10", "9", "C", "a", "b", "日", "😀", ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadDir = %q, want %q", got, want)
	}
	byteOrder := slices.Clone(want)
	sort.Strings(byteOrder)
	if reflect.DeepEqual(byteOrder, want) {
		t.Fatal("the names do not tell UTF-16 order from byte order")
	}

	entries, err := ReadDirEntries(dir)
	if err != nil {
		t.Fatal(err)
	}
	var entryNames []string
	for _, e := range entries {
		entryNames = append(entryNames, e.Name())
	}
	if runtime.GOOS != "windows" {
		// libuv's scandir order (strcmp), which graceful-fs keeps.
		if !slices.IsSorted(entryNames) {
			t.Fatalf("ReadDirEntries = %q, want strcmp order", entryNames)
		}
	}
	slices.Sort(entryNames)
	if !reflect.DeepEqual(entryNames, byteOrder) {
		t.Fatalf("ReadDirEntries names = %q", entryNames)
	}
	if _, err := ReadDir(filepath.Join(dir, "missing")); errCode(err) != "ENOENT" {
		t.Fatalf("ReadDir(missing) = %v", err)
	}
}

// The blocking functions may be called from many goroutines at once.
func TestBlockingConcurrent(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := filepath.Join(dir, fmt.Sprint(i))
			if err := WriteFile(p, []byte(fmt.Sprint(i)), 0o644); err != nil {
				t.Error(err)
				return
			}
			b, err := ReadFile(p)
			if err != nil || string(b) != fmt.Sprint(i) {
				t.Errorf("ReadFile = %q, %v", b, err)
			}
			if _, err := ReadDir(dir); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	names, _ := ReadDir(dir)
	if len(names) != 40 {
		t.Fatalf("%d files", len(names))
	}
}

// A rename blocked by an open handle (anti-virus software does this on
// Windows) is retried until the handle closes.
func TestBlockingRenameRetry(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("a Windows file lock")
	}
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := os.Open(src) // opened without FILE_SHARE_DELETE
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(src, dst); err == nil {
		t.Skip("an open file can be renamed here")
	} else if c := errCode(err); c != "EBUSY" && c != "EPERM" && c != "EACCES" {
		t.Fatalf("os.Rename of an open file: %v (code %q)", err, c)
	}
	go func() {
		time.Sleep(150 * time.Millisecond)
		lock.Close()
	}()
	start := time.Now()
	if err := Rename(src, dst); err != nil {
		t.Fatalf("Rename = %v", err)
	}
	if d := time.Since(start); d < 100*time.Millisecond {
		t.Fatalf("Rename returned after %v, before the lock was released", d)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatal(err)
	}
}

type codeError string

func (e codeError) Error() string { return "error " + string(e) }
func (e codeError) Code() string  { return string(e) }

func TestErrCode(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{errors.New("plain"), ""},
		{codeError("EMFILE"), "EMFILE"},
		{fmt.Errorf("wrapped: %w", codeError("ENFILE")), "ENFILE"},
		{&os.PathError{Op: "open", Path: "x", Err: syscall.ENOENT}, "ENOENT"},
		{fmt.Errorf("w: %w", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.ENOENT}), "ENOENT"},
	}
	if runtime.GOOS != "windows" {
		for _, e := range []syscall.Errno{syscall.EMFILE, syscall.ENFILE, syscall.EAGAIN, syscall.ENOSYS, syscall.EINVAL,
			syscall.EPERM, syscall.EACCES, syscall.EBUSY, syscall.ENOENT} {
			cases = append(cases, struct {
				err  error
				want string
			}{&os.PathError{Op: "x", Path: "y", Err: e}, errnoName(e)})
		}
		cases = append(cases, struct {
			err  error
			want string
		}{syscall.EEXIST, ""})
	}
	for _, c := range cases {
		if got := errCode(c.err); got != c.want {
			t.Errorf("errCode(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

// errnoName is the symbol of the errnos sysErrCode names on Unix.
func errnoName(e syscall.Errno) string {
	return map[syscall.Errno]string{syscall.EMFILE: "EMFILE", syscall.ENFILE: "ENFILE", syscall.EAGAIN: "EAGAIN",
		syscall.ENOSYS: "ENOSYS", syscall.EINVAL: "EINVAL", syscall.EPERM: "EPERM", syscall.EACCES: "EACCES",
		syscall.EBUSY: "EBUSY", syscall.ENOENT: "ENOENT"}[e]
}

// A Go string read from a file system may hold bytes that are not UTF-8; its
// sort key is the string Node would have decoded from them, as for a Buffer.
func TestSortInvalidUTF8(t *testing.T) {
	cases := map[string][]uint16{
		"\xf0\x9f\x98":         {0xfffd},
		"\xe2\x82":             {0xfffd},
		"\xed\xa0\x80":         {0xfffd, 0xfffd, 0xfffd},
		"\xc0\x80":             {0xfffd, 0xfffd},
		"\xf4\x90\x80\x80":     {0xfffd, 0xfffd, 0xfffd, 0xfffd},
		"\xe0\x80\x80":         {0xfffd, 0xfffd, 0xfffd},
		"a\xf0\x9fb":           {'a', 0xfffd, 'b'},
		"😀":                    utf16.Encode([]rune("😀")),
		"\U0010ffffé":          utf16.Encode([]rune("\U0010ffffé")),
		"\xff\xfe":             {0xfffd, 0xfffd},
		"\xc3":                 {0xfffd},
		"\xf0\x90\x80\x80\xbf": {0xd800, 0xdc00, 0xfffd},
	}
	for s, want := range cases {
		if got := decodeUTF8([]byte(s)); !reflect.DeepEqual(got, want) {
			t.Errorf("decodeUTF8(%q) = %x, want %x", s, got, want)
		}
	}
	files := []any{"b", "\xff", "a\xf0\x9fb", []byte("\xf0\x9f\x98"), "\xef\xbf\xbd", "a", "\xff\xff"}
	sortFiles(files)
	want := []any{"a", "a\xf0\x9fb", "b", "\xff", []byte("\xf0\x9f\x98"), "\xef\xbf\xbd", "\xff\xff"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("sortFiles = %q, want %q", files, want)
	}
}

func TestRealHost(t *testing.T) {
	h := newRealHost()
	if d := h.Now() - time.Now().UnixMilli(); d < -5 || d > 5 {
		t.Fatalf("Now is %d ms off", d)
	}
	var mu sync.Mutex
	var order []string
	log := func(s string) { mu.Lock(); order = append(order, s); mu.Unlock() }
	done := make(chan struct{})
	h.post(func() {
		h.SetTimeout(func() { log("t20") }, 20)
		cleared := h.SetTimeout(func() { log("never") }, 5)
		h.ClearTimeout(cleared)
		h.ClearTimeout(nil)
		h.SetTimeout(func() { log("t0"); h.SetTimeout(func() { log("t40"); close(done) }, 40) }, 0)
		h.NextTick(func() { log("tick") })
		log("task")
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timers did not run")
	}
	if got := fmt.Sprint(order); got != "[task tick t0 t20 t40]" {
		t.Fatalf("order %s", got)
	}
}

func TestPlatform(t *testing.T) {
	for goos, want := range map[string]string{"windows": "win32", "linux": "linux", "darwin": "darwin", "illumos": "sunos",
		"solaris": "sunos", "freebsd": "freebsd", "android": "android"} {
		if got := nodePlatform(goos); got != want {
			t.Errorf("nodePlatform(%q) = %q, want %q", goos, got, want)
		}
	}
	if platformOf("win32", "linux") != "win32" || platformOf("", "linux") != "linux" {
		t.Error("platformOf")
	}
	if want := platformOf(os.Getenv("GRACEFUL_FS_PLATFORM"), nodePlatform(runtime.GOOS)); std.rt.Platform != want {
		t.Errorf("the real file system's platform is %q, want %q", std.rt.Platform, want)
	}
	if (processGetuid() == nil) != (runtime.GOOS == "windows") {
		t.Error("process.getuid exists exactly off Windows")
	}
}

// emfileFS is an fs module in real time with a limit of open files: open
// fails with EMFILE while limit files are open.
type emfileFS struct {
	mu            sync.Mutex
	open, maxOpen int
	limit         int
	emfiles       atomic.Int64
}

func (e *emfileFS) module(post func(func())) *FS {
	return &FS{
		Open: func(path, flags, mode any, cb func(error, any)) {
			go func() {
				e.mu.Lock()
				var err error
				if e.open >= e.limit {
					err = codeError("EMFILE")
					e.emfiles.Add(1)
				} else {
					e.open++
					e.maxOpen = max(e.maxOpen, e.open)
				}
				e.mu.Unlock()
				post(func() {
					if err != nil {
						cb(err, nil)
						return
					}
					cb(nil, path)
				})
			}()
		},
		Close: func(fd any, cb func(error)) {
			go func() {
				e.mu.Lock()
				e.open--
				e.mu.Unlock()
				post(func() { cb(nil) })
			}()
		},
		CloseSync: func(fd any) error { return nil },
	}
}

// Many goroutines opening through graceful-fs against a limit of open files:
// the opens beyond the limit wait on the queue and each close lets one in.
func TestRealTimeQueue(t *testing.T) {
	e := &emfileFS{limit: 4}
	r := newRealFS(e.module)
	var wg sync.WaitGroup
	var opened atomic.Int64
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var fd any
			var err error
			r.call(func(done func()) {
				r.fs.Open(fmt.Sprint(i), 0, nil, func(e error, f any) { fd, err = f, e; done() })
			})
			if err != nil {
				t.Errorf("open %d: %v", i, err)
				return
			}
			opened.Add(1)
			time.Sleep(time.Millisecond)
			if err := r.errCall(func(cb func(error)) { r.fs.Close(fd, cb) }); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if opened.Load() != 40 || e.maxOpen > e.limit || e.emfiles.Load() == 0 {
		t.Fatalf("opened %d, at most %d open (limit %d), %d EMFILE", opened.Load(), e.maxOpen, e.limit, e.emfiles.Load())
	}
	var left int
	r.call(func(done func()) { left = len(r.rt.global.items); done() })
	if left != 0 {
		t.Fatalf("%d calls left on the queue", left)
	}
}
