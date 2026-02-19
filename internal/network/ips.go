package network

import (
	"net"
	"sort"
)

// GetPublicIPs returns all non-loopback IP addresses that should be accessible
func GetPublicIPs() []string {
	var ips []string
	interfaces, _ := net.Interfaces()

	for _, iface := range interfaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Skip loopback and non-IPv4
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			ips = append(ips, ip.String())
		}
	}

	// Sort and deduplicate
	sort.Strings(ips)
	unique := make([]string, 0, len(ips))
	seen := make(map[string]bool)
	for _, ip := range ips {
		if !seen[ip] {
			unique = append(unique, ip)
			seen[ip] = true
		}
	}

	return unique
}

// GetDefaultIP returns the most likely IP address for external access
// Prefers 192.168.x.x over other ranges
func GetDefaultIP() string {
	ips := GetPublicIPs()
	if len(ips) == 0 {
		return "127.0.0.1"
	}

	for _, ip := range ips {
		// Prefer 192.168.x.x range (common for LAN access)
		if len(ip) >= 8 && ip[:7] == "192.168" {
			return ip
		}
	}

	return ips[0]
}
