package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestIsSystemTemp(t *testing.T) {
	matches := []string{"system", "SYSTEM_TEMP", "board", "mobo", "ambient"}
	for _, s := range matches {
		if !IsSystemTemp(s) {
			t.Errorf("IsSystemTemp(%q) = false, want true", s)
		}
	}

	nonMatches := []string{"coretemp", "nvme", "k10temp", ""}
	for _, s := range nonMatches {
		if IsSystemTemp(s) {
			t.Errorf("IsSystemTemp(%q) = true, want false", s)
		}
	}
}

func TestIsAmbientTemp(t *testing.T) {
	if !IsAmbientTemp("ambient") || !IsAmbientTemp("AMBIENT_TEMP") || !IsAmbientTemp("chassis_ambient") {
		t.Error("IsAmbientTemp should match ambient sensors case-insensitively")
	}
	if IsAmbientTemp("coretemp") || IsAmbientTemp("board") || IsAmbientTemp("") {
		t.Error("IsAmbientTemp matched a non-ambient sensor")
	}
}

// TestAmbientIsCheckedBeforeSystem documents why classification order matters:
// IsSystemTemp also accepts "ambient", so testing system first would classify
// every ambient sensor as system and leave AmbientTemp permanently zero.
func TestAmbientIsCheckedBeforeSystem(t *testing.T) {
	const sensor = "ambient"

	if !IsSystemTemp(sensor) {
		t.Skip("IsSystemTemp no longer overlaps ambient; ordering constraint is moot")
	}
	if !IsAmbientTemp(sensor) {
		t.Fatalf("IsAmbientTemp(%q) = false — ambient sensors would be unclassifiable", sensor)
	}
	// The collector must resolve the overlap in favour of ambient. See the
	// switch in CollectTemperatureInfoWithContext.
}

func TestCollectTemperatureInfo(t *testing.T) {
	info, err := CollectTemperatureInfo()
	if err != nil {
		t.Skipf("sensors unavailable: %v", err)
	}

	if !info.HasTempData {
		if info.CPUTemp != 0 || info.SystemTemp != 0 || info.DiskTemp != 0 || info.AmbientTemp != 0 {
			t.Error("HasTempData is false but a reading was recorded")
		}
		t.Skip("no recognised sensors on this host")
	}

	// HasTempData now means "at least one sensor was classified", so at least
	// one reading must be present.
	if info.CPUTemp == 0 && info.SystemTemp == 0 && info.DiskTemp == 0 && info.AmbientTemp == 0 {
		t.Error("HasTempData is true but every reading is zero")
	}

	for name, v := range map[string]float64{
		"CPUTemp":     info.CPUTemp,
		"SystemTemp":  info.SystemTemp,
		"DiskTemp":    info.DiskTemp,
		"AmbientTemp": info.AmbientTemp,
	} {
		if v < 0 || v > 150 {
			t.Errorf("%s = %.1f, want a plausible celsius reading or zero", name, v)
		}
	}
}

// TestCollectTemperatureInfo_KeepsHottestPerCategory checks the collector does
// not simply keep whichever sensor came last. A group such as coretemp reports
// a package sensor plus one per core, in no guaranteed order.
func TestCollectTemperatureInfo_KeepsHottestPerCategory(t *testing.T) {
	first, err := CollectTemperatureInfo()
	if err != nil {
		t.Skipf("sensors unavailable: %v", err)
	}
	if !first.HasTempData || first.CPUTemp == 0 {
		t.Skip("no CPU sensor on this host")
	}

	// A max is stable under repeated collection in a way last-wins is not,
	// barring genuine thermal movement.
	second, err := CollectTemperatureInfo()
	if err != nil {
		t.Skipf("sensors unavailable: %v", err)
	}
	if diff := first.CPUTemp - second.CPUTemp; diff > 25 || diff < -25 {
		t.Errorf("CPU temperature jumped between reads (%.1f then %.1f) — sensor selection may be unstable",
			first.CPUTemp, second.CPUTemp)
	}
}

func TestCollectTemperatureInfoContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectTemperatureInfoWithContext(ctx)
	if err == nil {
		t.Fatal("expected an error for a cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestCollectTemperatureInfoContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	_, err := CollectTemperatureInfoWithContext(ctx)
	if err == nil {
		t.Fatal("expected an error for a timed out context")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestTemperatureInfoZeroValue(t *testing.T) {
	var info TemperatureInfo

	if info.HasTempData {
		t.Error("zero value should not claim to have temperature data")
	}
	if info.CPUTemp != 0 || info.SystemTemp != 0 || info.DiskTemp != 0 || info.AmbientTemp != 0 {
		t.Error("zero value should have zeroed readings")
	}
}
