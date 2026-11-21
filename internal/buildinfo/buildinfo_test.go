package buildinfo

import (
	"testing"
	"time"
)

// Test binaries carry no version control stamp, so only the build time set
// by -ldflags and the fallbacks can be checked here; the UI check sees the
// commit on the settings page of a real build.
func TestRead(t *testing.T) {
	old := Time
	defer func() { Time = old }()

	Time = "2026-11-21T08:31:53Z"
	if got, want := Read().BuildTime, time.Date(2026, 11, 21, 8, 31, 53, 0, time.UTC); !got.Equal(want) {
		t.Errorf("BuildTime = %v, want %v", got, want)
	}
	Time = "yesterday"
	if got := Read().BuildTime; !got.IsZero() {
		t.Errorf("BuildTime of an invalid Time = %v, want zero", got)
	}
}

func TestShort(t *testing.T) {
	for _, c := range []struct {
		i    Info
		want string
	}{
		{Info{}, "unknown"},
		{Info{Commit: "c023518d7b6f2fdf1145490a04fdb97b4e169c4d"}, "c023518"},
		{Info{Commit: "c023518d7b6f2fdf1145490a04fdb97b4e169c4d", Modified: true}, "c023518+changes"},
	} {
		if got := c.i.Short(); got != c.want {
			t.Errorf("%+v.Short() = %q, want %q", c.i, got, c.want)
		}
	}
}
