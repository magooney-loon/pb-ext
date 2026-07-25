package monitoring

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectDiskInfo(t *testing.T) {
	info, err := CollectDiskInfo("")
	if err != nil {
		t.Skipf("disk collection unavailable: %v", err)
	}

	if info.Path != DefaultDiskPath {
		t.Errorf("Path = %q, want %q", info.Path, DefaultDiskPath)
	}
	if info.Total == 0 {
		t.Skip("no disk capacity reported")
	}
	if info.Used > info.Total {
		t.Errorf("Used (%d) exceeds Total (%d)", info.Used, info.Total)
	}
	if info.Free > info.Total {
		t.Errorf("Free (%d) exceeds Total (%d)", info.Free, info.Total)
	}
	if info.Usage < 0 || info.Usage > 100 {
		t.Errorf("Usage = %.2f, want within [0,100]", info.Usage)
	}
}

// TestCollectDiskInfo_UsageExcludesReservedBlocks pins the ratio the dashboard
// must display. Free counts blocks available to unprivileged users while Total
// counts every block, so Used/Total understates how full the disk is; the
// correct figure — and the one df prints — is Used/(Used+Free).
func TestCollectDiskInfo_UsageExcludesReservedBlocks(t *testing.T) {
	info, err := CollectDiskInfo("")
	if err != nil {
		t.Skipf("disk collection unavailable: %v", err)
	}
	if info.Used+info.Free == 0 {
		t.Skip("no usable disk reported")
	}

	want := float64(info.Used) / float64(info.Used+info.Free) * 100
	if diff := math.Abs(info.Usage - want); diff > 0.1 {
		t.Errorf("Usage = %.3f, want %.3f (Used/(Used+Free))", info.Usage, want)
	}

	// The naive formula is a different number whenever blocks are reserved.
	naive := float64(info.Used) / float64(info.Total) * 100
	if info.Used+info.Free < info.Total {
		t.Logf("reserved blocks present: Usage=%.2f%% vs naive Used/Total=%.2f%%", info.Usage, naive)
		if naive > info.Usage+0.001 {
			t.Errorf("naive formula (%.3f) should be lower than the reported usage (%.3f)", naive, info.Usage)
		}
	}
}

func TestCollectDiskInfoContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectDiskInfoWithContext(ctx, "")
	if err == nil {
		t.Fatal("expected an error for a cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestCollectDiskInfoContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	_, err := CollectDiskInfoWithContext(ctx, "")
	if err == nil {
		t.Fatal("expected an error for a timed out context")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestCollectDiskInfoStable(t *testing.T) {
	first, err := CollectDiskInfo("")
	if err != nil {
		t.Skipf("disk collection unavailable: %v", err)
	}
	second, err := CollectDiskInfo("")
	if err != nil {
		t.Skipf("disk collection unavailable: %v", err)
	}

	// Capacity does not change between two consecutive reads.
	if first.Total != second.Total {
		t.Errorf("Total changed between reads: %d then %d", first.Total, second.Total)
	}
}

func TestIsDiskTemp(t *testing.T) {
	matches := []string{"nvme", "NVMe Composite", "drive_temp", "hdd", "SSD", "disk1"}
	for _, s := range matches {
		if !IsDiskTemp(s) {
			t.Errorf("IsDiskTemp(%q) = false, want true", s)
		}
	}

	nonMatches := []string{"coretemp", "k10temp", "acpitz", "cpu_thermal", ""}
	for _, s := range nonMatches {
		if IsDiskTemp(s) {
			t.Errorf("IsDiskTemp(%q) = true, want false", s)
		}
	}
}

func TestGetDiskTemperature(t *testing.T) {
	temp, ok := GetDiskTemperature()
	if !ok {
		t.Skip("no disk temperature sensor on this host")
	}
	if temp <= 0 || temp > 150 {
		t.Errorf("disk temperature = %.1f, want a plausible celsius reading", temp)
	}
}

func TestGetDiskTemperatureContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, ok := GetDiskTemperatureWithContext(ctx); ok {
		t.Error("expected no reading for a cancelled context")
	}
}

func TestDiskInfoZeroValue(t *testing.T) {
	var info DiskInfo

	if info.Total != 0 || info.Used != 0 || info.Free != 0 || info.Usage != 0 {
		t.Error("zero value should have zeroed fields")
	}
	if info.Path != "" {
		t.Errorf("Path = %q, want empty", info.Path)
	}
}

// --- path selection ---

// TestCollectDiskInfo_MeasuresRequestedPath is the point of taking a path at
// all: a PocketBase server cares about the filesystem holding pb_data, which in
// a container is a different device from "/".
func TestCollectDiskInfo_MeasuresRequestedPath(t *testing.T) {
	dir := t.TempDir()

	info, err := CollectDiskInfo(dir)
	if err != nil {
		t.Skipf("disk collection unavailable: %v", err)
	}

	if info.Path != dir {
		t.Errorf("Path = %q, want %q — the requested path must be reported", info.Path, dir)
	}
	if info.Total == 0 {
		t.Error("Total = 0 for an existing directory")
	}
}

func TestCollectDiskInfo_EmptyPathUsesDefault(t *testing.T) {
	info, err := CollectDiskInfo("")
	if err != nil {
		t.Skipf("disk collection unavailable: %v", err)
	}

	if info.Path != DefaultDiskPath {
		t.Errorf("Path = %q, want %q for an empty request", info.Path, DefaultDiskPath)
	}
}

// TestCollectDiskInfo_FallsBackWhenPathMissing covers a first run where the
// data directory has not been created yet: the card should still show
// something rather than an error and a row of zeroes.
func TestCollectDiskInfo_FallsBackWhenPathMissing(t *testing.T) {
	info, err := CollectDiskInfo(filepath.Join(t.TempDir(), "does", "not", "exist"))
	if err != nil {
		t.Skipf("disk collection unavailable: %v", err)
	}

	if info.Path != DefaultDiskPath {
		t.Errorf("Path = %q, want the %q fallback", info.Path, DefaultDiskPath)
	}
	if info.Total == 0 {
		t.Error("fallback produced no capacity")
	}
}

// TestCollectDiskInfo_DistinguishesFilesystems checks the path argument is
// actually honoured rather than quietly ignored, when a separate filesystem is
// available to compare against.
func TestCollectDiskInfo_DistinguishesFilesystems(t *testing.T) {
	root, err := CollectDiskInfo(DefaultDiskPath)
	if err != nil {
		t.Skipf("disk collection unavailable: %v", err)
	}

	// /dev/shm is a tmpfs on Linux and therefore a different device from /.
	const shm = "/dev/shm"
	if _, statErr := os.Stat(shm); statErr != nil {
		t.Skip("no separate filesystem available to compare against")
	}

	other, err := CollectDiskInfo(shm)
	if err != nil {
		t.Skipf("cannot query %s: %v", shm, err)
	}
	if other.Path != shm {
		t.Fatalf("Path = %q, want %q", other.Path, shm)
	}
	if other.Total == root.Total {
		t.Skip("both paths report the same capacity; not separate filesystems here")
	}
	t.Logf("%s total=%d vs %s total=%d — path argument is honoured",
		DefaultDiskPath, root.Total, shm, other.Total)
}

// --- stats plumbing ---

func TestCollectSystemStats_ReportsDiskPath(t *testing.T) {
	dir := t.TempDir()

	stats, err := CollectSystemStats(context.Background(), time.Now(), dir)
	if err != nil {
		t.Logf("partial stats: %v", err)
	}
	if stats == nil {
		t.Fatal("no stats returned")
	}

	if stats.DiskPath != dir {
		t.Errorf("DiskPath = %q, want %q", stats.DiskPath, dir)
	}
}

// TestCollectSystemStats_CacheIsKeyedOnDiskPath guards against the refresh
// cache handing back another filesystem's figures when the path changes.
func TestCollectSystemStats_CacheIsKeyedOnDiskPath(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()

	a, err := CollectSystemStats(context.Background(), time.Now(), first)
	if err != nil {
		t.Logf("partial stats: %v", err)
	}
	// Immediately after, well inside StatsRefreshInterval.
	b, err := CollectSystemStats(context.Background(), time.Now(), second)
	if err != nil {
		t.Logf("partial stats: %v", err)
	}

	if a == nil || b == nil {
		t.Fatal("no stats returned")
	}
	if a.DiskPath != first {
		t.Errorf("first DiskPath = %q, want %q", a.DiskPath, first)
	}
	if b.DiskPath != second {
		t.Errorf("second DiskPath = %q, want %q — the cache must be keyed on the path", b.DiskPath, second)
	}
}

func TestCollectSystemStats_CacheServesRepeatCalls(t *testing.T) {
	dir := t.TempDir()

	a, _ := CollectSystemStats(context.Background(), time.Now(), dir)
	b, _ := CollectSystemStats(context.Background(), time.Now(), dir)

	if a == nil || b == nil {
		t.Fatal("no stats returned")
	}
	if a != b {
		t.Error("repeat call within the refresh interval should return the cached snapshot")
	}
}
