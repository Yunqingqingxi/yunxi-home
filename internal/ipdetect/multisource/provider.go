// Package multisource 提供基于多数据源并发竞速的 IP 检测器，实现 base.Detector 接口。
package multisource

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"log/slog"

	"github.com/yxd/yunxi-home/internal/ipdetect/base"
)

// Source IP 数据源配置
type Source struct {
	Name string
	URL  string
}

// Config IP 检测器配置
type Config struct {
	Sources    []Source
	DNSServers []string // 自定义 DNS 服务器，默认使用阿里云 DNS
}

// Detector 多源并发 IP 检测器
type Detector struct {
	sources    []Source
	httpClient *http.Client
	cache      *cache
	retryCount int
	retryDelay time.Duration
}

// 编译期检查接口实现
var _ base.Detector = (*Detector)(nil)

// DefaultSources 默认 IPv6 数据源
var DefaultSources = []Source{
	{Name: "api6", URL: "https://api6.ipify.org?format=json"},
	{Name: "jsoncn", URL: "https://ipv6.json.cn"},
	{Name: "identme", URL: "https://v6.ident.me"},
	{Name: "ipwcn", URL: "https://6.ipw.cn"},
}

// DefaultDNSServers 阿里云公共 DNS 服务器
var DefaultDNSServers = []string{"223.5.5.5:53", "223.6.6.6:53"}

// New 创建多源 IP 检测器
func New(cfg *Config) *Detector {
	sources := DefaultSources
	dnsServers := DefaultDNSServers

	if cfg != nil {
		if len(cfg.Sources) > 0 {
			sources = cfg.Sources
		}
		if len(cfg.DNSServers) > 0 {
			dnsServers = cfg.DNSServers
		}
	}

	// 构建自定义 DNS resolver
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 3 * time.Second}
			for _, server := range dnsServers {
				conn, err := d.DialContext(ctx, "udp", server)
				if err == nil {
					return conn, nil
				}
			}
			return d.DialContext(ctx, network, address)
		},
	}

	return &Detector{
		sources: sources,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   3 * time.Second,
					KeepAlive: 30 * time.Second,
					Resolver:  resolver,
				}).DialContext,
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		},
		cache:      newCache(5 * time.Minute),
		retryCount: 2,
		retryDelay: 1 * time.Second,
	}
}

// GetCurrentIPv6 并发获取公网 IPv6 地址
func (d *Detector) GetCurrentIPv6(ctx context.Context) (string, error) {
	slog.Info("开始检测IPv6", "数据源数", len(d.sources))
	type result struct {
		ip     string
		source string
	}

	resultCh := make(chan result, len(d.sources))
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for _, src := range d.sources {
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			ip, err := d.fetchFromSource(ctx, s.URL)
			if err != nil {
				return
			}
			if IsValidIPv6(ip) {
				select {
				case resultCh <- result{ip: ip, source: s.Name}:
				case <-ctx.Done():
				}
			}
		}(src)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	select {
	case r, ok := <-resultCh:
		if ok {
			return r.ip, nil
		}
	case <-ctx.Done():
	}

	return "", fmt.Errorf("无法获取公网 IPv6 地址，所有数据源均失败")
}

// GetCurrentIPv4 并发获取公网 IPv4 地址
func (d *Detector) GetCurrentIPv4(ctx context.Context) (string, error) {
	slog.Info("开始检测IPv4", "数据源数", len(d.sources))
	resultCh := make(chan string, len(d.sources))
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	ipv4Sources := []Source{
		{Name: "api4", URL: "https://api.ipify.org?format=json"},
		{Name: "ifconfig", URL: "https://ifconfig.me/ip"},
	}

	for _, src := range ipv4Sources {
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			ip, err := d.fetchFromSource(ctx, s.URL)
			if err != nil {
				return
			}
			if IsValidIPv4(ip) {
				select {
				case resultCh <- ip:
				case <-ctx.Done():
				}
			}
		}(src)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	select {
	case ip, ok := <-resultCh:
		if ok {
			return ip, nil
		}
	case <-ctx.Done():
	}

	return "", fmt.Errorf("无法获取公网 IPv4 地址")
}

func (d *Detector) fetchFromSource(ctx context.Context, url string) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= d.retryCount; attempt++ {
		if attempt > 0 {
			delay := d.retryDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}

		ip, err := d.doFetch(ctx, url)
		if err == nil {
			return ip, nil
		}
		lastErr = err
	}

	return "", fmt.Errorf("数据源 %s 请求失败: %w", url, lastErr)
}

func (d *Detector) doFetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain, application/json")
	req.Header.Set("User-Agent", "Yunxi-Home/3.0")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(string(body))

	if strings.HasPrefix(ip, "{") {
		ip = extractJSONIP(ip)
	}

	return ip, nil
}

func extractJSONIP(raw string) string {
	for _, key := range []string{`"ip":"`, `"ip": "`} {
		start := strings.Index(raw, key)
		if start >= 0 {
			start += len(key)
			end := strings.IndexByte(raw[start:], '"')
			if end > 0 {
				return raw[start : start+end]
			}
		}
	}
	return raw
}

// GetCachedIP 获取缓存的 IP
func (d *Detector) GetCachedIP(domain string) (string, bool) {
	return d.cache.Get(domain)
}

// SetCachedIP 设置缓存 IP
func (d *Detector) SetCachedIP(domain, ip string) {
	d.cache.Set(domain, ip)
}
