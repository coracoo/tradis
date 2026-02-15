package handlers

import (
	"fmt"
	"net"
	"strings"
	"sync"
)

type IPAllowlist struct {
	mu   sync.RWMutex
	raw  string
	nets []*net.IPNet
}

func NewIPAllowlist(raw string) (*IPAllowlist, []string) {
	a := &IPAllowlist{}
	notes := a.Set(raw)
	return a, notes
}

func (a *IPAllowlist) Raw() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.raw
}

func (a *IPAllowlist) AllowsIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	a.mu.RLock()
	nets := append([]*net.IPNet(nil), a.nets...)
	a.mu.RUnlock()
	for _, n := range nets {
		if n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
}

func (a *IPAllowlist) Set(raw string) []string {
	nets, normalized, notes := parseIPAllowlistRaw(raw)
	a.mu.Lock()
	a.raw = normalized
	a.nets = nets
	a.mu.Unlock()
	return notes
}

func parseIPAllowlistRaw(raw string) ([]*net.IPNet, string, []string) {
	notes := make([]string, 0)
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, "", notes
	}

	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", ",")
	parts := strings.Split(s, ",")
	out := make([]*net.IPNet, 0, len(parts))
	items := make([]string, 0, len(parts))

	for _, part := range parts {
		it := strings.TrimSpace(part)
		if it == "" {
			continue
		}
		if strings.Contains(it, "/") {
			_, n, err := net.ParseCIDR(it)
			if err != nil {
				notes = append(notes, fmt.Sprintf("忽略无效 CIDR: %s", it))
				continue
			}
			out = append(out, n)
			items = append(items, it)
			continue
		}
		ip := net.ParseIP(it)
		if ip == nil {
			notes = append(notes, fmt.Sprintf("忽略无效 IP: %s", it))
			continue
		}
		if v4 := ip.To4(); v4 != nil {
			out = append(out, &net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)})
			items = append(items, v4.String())
		} else {
			out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)})
			items = append(items, ip.String())
		}
	}

	normalized := strings.Join(items, ",")
	return out, normalized, notes
}
