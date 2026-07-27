package monitoring

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestCollectProcessInfo(t *testing.T) {
	info, err := CollectProcessInfo()
	if err != nil {
		// Partial data is expected where /proc is restricted; the PID must
		// still be right.
		t.Logf("partial process info: %v", err)
	}

	if info.PID != int32(os.Getpid()) {
		t.Errorf("PID = %d, want %d (the current process)", info.PID, os.Getpid())
	}
}

func TestCollectProcessInfo_MemoryIsPlausible(t *testing.T) {
	info, err := CollectProcessInfo()
	if err != nil {
		t.Logf("partial process info: %v", err)
	}

	if info.RSS == 0 {
		t.Skip("RSS unavailable on this host")
	}

	// Resident memory cannot exceed the address space reservation.
	if info.VMS > 0 && info.RSS > info.VMS {
		t.Errorf("RSS (%d) exceeds VMS (%d)", info.RSS, info.VMS)
	}

	sys, err := CollectMemoryInfo()
	if err == nil && sys.Total > 0 {
		if info.RSS > sys.Total {
			t.Errorf("RSS (%d) exceeds total system memory (%d)", info.RSS, sys.Total)
		}

		// MemoryPercent should agree with RSS/Total.
		want := float64(info.RSS) / float64(sys.Total) * 100
		if diff := info.MemoryPercent - want; diff > 1 || diff < -1 {
			t.Errorf("MemoryPercent = %.3f, want ~%.3f (RSS/Total)", info.MemoryPercent, want)
		}
	}
}

func TestCollectProcessInfo_ThreadsAndFiles(t *testing.T) {
	info, err := CollectProcessInfo()
	if err != nil {
		t.Logf("partial process info: %v", err)
	}

	if info.NumThreads < 0 {
		t.Errorf("NumThreads = %d, want >= 0", info.NumThreads)
	}
	if info.OpenFiles < 0 {
		t.Errorf("OpenFiles = %d, want >= 0", info.OpenFiles)
	}
	// A running Go program always has more than one OS thread.
	if info.NumThreads == 1 {
		t.Logf("NumThreads = 1, unusual for a Go process but not invalid")
	}
}

// The raw descriptor count is not an alertable figure on its own — the soft
// limit is 1024 on some hosts and 1048576 on others, so the same count means
// "about to fail" or "idle" depending on the machine. The ratio is what the
// saturation rule watches, and a limit of 0 must read as "unknown" rather than
// producing a ratio against nothing.
func TestCollectProcessInfo_OpenFileRatio(t *testing.T) {
	info, err := CollectProcessInfo()
	if err != nil {
		t.Logf("partial process info: %v", err)
	}

	if info.OpenFilesLimit == 0 {
		// Windows, or a failed lookup. The ratio must stay at zero rather than
		// implying a measurement.
		if info.OpenFilesPercent != 0 {
			t.Errorf("OpenFilesPercent = %v with an unknown limit, want 0", info.OpenFilesPercent)
		}
		t.Skip("RLIMIT_NOFILE is unavailable on this platform")
	}

	if info.OpenFiles > 0 && info.OpenFilesPercent <= 0 {
		t.Errorf("OpenFilesPercent = %v with %d files open against a limit of %d, want it computed",
			info.OpenFilesPercent, info.OpenFiles, info.OpenFilesLimit)
	}
	if info.OpenFilesPercent < 0 || info.OpenFilesPercent > 100 {
		t.Errorf("OpenFilesPercent = %v, want 0..100", info.OpenFilesPercent)
	}

	// The test binary holds a handful of descriptors against a limit that is
	// normally in the thousands, so it must sit nowhere near the ceiling.
	if info.OpenFilesPercent > 90 {
		t.Errorf("OpenFilesPercent = %v for a test binary (%d/%d); the ratio looks inverted",
			info.OpenFilesPercent, info.OpenFiles, info.OpenFilesLimit)
	}
}

func TestOpenFilesLimit_ReportsAPlausibleCeiling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no RLIMIT_NOFILE on Windows")
	}

	got := openFilesLimit()
	if got == 0 {
		// Legitimate on a host with RLIMIT_NOFILE set to unlimited: there is no
		// ceiling to saturate, so there is no ratio, and the rule skips.
		t.Skip("this host reports no descriptor ceiling")
	}
	if got >= maxPlausibleFDLimit {
		t.Errorf("openFilesLimit() = %d, want an implausible ceiling normalized to 0", got)
	}
}

// RLIM_INFINITY is ^uint64(0) on Linux and max-int64 on the BSDs, and FreeBSD's
// Rlimit.Cur is signed so a negative arrives here as a huge unsigned value.
// All of those mean "no ceiling to measure against", which has to collapse to
// the same 0 that means "unknown" — otherwise it lands in the dashboard as
// 18446744073709551615 and in the metrics as a ratio against nothing.
func TestNormalizeFDLimit(t *testing.T) {
	cases := []struct {
		name string
		in   uint64
		want uint64
	}{
		{"typical", 1024, 1024},
		{"raised", 1048576, 1048576},
		{"large but real", 1 << 30, 1 << 30},
		{"linux RLIM_INFINITY", ^uint64(0), 0},
		{"bsd RLIM_INFINITY", 1<<63 - 1, 0},
		{"freebsd negative one", uint64(0xFFFFFFFFFFFFFFFF), 0},
		{"zero", 0, 0},
		{"at the sanity cap", maxPlausibleFDLimit, 0},
		{"just under the cap", maxPlausibleFDLimit - 1, maxPlausibleFDLimit - 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeFDLimit(tc.in); got != tc.want {
				t.Errorf("normalizeFDLimit(%d) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// An absent ceiling must never produce a percentage — the saturation rule reads
// a limit of 0 as "skip", and a ratio of 0% would read as "healthy" instead.
func TestCollectProcessInfo_NoRatioWithoutACeiling(t *testing.T) {
	info, err := CollectProcessInfo()
	if err != nil {
		t.Logf("partial process info: %v", err)
	}

	if info.OpenFilesLimit == 0 && info.OpenFilesPercent != 0 {
		t.Errorf("OpenFilesPercent = %v with no ceiling, want 0", info.OpenFilesPercent)
	}
	if info.OpenFilesLimit >= maxPlausibleFDLimit {
		t.Errorf("OpenFilesLimit = %d, want it normalized", info.OpenFilesLimit)
	}
}

func TestCollectProcessInfo_CPUPercentInRange(t *testing.T) {
	info, err := CollectProcessInfo()
	if err != nil {
		t.Logf("partial process info: %v", err)
	}

	if info.CPUPercent < 0 {
		t.Errorf("CPUPercent = %.2f, want >= 0", info.CPUPercent)
	}
	// gopsutil reports process CPU as a share of one core's time averaged over
	// the process lifetime, so it can exceed 100 on a busy multi-threaded
	// process but should not be wild.
	if info.CPUPercent > float64(100*runtime.NumCPU()) {
		t.Errorf("CPUPercent = %.2f exceeds what %d cores can produce", info.CPUPercent, runtime.NumCPU())
	}
}

func TestCollectProcessInfoContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectProcessInfoWithContext(ctx)
	if err == nil {
		t.Fatal("expected an error for a cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestCollectProcessInfoContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	_, err := CollectProcessInfoWithContext(ctx)
	if err == nil {
		t.Fatal("expected an error for a timed out context")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestCollectProcessInfoConcurrent(t *testing.T) {
	const goroutines = 8
	pids := make(chan int32, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			info, _ := CollectProcessInfo()
			pids <- info.PID
		}()
	}

	want := int32(os.Getpid())
	for i := 0; i < goroutines; i++ {
		if got := <-pids; got != want {
			t.Errorf("concurrent collection returned PID %d, want %d", got, want)
		}
	}
}

func TestProcessInfoZeroValue(t *testing.T) {
	var info ProcessInfo

	if info.PID != 0 || info.RSS != 0 || info.VMS != 0 {
		t.Error("zero value should have zeroed fields")
	}
	if info.CPUPercent != 0 || info.MemoryPercent != 0 {
		t.Error("zero value should have zeroed percentages")
	}
}
