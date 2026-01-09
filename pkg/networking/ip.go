// MIT License
//
// Copyright (c) 2026 Project AERO Contributors
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package networking provides intelligent network interface detection
// with heuristics to filter virtual adapters and prioritize LAN interfaces.
package networking

import (
	"net"
	"strings"
	"time"
)

// NetworkInterface represents a network interface with metadata.
type NetworkInterface struct {
	Name     string `json:"name"`
	IP       string `json:"ip"`
	Priority int    `json:"priority"` // Lower is better
}

// virtualAdapterPatterns contains substrings that identify virtual network adapters.
// These are filtered out to avoid confusing users with non-functional IPs.
var virtualAdapterPatterns = []string{
	"docker",
	"veth",
	"vmware",
	"vmnet",
	"virtual",
	"vbox",
	"virtualbox",
	"wsl",
	"hyper-v",
	"hyperv",
	"virbr",
	"br-",
	"vnic",
	"tailscale",
	"zt",        // ZeroTier
	"tun",       // VPN tunnels
	"tap",       // VPN tunnels
	"nordlynx",  // NordVPN
	"proton",    // ProtonVPN
	"wireguard",
	"mullvad",
}

// GetPreferredOutboundIP returns the best-guess IP for LAN file transfer
// and a list of all valid candidate interfaces.
//
// Heuristic Logic:
//  1. Filter out loopback, down, and virtual interfaces
//  2. Prioritize private LAN ranges (192.168.x.x, 10.x.x.x, 172.16-31.x.x)
//  3. Use UDP dial to 8.8.8.8:80 to determine OS routing preference
//  4. Return best guess + all candidates for UI dropdown
//
// Performance: Completes in <100ms under normal conditions.
// Conservative: Shows all valid IPs rather than hiding potentially correct ones.
func GetPreferredOutboundIP() (string, []NetworkInterface, error) {
	candidates := []NetworkInterface{}

	// Get all network interfaces
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", nil, err
	}

	for _, iface := range ifaces {
		// Skip loopback interfaces
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Skip interfaces that are down
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Skip virtual adapters
		if isVirtualAdapter(iface.Name) {
			continue
		}

		// Get addresses for this interface
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipnet.IP

			// Skip loopback IPs
			if ip.IsLoopback() {
				continue
			}

			// Skip IPv6 for now (mobile phones primarily use IPv4 for LAN)
			if ip.To4() == nil {
				continue
			}

			// Calculate priority (lower is better)
			priority := calculatePriority(ip, iface.Name)

			candidates = append(candidates, NetworkInterface{
				Name:     iface.Name,
				IP:       ip.String(),
				Priority: priority,
			})
		}
	}

	// Sort candidates by priority
	sortByPriority(candidates)

	// Try to determine the best IP using UDP dial
	bestGuess := getBestGuessViaRouting()

	// If we got a best guess, verify it's in our candidates
	if bestGuess != "" {
		for _, c := range candidates {
			if c.IP == bestGuess {
				return bestGuess, candidates, nil
			}
		}
	}

	// Fall back to highest priority candidate
	if len(candidates) > 0 {
		return candidates[0].IP, candidates, nil
	}

	return "", candidates, nil
}

// isVirtualAdapter checks if the interface name matches known virtual adapter patterns.
func isVirtualAdapter(name string) bool {
	nameLower := strings.ToLower(name)
	for _, pattern := range virtualAdapterPatterns {
		if strings.Contains(nameLower, pattern) {
			return true
		}
	}
	return false
}

// calculatePriority assigns a priority score to an IP address.
// Lower values indicate higher priority (better for LAN transfers).
//
// Priority levels:
//
//	10: 192.168.x.x - Most common home/office networks
//	20: 10.x.x.x - Corporate networks, still good
//	30: 172.16-31.x.x - Private range, less common
//	40: Link-local (169.254.x.x) - Fallback, usually means no DHCP
//	50: Other private ranges
//	100: Everything else (public IPs, etc.)
func calculatePriority(ip net.IP, ifaceName string) int {
	ip4 := ip.To4()
	if ip4 == nil {
		return 100
	}

	// Boost priority for interfaces with common LAN names
	nameBoost := 0
	nameLower := strings.ToLower(ifaceName)
	if strings.Contains(nameLower, "wi-fi") ||
		strings.Contains(nameLower, "wifi") ||
		strings.Contains(nameLower, "wlan") ||
		strings.Contains(nameLower, "wireless") {
		nameBoost = -2 // WiFi is typically the correct interface for mobile transfer
	} else if strings.Contains(nameLower, "ethernet") ||
		strings.Contains(nameLower, "eth") ||
		strings.Contains(nameLower, "en0") {
		nameBoost = -1 // Ethernet is also good
	}

	// 192.168.x.x - Most common home/office networks
	if ip4[0] == 192 && ip4[1] == 168 {
		return 10 + nameBoost
	}

	// 10.x.x.x - Corporate networks
	if ip4[0] == 10 {
		return 20 + nameBoost
	}

	// 172.16.0.0 - 172.31.255.255 - Private range
	if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
		return 30 + nameBoost
	}

	// 169.254.x.x - Link-local (usually means no DHCP)
	if ip4[0] == 169 && ip4[1] == 254 {
		return 40 + nameBoost
	}

	// Other addresses
	return 100 + nameBoost
}

// sortByPriority sorts network interfaces by priority (lower first).
func sortByPriority(interfaces []NetworkInterface) {
	// Simple bubble sort - fast enough for small lists
	n := len(interfaces)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if interfaces[j].Priority > interfaces[j+1].Priority {
				interfaces[j], interfaces[j+1] = interfaces[j+1], interfaces[j]
			}
		}
	}
}

// getBestGuessViaRouting uses the OS routing table to determine
// which interface would be used to reach the internet.
// This is done by attempting a UDP dial to 8.8.8.8:80 (no data sent).
//
// This method is highly reliable because it asks the OS directly
// which interface it would use for outbound traffic.
func getBestGuessViaRouting() string {
	// Set a short timeout to ensure <100ms performance
	conn, err := net.DialTimeout("udp4", "8.8.8.8:80", 50*time.Millisecond)
	if err != nil {
		return ""
	}
	defer conn.Close()

	// Get the local address used for this connection
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// GetAllInterfaces returns all valid network interfaces without filtering.
// Useful for debugging or when the user needs to see everything.
func GetAllInterfaces() ([]NetworkInterface, error) {
	candidates := []NetworkInterface{}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipnet.IP
			if ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			isVirtual := isVirtualAdapter(iface.Name)
			priority := calculatePriority(ip, iface.Name)
			if isVirtual {
				priority += 1000 // Push virtual adapters to the end
			}

			candidates = append(candidates, NetworkInterface{
				Name:     iface.Name,
				IP:       ip.String(),
				Priority: priority,
			})
		}
	}

	sortByPriority(candidates)
	return candidates, nil
}
