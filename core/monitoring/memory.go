package monitoring

import (
	"context"

	"github.com/shirou/gopsutil/v3/mem"
)

// MemoryInfo holds memory information.
//
// Free and Available are different things and the distinction matters on Linux:
// Free is MemFree — pages that are completely untouched — while Available is
// MemAvailable, the memory a new allocation can actually get, including the
// page cache the kernel would reclaim. On a warm machine Free is near zero and
// Available is most of RAM, so Available is the number to show a human. Note
// that Used + Free does not equal Total; the remainder is Cached.
type MemoryInfo struct {
	Total uint64 `json:"total"`
	// Used excludes buffers and cache, so it reflects memory genuinely in use
	// by processes.
	Used uint64 `json:"used"`
	// Free is untouched memory only. Prefer Available for display.
	Free uint64 `json:"free"`
	// Available is memory obtainable without swapping, cache included.
	Available uint64 `json:"available"`
	// Cached is page cache plus reclaimable slab — accounted as neither Used
	// nor Free, which is why the three don't sum to Total.
	Cached      uint64  `json:"cached"`
	UsedPercent float64 `json:"used_percent"`
	SwapTotal   uint64  `json:"swap_total"`
	SwapUsed    uint64  `json:"swap_used"`
	SwapPercent float64 `json:"swap_percent"`
}

// CollectMemoryInfoWithContext gathers memory info with context support
func CollectMemoryInfoWithContext(ctx context.Context) (MemoryInfo, error) {
	select {
	case <-ctx.Done():
		return MemoryInfo{}, ctx.Err()
	default:
	}

	result := MemoryInfo{}

	if memInfo, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		result.Total = memInfo.Total
		result.Used = memInfo.Used
		result.Free = memInfo.Free
		result.Available = memInfo.Available
		result.Cached = memInfo.Cached
		result.UsedPercent = memInfo.UsedPercent

		// Platforms without a native "available" metric can leave it at zero;
		// fall back to the conservative free+cache estimate rather than
		// reporting that no memory is obtainable.
		if result.Available == 0 && result.Total > 0 {
			result.Available = result.Free + result.Cached
		}
	} else {
		return result, err
	}

	select {
	case <-ctx.Done():
		return result, ctx.Err()
	default:
	}

	if swapInfo, err := mem.SwapMemoryWithContext(ctx); err == nil {
		result.SwapTotal = swapInfo.Total
		result.SwapUsed = swapInfo.Used
		result.SwapPercent = swapInfo.UsedPercent
	}

	return result, nil
}

// CollectMemoryInfo uses background context
func CollectMemoryInfo() (MemoryInfo, error) {
	return CollectMemoryInfoWithContext(context.Background())
}
