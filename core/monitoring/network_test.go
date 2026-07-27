package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v3/net"
)

// --- loopback detection ---

// TestIsLoopback_DoesNotMatchNamesContainingLo is the regression test for
// filtering interfaces with strings.Contains(name, "lo"): that also matches
// systemd's predictable names for onboard wireless ("wlo1") and onboard
// ethernet ("eno1"), silently dropping a machine's primary interface and its
// byte counters from the dashboard.
func TestIsLoopback_DoesNotMatchNamesContainingLo(t *testing.T) {
	notLoopback := []string{
		"wlo1", // onboard wireless — the case that broke
		"wlo2",
		"eno1", // onboard ethernet
		"enlo1",
		"vlon0",
		"eth0",
		"enp7s0",
		"wlan0",
		"docker0",
		"tailscale0",
	}

	for _, name := range notLoopback {
		iface := net.InterfaceStat{Name: name, Flags: []string{"up", "broadcast", "multicast"}}
		if isLoopback(iface) {
			t.Errorf("isLoopback(%q) = true, want false — real interfaces must not be filtered", name)
		}
	}
}

func TestIsLoopback_MatchesActualLoopback(t *testing.T) {
	cases := []net.InterfaceStat{
		{Name: "lo", Flags: []string{"up", "loopback"}},
		{Name: "lo0", Flags: []string{"up", "loopback"}},
		// Flag wins even when the name is unusual.
		{Name: "Loopback Pseudo-Interface 1", Flags: []string{"up", "loopback"}},
		{Name: "bridge0", Flags: []string{"loopback"}},
		// Name fallback for platforms that report no flags.
		{Name: "lo", Flags: nil},
		{Name: "LO", Flags: nil},
	}

	for _, iface := range cases {
		if !isLoopback(iface) {
			t.Errorf("isLoopback(%q, flags=%v) = false, want true", iface.Name, iface.Flags)
		}
	}
}

// --- collection ---

func TestCollectNetworkInfo(t *testing.T) {
	stats, err := CollectNetworkInfo()
	if err != nil {
		t.Skipf("network collection unavailable: %v", err)
	}

	if stats.Interfaces == nil {
		t.Error("Interfaces must be non-nil so templates can range over it")
	}

	for _, iface := range stats.Interfaces {
		if iface.Name == "" {
			t.Error("interface reported with an empty name")
		}
		if iface.Name == "lo" || iface.Name == "lo0" {
			t.Errorf("loopback interface %q must be excluded", iface.Name)
		}
	}
}

// TestCollectNetworkInfo_TotalsMatchInterfaces pins the invariant that the
// headline byte counters are the sum of the interfaces actually reported —
// they cannot be zero while interfaces are listed.
func TestCollectNetworkInfo_TotalsMatchInterfaces(t *testing.T) {
	stats, err := CollectNetworkInfo()
	if err != nil {
		t.Skipf("network collection unavailable: %v", err)
	}

	var sent, recv uint64
	for _, iface := range stats.Interfaces {
		sent += iface.BytesSent
		recv += iface.BytesRecv
	}

	if sent != stats.TotalBytesSent {
		t.Errorf("TotalBytesSent = %d, want %d (sum of interfaces)", stats.TotalBytesSent, sent)
	}
	if recv != stats.TotalBytesRecv {
		t.Errorf("TotalBytesRecv = %d, want %d (sum of interfaces)", stats.TotalBytesRecv, recv)
	}
	if len(stats.Interfaces) > 0 && stats.TotalBytesRecv == 0 && stats.TotalBytesSent == 0 {
		t.Error("interfaces are reported but both byte totals are zero")
	}
}

// TestCollectNetworkInfo_ReportsNonLoopbackWithAddresses cross-checks the
// collector against the raw interface list: every up interface that has
// addresses and is not loopback should appear.
func TestCollectNetworkInfo_ReportsNonLoopbackWithAddresses(t *testing.T) {
	raw, err := net.Interfaces()
	if err != nil {
		t.Skipf("cannot enumerate interfaces: %v", err)
	}

	want := map[string]bool{}
	for _, iface := range raw {
		if !isLoopback(iface) && len(iface.Addrs) > 0 {
			want[iface.Name] = true
		}
	}
	if len(want) == 0 {
		t.Skip("no addressed non-loopback interfaces on this host")
	}

	stats, err := CollectNetworkInfo()
	if err != nil {
		t.Skipf("network collection unavailable: %v", err)
	}

	got := map[string]bool{}
	for _, iface := range stats.Interfaces {
		got[iface.Name] = true
	}

	for name := range want {
		if !got[name] {
			t.Errorf("interface %q has addresses and is not loopback, but was not reported", name)
		}
	}
}

func TestCollectNetworkInfoContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectNetworkInfoWithContext(ctx)
	if err == nil {
		t.Fatal("expected an error for a cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestCollectNetworkInfoContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	_, err := CollectNetworkInfoWithContext(ctx)
	if err == nil {
		t.Fatal("expected an error for a timed out context")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestCollectNetworkInfoConcurrent(t *testing.T) {
	const goroutines = 8
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			_, err := CollectNetworkInfo()
			errs <- err
		}()
	}
	for i := 0; i < goroutines; i++ {
		if err := <-errs; err != nil {
			t.Logf("concurrent collection %d: %v", i, err)
		}
	}
}

func TestNetworkStatsZeroValue(t *testing.T) {
	var stats NetworkStats

	if stats.ConnectionCount != 0 || stats.TotalBytesSent != 0 || stats.TotalBytesRecv != 0 {
		t.Error("zero value should have zeroed counters")
	}
	if len(stats.Interfaces) != 0 {
		t.Error("zero value should have no interfaces")
	}
}
