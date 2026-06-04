package provider

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// HTTPClientConfig configures the shared HTTP transport for AI provider API calls.
type HTTPClientConfig struct {
	// ConnectTimeout is the maximum time to establish a TCP connection.
	ConnectTimeout time.Duration
	// TLSHandshakeTimeout is the maximum time for TLS handshake.
	TLSHandshakeTimeout time.Duration
	// RequestTimeout is the overall HTTP request timeout (set on http.Client).
	RequestTimeout time.Duration
	// KeepAlive is the TCP keep-alive period for idle connections.
	KeepAlive time.Duration
	// IdleConnTimeout is the maximum time an idle connection stays in the pool.
	IdleConnTimeout time.Duration
	// MaxIdleConns controls the maximum number of idle connections across all hosts.
	MaxIdleConns int
	// MaxIdleConnsPerHost controls the maximum number of idle connections per host.
	MaxIdleConnsPerHost int
	// ForceIPv4 forces IPv4-only connections (needed for hosts without AAAA records).
	ForceIPv4 bool
	// EnableHTTP2 enables HTTP/2 support.
	EnableHTTP2 bool
}

// DefaultHTTPClientConfig returns sensible defaults for AI API communication.
// These defaults handle weak networks, transient failures, and connection reuse.
func DefaultHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		ConnectTimeout:      30 * time.Second,
		TLSHandshakeTimeout: 15 * time.Second,
		RequestTimeout:      10 * time.Minute,
		KeepAlive:           30 * time.Second,
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		ForceIPv4:           false,
		EnableHTTP2:         true,
	}
}

// NewHTTPClient creates an optimized http.Client for AI provider API calls.
// Features:
//   - Connection pooling with configurable idle limits
//   - TCP keep-alive to detect dead connections
//   - Optional IPv4-only mode for hosts without IPv6
//   - HTTP/2 support for multiplexed streaming
//   - Sensible timeouts at every layer
func NewHTTPClient(cfg HTTPClientConfig) *http.Client {
	dialer := &net.Dialer{
		Timeout:   cfg.ConnectTimeout,
		KeepAlive: cfg.KeepAlive,
	}

	// Build the dial context function
	dialContext := dialer.DialContext
	if cfg.ForceIPv4 {
		dialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp4", addr)
		}
	}

	transport := &http.Transport{
		DialContext:           dialContext,
		ForceAttemptHTTP2:     cfg.EnableHTTP2,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		// ResponseHeaderTimeout limits time waiting for response headers
		ResponseHeaderTimeout: 30 * time.Second,
		// DisableKeepAlives=false means connections are reused (pooled)
		DisableKeepAlives: false,
	}

	// Use modern TLS config
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   cfg.RequestTimeout,
	}
}

// NewIPv4HTTPClient is a convenience wrapper that forces IPv4-only connections.
// Use this for providers that don't have AAAA DNS records (e.g., DeepSeek).
func NewIPv4HTTPClient(timeout time.Duration) *http.Client {
	cfg := DefaultHTTPClientConfig()
	cfg.ForceIPv4 = true
	cfg.RequestTimeout = timeout
	return NewHTTPClient(cfg)
}

// NewStandardHTTPClient creates a standard HTTP client with dual-stack support.
func NewStandardHTTPClient(timeout time.Duration) *http.Client {
	cfg := DefaultHTTPClientConfig()
	cfg.RequestTimeout = timeout
	return NewHTTPClient(cfg)
}
