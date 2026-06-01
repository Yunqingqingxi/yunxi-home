package alidns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/yxd/yunxi-home/internal/dns/base"
)

// Client 阿里云 DNS 客户端，实现 base.Provider 接口。
type Client struct {
	accessKeyID     string
	accessKeySecret string
	endpoint        string
	httpClient      *http.Client
}

// 编译期检查接口实现
var _ base.Provider = (*Client)(nil)

// NewClient 创建阿里云 DNS 客户端
func NewClient(accessKeyID, accessKeySecret, endpoint string, dnsServers []string) *Client {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	if len(dnsServers) > 0 {
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
		httpClient.Transport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
				Resolver:  resolver,
			}).DialContext,
		}
	}
	return &Client{
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		endpoint:        endpoint,
		httpClient:      httpClient,
	}
}

// doRequest 执行阿里云 API 请求
func (c *Client) doRequest(ctx context.Context, action string, bizParams map[string]string) (map[string]interface{}, error) {
	slog.Info("阿里云DNS请求", "操作", action)
	baseParams := buildParams(c.accessKeyID, action)
	baseParams["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	allParams := MergeParams(baseParams, bizParams)

	allParams["Signature"] = Sign(allParams, c.accessKeySecret)
	urlStr := BuildSignedURL(c.endpoint, allParams, c.accessKeySecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		slog.Error("DNS请求创建失败", "操作", action, "错误", err)
		slog.Error("DNS请求创建失败", "错误", err)
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Yunxi-Home/3.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应 JSON 失败: %w, body: %s", err, string(body))
	}

	if code, ok := result["Code"]; ok {
		apiErr := &APIError{Code: fmt.Sprintf("%v", code)}
		if msg, ok := result["Message"]; ok {
			apiErr.Message = fmt.Sprintf("%v", msg)
		}
		if reqID, ok := result["RequestId"]; ok {
			apiErr.RequestID = fmt.Sprintf("%v", reqID)
		}
		return nil, apiErr
	}

	return result, nil
}
