// Package buildinfo tells which commit the binary was built from, and when.
//
// The commit and its time come from the version control stamp that go build
// writes into the binary (go run and go test do not). The build time is not
// part of that stamp: the build scripts (deploy/deploy.sh, the CI archives)
// set it with
//
//	-ldflags "-X starbattle/internal/buildinfo.Time=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
package buildinfo

import (
	"runtime/debug"
	"time"
)

// Time is the build time in RFC 3339, empty unless the build set it.
var Time string

// Info is what is known about the build. Fields are empty when unknown.
type Info struct {
	Commit     string    `json:"commit,omitempty"`   // full hash
	Modified   bool      `json:"modified,omitempty"` // built with uncommitted changes
	CommitTime time.Time `json:"committed,omitzero"`
	BuildTime  time.Time `json:"built,omitzero"`
}

// Read returns the info of the running binary.
func Read() Info {
	var i Info
	if t, err := time.Parse(time.RFC3339, Time); err == nil {
		i.BuildTime = t.UTC()
	}
	b, ok := debug.ReadBuildInfo()
	if !ok {
		return i
	}
	for _, s := range b.Settings {
		switch s.Key {
		case "vcs.revision":
			i.Commit = s.Value
		case "vcs.modified":
			i.Modified = s.Value == "true"
		case "vcs.time":
			if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
				i.CommitTime = t.UTC()
			}
		}
	}
	return i
}

// Short is the abbreviated commit, with "+changes" when it was modified, or
// "unknown".
func (i Info) Short() string {
	if i.Commit == "" {
		return "unknown"
	}
	s := i.Commit[:min(7, len(i.Commit))]
	if i.Modified {
		s += "+changes"
	}
	return s
}
