package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/host"
)

const (
	// StatsRefreshInterval is the minimum time between stats refreshes
	StatsRefreshInterval = 2 * time.Second
)

// SystemStats holds various system metrics
type SystemStats struct {
	Hostname           string             `json:"hostname"`
	Platform           string             `json:"platform"`
	OS                 string             `json:"os"`
	KernelVersion      string             `json:"kernel_version"`
	CPUInfo            []CPUInfo          `json:"cpu_info"`
	MemoryInfo         MemoryInfo         `json:"memory_info"`
	DiskTotal          uint64             `json:"disk_total"`
	DiskUsed           uint64             `json:"disk_used"`
	DiskFree           uint64             `json:"disk_free"`
	RuntimeStats       RuntimeStats       `json:"runtime_stats"`
	ProcessStats       ProcessInfo        `json:"process_stats"`
	StartTime          time.Time          `json:"start_time"`
	UptimeSecs         int64              `json:"uptime_secs"`
	HasTempData        bool               `json:"has_temp_data"`
	NetworkInterfaces  []NetworkInterface `json:"network_interfaces"`
	NetworkConnections int                `json:"network_connections"`
	NetworkBytesSent   uint64             `json:"network_bytes_sent"`
	NetworkBytesRecv   uint64             `json:"network_bytes_recv"`
}

type statsCollector struct {
	mu            sync.RWMutex
	lastCollected time.Time
	cachedStats   *SystemStats
}

var collector = &statsCollector{}

// CollectSystemStats gathers current system statistics with context support
func CollectSystemStats(ctx context.Context, startTime time.Time) (*SystemStats, error) {
	// Check if context is done
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// Continue with collection
	}

	collector.mu.RLock()
	if time.Since(collector.lastCollected) < StatsRefreshInterval && collector.cachedStats != nil {
		defer collector.mu.RUnlock()
		return collector.cachedStats, nil
	}
	collector.mu.RUnlock()

	collector.mu.Lock()
	defer collector.mu.Unlock()

	if time.Since(collector.lastCollected) < StatsRefreshInterval && collector.cachedStats != nil {
		return collector.cachedStats, nil
	}

	stats := &SystemStats{
		StartTime:  startTime,
		UptimeSecs: int64(time.Since(startTime).Seconds()),
	}

	// Check context after potential wait time
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// Continue with collection
	}

	// Get host info
	if hostInfo, err := host.InfoWithContext(ctx); err == nil {
		stats.Hostname = hostInfo.Hostname
		stats.Platform = hostInfo.Platform
		stats.OS = hostInfo.OS
		stats.KernelVersion = hostInfo.KernelVersion
	}

	// Collect CPU info
	cpuInfo, err := CollectCPUInfoWithContext(ctx)
	if err == nil {
		stats.CPUInfo = cpuInfo
	}

	// Collect memory info
	memInfo, err := CollectMemoryInfoWithContext(ctx)
	if err == nil {
		stats.MemoryInfo = memInfo
	}

	// Check context again
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// Continue with collection
	}

	// Collect disk info
	diskInfo, err := CollectDiskInfoWithContext(ctx)
	if err == nil {
		stats.DiskTotal = diskInfo.Total
		stats.DiskUsed = diskInfo.Used
		stats.DiskFree = diskInfo.Free
	}

	// Collect temperature info
	tempInfo, err := CollectTemperatureInfoWithContext(ctx)
	if err == nil {
		stats.HasTempData = tempInfo.HasTempData
	}

	// Collect process info
	procInfo, err := CollectProcessInfoWithContext(ctx)
	if err == nil {
		stats.ProcessStats = procInfo
	}

	// Collect runtime stats
	stats.RuntimeStats = CollectRuntimeStats()

	// Final context check before network collection
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// Continue with collection
	}

	// Collect network info
	netInfo, err := CollectNetworkInfoWithContext(ctx)
	if err == nil {
		stats.NetworkInterfaces = netInfo.Interfaces
		stats.NetworkConnections = netInfo.ConnectionCount
		stats.NetworkBytesSent = netInfo.TotalBytesSent
		stats.NetworkBytesRecv = netInfo.TotalBytesRecv
	}

	collector.cachedStats = stats
	collector.lastCollected = time.Now()

	return stats, nil
}

// CollectSystemStatsWithoutContext is a backward compatibility wrapper
// that uses a background context
func CollectSystemStatsWithoutContext(startTime time.Time) (*SystemStats, error) {
	return CollectSystemStats(context.Background(), startTime)
}
