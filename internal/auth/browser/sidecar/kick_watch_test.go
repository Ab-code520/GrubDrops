package sidecar

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// parseWatchAlive must report alive ONLY when the IVS <video> exists and
// is actually playing. A missing video, a paused player, or malformed
// probe output all count as not-alive so the watcher re-picks a channel
// instead of holding a dead tab.
func TestParseWatchAlive(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   bool
	}{
		{"playing", `{"video":true,"playing":true,"readyState":4}`, true},
		{"paused", `{"video":true,"playing":false,"readyState":4}`, false},
		{"no video element", `{"video":false,"playing":false}`, false},
		{"video present but not playing", `{"video":true,"playing":false}`, false},
		{"empty string", ``, false},
		{"malformed json", `not json`, false},
		{"probe error shape", `{"video":false,"playing":false,"err":"boom"}`, false},
		{"extra fields ignored", `{"video":true,"playing":true,"foo":"bar"}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseWatchAlive(tc.status))
		})
	}
}
