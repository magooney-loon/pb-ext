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
