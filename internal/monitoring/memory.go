package monitoring

import (
	"context"

	"github.com/shirou/gopsutil/v3/mem"
)

// MemoryInfo holds memory information
type MemoryInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
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
		result.UsedPercent = memInfo.UsedPercent
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
