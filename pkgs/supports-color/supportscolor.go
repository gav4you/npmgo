// Package supportscolor is a Go translation of the npm package
// supports-color@7.2.0 (https://github.com/chalk/supports-color, the Node
// build): whether a terminal supports colour, and at which level. The original
// copyright notice and MIT license are preserved in LICENSE.
//
// The JavaScript module inspects the process when it is loaded:
// process.argv (through has-flag), process.env, process.platform,
// os.release() and tty.isatty(1) / tty.isatty(2). Here that state is an
// explicit Env, and Load is the module body as a pure function of it. Stdout,
// Stderr and SupportsColor are the module loaded for this process
// (ProcessEnv) when the package is initialised.
//
// Version 7 does not read NO_COLOR; FORCE_COLOR, the --color / --no-color
// flags, CI and terminal variables decide.
package supportscolor

import (
	"math"
	"os"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/term"

	"github.com/gav4you/npmgo/internal/jsnum"
	hasflag "github.com/gav4you/npmgo/pkgs/has-flag"
)

// Env is the process state supports-color reads.
type Env struct {
	// Argv is process.argv, where the --color / --no-color flags are looked
	// for (has-flag).
	Argv []string
	// Env is process.env. Names are case-sensitive here; on Windows Node
	// looks them up case-insensitively (ProcessEnv upper-cases them there).
	Env map[string]string
	// Platform is process.platform: "win32", "linux", "darwin", …
	Platform string
	// OSRelease is os.release(), which is read only on win32 ("10.0.19045").
	OSRelease string
	// StdoutTTY and StderrTTY are tty.isatty(1) and tty.isatty(2).
	StdoutTTY, StderrTTY bool
}

// Level is the object translateLevel returns for a level other than 0,
// {level, hasBasic, has256, has16m}. Where JavaScript has false (level 0)
// this package returns nil.
type Level struct {
	Level    int
	HasBasic bool
	Has256   bool
	Has16m   bool
}

// Stream is the stream supportsColor(stream) inspects: only its isTTY.
type Stream struct{ IsTTY bool }

// Module is what require('supports-color') evaluates to in one environment.
type Module struct {
	// Stdout and Stderr are exports.stdout and exports.stderr.
	Stdout, Stderr *Level

	env        Env
	forceColor float64 // valid when forced; NaN for FORCE_COLOR=abc, -0 for -0
	forced     bool    // forceColor !== undefined
}

// Load evaluates the module body for env and returns its exports.
func Load(env Env) *Module {
	m := &Module{env: env}
	switch {
	case m.hasFlag("no-color") || m.hasFlag("no-colors") || m.hasFlag("color=false") || m.hasFlag("color=never"):
		m.forced, m.forceColor = true, 0
	case m.hasFlag("color") || m.hasFlag("colors") || m.hasFlag("color=true") || m.hasFlag("color=always"):
		m.forced, m.forceColor = true, 1
	}
	if force, ok := env.Env["FORCE_COLOR"]; ok {
		m.forced = true
		switch {
		case force == "true":
			m.forceColor = 1
		case force == "false":
			m.forceColor = 0
		case force == "":
			m.forceColor = 1
		default:
			m.forceColor = min(jsnum.ParseIntRadix(force, 10), 3)
		}
	}
	m.Stdout = translateLevel(m.supportsColor(true, env.StdoutTTY))
	m.Stderr = translateLevel(m.supportsColor(true, env.StderrTTY))
	return m
}

func (m *Module) hasFlag(flag string) bool { return hasflag.HasFlag(flag, m.env.Argv) }

// SupportsColor is exports.supportsColor(stream): the level for a stream.
// A nil stream is an absent one (supportsColor(undefined)).
func (m *Module) SupportsColor(stream *Stream) *Level {
	if stream == nil {
		return translateLevel(m.supportsColor(false, false))
	}
	return translateLevel(m.supportsColor(true, stream.IsTTY))
}

// translateLevel returns nil for level 0. Levels come from Math.min of a
// parsed FORCE_COLOR and may be negative; beyond the int range they
// saturate.
func translateLevel(level float64) *Level {
	if level == 0 {
		return nil
	}
	n := math.MaxInt
	if level < math.MaxInt {
		n = int(max(level, math.MinInt))
	}
	return &Level{Level: n, HasBasic: true, Has256: level >= 2, Has16m: level >= 3}
}

var (
	teamcity = regexp.MustCompile(`^(9\.(0*[1-9]\d*)\.|\d{2,}\.)`)
	term256  = regexp.MustCompile(`-256(color)?$`)
	termRe   = regexp.MustCompile(`^screen|^xterm|^vt100|^vt220|^rxvt|color|ansi|cygwin|linux`)
)

// asciiLower lower-cases A-Z only: JavaScript's case-insensitive regular
// expressions (without the u flag) fold no other character onto these
// patterns' letters (U+017F long s does not match s).
func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, s)
}

func (m *Module) supportsColor(haveStream, streamIsTTY bool) float64 {
	env := m.env.Env
	if m.forced && m.forceColor == 0 {
		return 0
	}
	if m.hasFlag("color=16m") || m.hasFlag("color=full") || m.hasFlag("color=truecolor") {
		return 3
	}
	if m.hasFlag("color=256") {
		return 2
	}
	if haveStream && !streamIsTTY && !m.forced {
		return 0
	}
	minLevel := 0.0 // forceColor || 0
	if m.forced && !math.IsNaN(m.forceColor) {
		minLevel = m.forceColor + 0 // -0 is falsy too
	}
	if v, ok := env["TERM"]; ok && v == "dumb" {
		return minLevel
	}
	if m.env.Platform == "win32" {
		// Windows 10 build 10586 is the first Windows release that supports
		// 256 colors. Windows 10 build 14931 is the first release that
		// supports 16m/TrueColor.
		osRelease := strings.Split(m.env.OSRelease, ".")
		part := func(i int) float64 {
			if i < len(osRelease) {
				return jsnum.Number(osRelease[i])
			}
			return math.NaN() // Number(undefined)
		}
		if part(0) >= 10 && part(2) >= 10586 {
			if part(2) >= 14931 {
				return 3
			}
			return 2
		}
		return 1
	}
	if _, ok := env["CI"]; ok {
		for _, sign := range []string{"TRAVIS", "CIRCLECI", "APPVEYOR", "GITLAB_CI", "GITHUB_ACTIONS", "BUILDKITE"} {
			if _, ok := env[sign]; ok {
				return 1
			}
		}
		if v, ok := env["CI_NAME"]; ok && v == "codeship" {
			return 1
		}
		return minLevel
	}
	if v, ok := env["TEAMCITY_VERSION"]; ok {
		if teamcity.MatchString(v) {
			return 1
		}
		return 0
	}
	if v, ok := env["COLORTERM"]; ok && v == "truecolor" {
		return 3
	}
	if program, ok := env["TERM_PROGRAM"]; ok {
		version := jsnum.ParseIntRadix(strings.Split(env["TERM_PROGRAM_VERSION"], ".")[0], 10)
		switch program {
		case "iTerm.app":
			if version >= 3 {
				return 3
			}
			return 2
		case "Apple_Terminal":
			return 2
		}
	}
	term, ok := env["TERM"]
	if !ok {
		term = "undefined" // RegExp.prototype.test(undefined) tests "undefined"
	}
	if term256.MatchString(asciiLower(term)) {
		return 2
	}
	if termRe.MatchString(asciiLower(term)) {
		return 1
	}
	if _, ok := env["COLORTERM"]; ok {
		return 1
	}
	return minLevel
}

// ProcessEnv reads this process's state: os.Args, the environment (with the
// names upper-cased on Windows, where Node's process.env ignores their case),
// runtime.GOOS as a Node platform name, the Windows version as os.release()
// prints it (empty elsewhere, where supports-color does not read it) and
// whether stdout and stderr are terminals.
func ProcessEnv() Env {
	env := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		if runtime.GOOS == "windows" {
			k = strings.ToUpper(k)
		}
		if _, dup := env[k]; !dup {
			env[k] = v
		}
	}
	return Env{
		Argv:      os.Args,
		Env:       env,
		Platform:  platform(runtime.GOOS),
		OSRelease: osRelease(),
		StdoutTTY: term.IsTerminal(int(os.Stdout.Fd())),
		StderrTTY: term.IsTerminal(int(os.Stderr.Fd())),
	}
}

// platform maps runtime.GOOS to process.platform.
func platform(goos string) string {
	switch goos {
	case "windows":
		return "win32"
	case "solaris", "illumos":
		return "sunos"
	}
	return goos
}

var process = Load(ProcessEnv())

var (
	// Stdout is require('supports-color').stdout for this process: its
	// colour level when the package was initialised, nil if none.
	Stdout = process.Stdout
	// Stderr is require('supports-color').stderr for this process.
	Stderr = process.Stderr
)

// SupportsColor is require('supports-color').supportsColor(stream) for this
// process (see Module.SupportsColor).
func SupportsColor(stream *Stream) *Level { return process.SupportsColor(stream) }
