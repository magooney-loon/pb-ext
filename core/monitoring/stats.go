package monitoring

import (
	"context"
	"errors"
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
	Hostname      string     `json:"hostname"`
	Platform      string     `json:"platform"`
	OS            string     `json:"os"`
	KernelVersion string     `json:"kernel_version"`
	CPUInfo       []CPUInfo  `json:"cpu_info"`
	MemoryInfo    MemoryInfo `json:"memory_info"`
	// DiskPath is the filesystem the disk figures describe.
	DiskPath  string `json:"disk_path"`
	DiskTotal uint64 `json:"disk_total"`
	DiskUsed  uint64 `json:"disk_used"`
	DiskFree  uint64 `json:"disk_free"`
	// DiskUsagePercent is Used/(Used+Free) as reported by the filesystem, which
	// accounts for reserved blocks. Do not recompute it as Used/Total.
	DiskUsagePercent float64      `json:"disk_usage_percent"`
	RuntimeStats     RuntimeStats `json:"runtime_stats"`
	ProcessStats     ProcessInfo  `json:"process_stats"`
	StartTime        time.Time    `json:"start_time"`
	UptimeSecs       int64        `json:"uptime_secs"`
	// HasTempData mirrors Temperatures.HasTempData.
	HasTempData bool `json:"has_temp_data"`
	// Temperatures holds the already-classified sensor readings, so consumers
	// never need to re-read sensors themselves.
	Temperatures       TemperatureInfo    `json:"temperatures"`
	NetworkInterfaces  []NetworkInterface `json:"network_interfaces"`
	NetworkConnections int                `json:"network_connections"`
	NetworkBytesSent   uint64             `json:"network_bytes_sent"`
	NetworkBytesRecv   uint64             `json:"network_bytes_recv"`
}

type statsCollector struct {
	mu            sync.RWMutex
	lastCollected time.Time
	cachedStats   *SystemStats
	cachedForPath string
}

var collector = &statsCollector{}

// CollectSystemStats gathers system statistics with context support.
//
// diskPath selects the filesystem reported in the disk figures; pass the
// application's data directory rather than "/". See CollectDiskInfoWithContext.
func CollectSystemStats(ctx context.Context, startTime time.Time, diskPath string) (*SystemStats, error) {
	var multiError []error

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// The cache is keyed on diskPath too, so asking about a different
	// filesystem never returns another path's figures.
	collector.mu.RLock()
	if collector.isFresh(diskPath) {
		defer collector.mu.RUnlock()
		return collector.cachedStats, nil
	}
	collector.mu.RUnlock()

	collector.mu.Lock()
	defer collector.mu.Unlock()

	if collector.isFresh(diskPath) {
		return collector.cachedStats, nil
	}

	stats := &SystemStats{
		StartTime:  startTime,
		UptimeSecs: int64(time.Since(startTime).Seconds()),
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if hostInfo, err := host.InfoWithContext(ctx); err == nil {
		stats.Hostname = hostInfo.Hostname
		stats.Platform = hostInfo.Platform
		stats.OS = hostInfo.OS
		stats.KernelVersion = hostInfo.KernelVersion
	} else {
		multiError = append(multiError, err)
	}

	cpuInfo, err := CollectCPUInfoWithContext(ctx)
	if err != nil {
		multiError = append(multiError, err)
	}
	stats.CPUInfo = cpuInfo

	memInfo, err := CollectMemoryInfoWithContext(ctx)
	if err != nil {
		multiError = append(multiError, err)
	}
	stats.MemoryInfo = memInfo

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	diskInfo, err := CollectDiskInfoWithContext(ctx, diskPath)
	if err != nil {
		multiError = append(multiError, err)
	}
	stats.DiskPath = diskInfo.Path
	stats.DiskTotal = diskInfo.Total
	stats.DiskUsed = diskInfo.Used
	stats.DiskFree = diskInfo.Free
	stats.DiskUsagePercent = diskInfo.Usage

	tempInfo, err := CollectTemperatureInfoWithContext(ctx)
	if err != nil {
		multiError = append(multiError, err)
	}
	stats.Temperatures = tempInfo
	stats.HasTempData = tempInfo.HasTempData

	procInfo, err := CollectProcessInfoWithContext(ctx)
	if err != nil {
		multiError = append(multiError, err)
	}
	stats.ProcessStats = procInfo

	stats.RuntimeStats = CollectRuntimeStats()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	netInfo, err := CollectNetworkInfoWithContext(ctx)
	if err != nil {
		multiError = append(multiError, err)
	}
	stats.NetworkInterfaces = netInfo.Interfaces
	stats.NetworkConnections = netInfo.ConnectionCount
	stats.NetworkBytesSent = netInfo.TotalBytesSent
	stats.NetworkBytesRecv = netInfo.TotalBytesRecv

	collector.cachedStats = stats
	collector.cachedForPath = diskPath
	collector.lastCollected = time.Now()

	err = nil
	if len(multiError) >= 1 {
		err = errors.Join(multiError...)
	}

	return stats, err
}

// CollectSystemStatsWithoutContext uses a background context
func CollectSystemStatsWithoutContext(startTime time.Time, diskPath string) (*SystemStats, error) {
	return CollectSystemStats(context.Background(), startTime, diskPath)
}

// isFresh reports whether the cached snapshot is recent enough and describes
// the requested filesystem. Callers must hold at least a read lock.
func (c *statsCollector) isFresh(diskPath string) bool {
	return c.cachedStats != nil &&
		c.cachedForPath == diskPath &&
		time.Since(c.lastCollected) < StatsRefreshInterval
}
