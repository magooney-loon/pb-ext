package monitoring

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/v3/net"
)

// NetworkInterface holds information about a network interface
type NetworkInterface struct {
	Name        string `json:"name"`
	IPAddress   string `json:"ip_address"`
	BytesSent   uint64 `json:"bytes_sent"`
	BytesRecv   uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
}

// NetworkStats holds aggregated network statistics
type NetworkStats struct {
	Interfaces      []NetworkInterface `json:"interfaces"`
	ConnectionCount int                `json:"connection_count"`
	TotalBytesSent  uint64             `json:"total_bytes_sent"`
	TotalBytesRecv  uint64             `json:"total_bytes_recv"`
}

// CollectNetworkInfoWithContext gathers network statistics with context
func CollectNetworkInfoWithContext(ctx context.Context) (NetworkStats, error) {
	select {
	case <-ctx.Done():
		return NetworkStats{}, ctx.Err()
	default:
	}

	result := NetworkStats{
		Interfaces: make([]NetworkInterface, 0),
	}

	interfaces, err := net.InterfacesWithContext(ctx)
	if err != nil {
		return result, err
	}

	select {
	case <-ctx.Done():
		return result, ctx.Err()
	default:
	}

	ioStats, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		return result, err
	}

	select {
	case <-ctx.Done():
		return result, ctx.Err()
	default:
	}

	connections, err := net.ConnectionsWithContext(ctx, "all")
	if err == nil {
		result.ConnectionCount = len(connections)
	}

	for _, iface := range interfaces {
		// Skip loopback and interfaces without addresses
		if strings.Contains(strings.ToLower(iface.Name), "lo") || len(iface.Addrs) == 0 {
			continue
		}

		for _, io := range ioStats {
			if io.Name == iface.Name {
				netIface := NetworkInterface{
					Name:        iface.Name,
					BytesSent:   io.BytesSent,
					BytesRecv:   io.BytesRecv,
					PacketsSent: io.PacketsSent,
					PacketsRecv: io.PacketsRecv,
				}

				// Get first non-local IPv4 address
				for _, addr := range iface.Addrs {
					if !strings.HasPrefix(addr.Addr, "127.") && !strings.Contains(addr.Addr, ":") {
						netIface.IPAddress = addr.Addr
						break
					}
				}

				result.Interfaces = append(result.Interfaces, netIface)
				result.TotalBytesSent += io.BytesSent
				result.TotalBytesRecv += io.BytesRecv
				break
			}
		}
	}

	return result, nil
}

// CollectNetworkInfo uses background context
func CollectNetworkInfo() (NetworkStats, error) {
	return CollectNetworkInfoWithContext(context.Background())
}
