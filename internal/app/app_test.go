//go:build windows

package app

import (
	"strings"
	"testing"
	"time"

	"ytmusic/internal/config"
	"ytmusic/internal/win32"
)

func TestFormatPlayTimeAndTargetHeight(t *testing.T) {
	if got := formatPlayTimeTooltip(-5); got != "0:00" {
		t.Fatalf("expected 0:00 for negative, got %q", got)
	}
	if got := formatPlayTimeTooltip(65); got != "1:05" {
		t.Fatalf("expected 1:05, got %q", got)
	}
	if got := formatPlayTimeTooltip(3665); got != "1h 01m" {
		t.Fatalf("expected 1h 01m, got %q", got)
	}

	if h := computeTargetHeight(false, false); h != baseHeight {
		t.Fatalf("expected mini height %d, got %d", baseHeight, h)
	}
	if h := computeTargetHeight(false, true); h != baseHeight+playlistHeight {
		t.Fatalf("expected queue height %d, got %d", baseHeight+playlistHeight, h)
	}
	if h := computeTargetHeight(true, true); h != baseHeight+videoHeight+playlistHeight {
		t.Fatalf("expected full height %d, got %d", baseHeight+videoHeight+playlistHeight, h)
	}
	def := config.DefaultAppState()
	if def.VideoQuality != 720 {
		t.Fatalf("expected default VideoQuality=720, got %d", def.VideoQuality)
	}
}

func TestAppTrailingPlaytimeAndResetStats(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	a := &App{
		state:            config.DefaultAppState(),
		baseTotalPlaySec: 120,
		sessionPlaySec:   15,
		playActive:       true,
		playAnchorTime:   time.Now().Add(-5 * time.Second),
	}
	a.state.TotalPlayTimeSec = 135

	a.mu.Lock()
	a.creditTrailingPlayTimeLocked()
	sess := a.sessionPlaySec
	tot := a.state.TotalPlayTimeSec
	a.mu.Unlock()

	if sess < 19.5 || tot < 139 {
		t.Fatalf("expected trailing playtime credited (>=19.5s session, >=139s total), got sess=%.2f tot=%d", sess, tot)
	}

	a.ResetPlayTimeStats()
	if a.baseTotalPlaySec != 0 || a.sessionPlaySec != 0 || a.state.TotalPlayTimeSec != 0 {
		t.Fatalf("expected ResetPlayTimeStats to zero all counters, got base=%d sess=%.2f total=%d",
			a.baseTotalPlaySec, a.sessionPlaySec, a.state.TotalPlayTimeSec)
	}

	// Verify long-title tray formatting preserves stats suffix within 120 UTF-16 units.
	longTitle := strings.Repeat("Very Long Taylor Swift Song Title ", 4)
	const prefix = "ytmusic :: "
	stats := "\nSession: 1:05 | Total: 12h 30m"
	maxTitleUnits := 120 - win32.UTF16Len(prefix) - win32.UTF16Len(stats)
	fullTip := prefix + win32.TruncateTooltip(longTitle, maxTitleUnits) + stats
	if win32.UTF16Len(fullTip) > 120 {
		t.Fatalf("expected full tray tooltip <= 120 UTF-16 units, got %d", win32.UTF16Len(fullTip))
	}
	if !strings.HasSuffix(fullTip, stats) {
		t.Fatalf("expected tray tooltip to preserve stats suffix, got %q", fullTip)
	}

	// Verify ResetPlayTimeStats increments statsEpoch and refreshAccountProfileSync errors when signed out.
	prevEpoch := a.statsEpoch
	a.ResetPlayTimeStats()
	if a.statsEpoch != prevEpoch+1 {
		t.Fatalf("expected statsEpoch incremented from %d to %d, got %d", prevEpoch, prevEpoch+1, a.statsEpoch)
	}
	if err := a.refreshAccountProfileSync(); err == nil {
		t.Fatalf("expected refreshAccountProfileSync error when signed out")
	}
}
