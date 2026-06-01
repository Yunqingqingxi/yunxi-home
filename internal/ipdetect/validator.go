package ipdetect

import (
	"net"
	"strings"
)

// IsValidIPv6 校验 IPv6 地址格式
func IsValidIPv6(ip string) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	// 确保是 IPv6 而非 IPv4
	return strings.Contains(ip, ":") && parsed.To16() != nil
}

// IsValidIPv4 校验 IPv4 地址格式
func IsValidIPv4(ip string) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	// 确保是 IPv4（点分十进制）
	return strings.Contains(ip, ".") && parsed.To4() != nil
}

// IsIPChanged 判断 IP 是否变化
func IsIPChanged(oldIP, newIP string) bool {
	return strings.TrimSpace(oldIP) != strings.TrimSpace(newIP)
}
