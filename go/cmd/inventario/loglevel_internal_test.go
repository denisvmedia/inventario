package main

// White-box: logLevel is unexported and the package is main, so there is no
// external test package to put this in.

import (
	"log/slog"
	"testing"

	qt "github.com/frankban/quicktest"
)

// A deployment that never sets the variable has to keep the level it has
// today, and a typo has to land there too rather than silently selecting
// something quieter or noisier than intended.
func TestLogLevel(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want slog.Level
	}{
		{"", slog.LevelInfo},
		{"   ", slog.LevelInfo},
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"Info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{" debug ", slog.LevelDebug},
		// The offset form comes from slog itself; worth pinning because it is
		// the only way to ask for verbosity between two named levels.
		{"debug+2", slog.LevelDebug + 2},
		{"info-4", slog.LevelDebug},
		// Anything else falls back rather than failing the boot: a logging
		// typo should not be the reason a deployment will not start.
		{"verbose", slog.LevelInfo},
		{"trace", slog.LevelInfo},
		{"7", slog.LevelInfo},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			c := qt.New(t)
			c.Check(logLevel(tc.raw), qt.Equals, tc.want)
		})
	}
}

// The whole point of the setting: slog.Debug calls are unreachable at the
// default level and reachable below it.
func TestLogLevel_DebugIsReachableOnlyBelowInfo(t *testing.T) {
	c := qt.New(t)
	c.Check(slog.LevelDebug >= logLevel(""), qt.IsFalse)
	c.Check(slog.LevelDebug >= logLevel("debug"), qt.IsTrue)
}
