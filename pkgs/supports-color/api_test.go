package supportscolor

import (
	"os"
	"reflect"
	"regexp"
	"runtime"
	"testing"
)

// TestProcessEnv checks what ProcessEnv reads from the real process (these
// values are machine-dependent, so they are not parity-checked).
func TestProcessEnv(t *testing.T) {
	env := ProcessEnv()
	if !reflect.DeepEqual(env.Argv, os.Args) {
		t.Errorf("Argv = %q, want os.Args", env.Argv)
	}
	if want := platform(runtime.GOOS); env.Platform != want {
		t.Errorf("Platform = %q, want %q", env.Platform, want)
	}
	if runtime.GOOS == "windows" {
		if env.Platform != "win32" || !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(env.OSRelease) {
			t.Errorf("Platform %q, OSRelease %q", env.Platform, env.OSRelease)
		}
		if path, ok := os.LookupEnv("Path"); ok && env.Env["PATH"] != path {
			t.Errorf("PATH not found under its upper-case name")
		}
	}
	if got := Load(env); levelJSON(got.Stdout) != levelJSON(Stdout) || levelJSON(got.Stderr) != levelJSON(Stderr) {
		t.Errorf("Stdout / Stderr differ from Load(ProcessEnv())")
	}
	for goos, want := range map[string]string{"windows": "win32", "linux": "linux", "darwin": "darwin", "illumos": "sunos", "freebsd": "freebsd"} {
		if got := platform(goos); got != want {
			t.Errorf("platform(%q) = %q, want %q", goos, got, want)
		}
	}
}
