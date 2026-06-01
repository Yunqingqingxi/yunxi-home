package alidns

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Yunqingqingxi/yunxi-home/internal/dns/base"
)

// ── 记录查询 ────────────────────────────────────────────

// extractRecords 从 API 响应中提取记录列表
func extractRecords(result map[string]interface{}) ([]DomainRecord, error) {
	recordsData, ok := result["DomainRecords"]
	if !ok {
		return nil, nil
	}
	recordsMap, ok := recordsData.(map[string]interface{})
	if !ok {
		return nil, nil
	}
	recordList, ok := recordsMap["Record"]
	if !ok {
		return nil, nil
	}

	data, err := json.Marshal(recordList)
	if err != nil {
		return nil, fmt.Errorf("序列化记录列表失败: %w", err)
	}

	var records []DomainRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("反序列化记录列表失败: %w", err)
	}
	return records, nil
}

// extractDomains 从 API 响应中提取域名列表
func extractDomains(result map[string]interface{}) ([]DomainInfo, error) {
	domainsData, ok := result["Domains"]
	if !ok {
		return nil, nil
	}
	domainsMap, ok := domainsData.(map[string]interface{})
	if !ok {
		return nil, nil
	}
	domainList, ok := domainsMap["Domain"]
	if !ok {
		return nil, nil
	}

	data, err := json.Marshal(domainList)
	if err != nil {
		return nil, fmt.Errorf("序列化域名列表失败: %w", err)
	}

	var domains []DomainInfo
	if err := json.Unmarshal(data, &domains); err != nil {
		return nil, fmt.Errorf("反序列化域名列表失败: %w", err)
	}
	return domains, nil
}

// getIntFromResult 从 API 响应中提取整数
func getIntFromResult(result map[string]interface{}, key string) int {
	if v, ok := result[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		}
	}
	return 0
}

// ── base.Provider 实现 ──────────────────────────────────

// FindRecord 查找匹配的域名记录
func (c *Client) FindRecord(ctx context.Context, domainName, rr, recordType string) (*base.Record, error) {
	params := map[string]string{
		"DomainName":  domainName,
		"PageSize":    "100",
		"RRKeyWord":   rr,
		"TypeKeyWord": recordType,
		"SearchMode":  "ADVANCED",
	}

	result, err := c.doRequest(ctx, "DescribeDomainRecords", params)
	if err != nil {
		return nil, err
	}

	records, err := extractRecords(result)
	if err != nil {
		return nil, err
	}

	for _, rec := range records {
		if rec.RR == rr && rec.Type == recordType {
			return toBaseRecord(&rec), nil
		}
	}

	return nil, nil
}

// UpdateRecord 更新域名记录
func (c *Client) UpdateRecord(ctx context.Context, recordID, rr, recordType, value string, ttl int) error {
	params := map[string]string{
		"RecordId": recordID,
		"RR":       rr,
		"Type":     recordType,
		"Value":    value,
	}
	if ttl > 0 {
		params["TTL"] = strconv.Itoa(ttl)
	}

	_, err := c.doRequest(ctx, "UpdateDomainRecord", params)
	return err
}

// AddRecord 添加域名记录，返回 RecordID
func (c *Client) AddRecord(ctx context.Context, domainName, rr, recordType, value string, ttl int) (string, error) {
	params := map[string]string{
		"DomainName": domainName,
		"RR":         rr,
		"Type":       recordType,
		"Value":      value,
	}
	if ttl > 0 {
		params["TTL"] = strconv.Itoa(ttl)
	}

	result, err := c.doRequest(ctx, "AddDomainRecord", params)
	if err != nil {
		return "", err
	}

	if recordID, ok := result["RecordId"]; ok {
		return fmt.Sprintf("%v", recordID), nil
	}

	return "", fmt.Errorf("添加记录成功但未返回 RecordId")
}

// SetRecordStatus 设置记录状态（Enable/Disable）
func (c *Client) SetRecordStatus(ctx context.Context, recordID, status string) error {
	params := map[string]string{
		"RecordId": recordID,
		"Status":   status,
	}
	_, err := c.doRequest(ctx, "SetDomainRecordStatus", params)
	return err
}

// DeleteRecord 删除阿里云上的解析记录
func (c *Client) DeleteRecord(ctx context.Context, recordID string) error {
	params := map[string]string{"RecordId": recordID}
	_, err := c.doRequest(ctx, "DeleteDomainRecord", params)
	return err
}

// ListDomains 获取阿里云账号下的域名列表
func (c *Client) ListDomains(ctx context.Context, keyword string, page, size int) (*base.ListResult, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	params := map[string]string{
		"PageNumber": strconv.Itoa(page),
		"PageSize":   strconv.Itoa(size),
	}
	if keyword != "" {
		params["KeyWord"] = keyword
	}

	result, err := c.doRequest(ctx, "DescribeDomains", params)
	if err != nil {
		return nil, err
	}

	domains, err := extractDomains(result)
	if err != nil {
		return nil, err
	}

	return &base.ListResult{
		TotalCount: getIntFromResult(result, "TotalCount"),
		Domains:    toBaseDomains(domains),
	}, nil
}

// ListAllRecords 查询指定域名下的所有解析记录
func (c *Client) ListAllRecords(ctx context.Context, domainName string, page, size int) ([]base.Record, int, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 500 {
		size = 50
	}

	params := map[string]string{
		"DomainName": domainName,
		"PageSize":   strconv.Itoa(size),
		"PageNumber": strconv.Itoa(page),
	}

	result, err := c.doRequest(ctx, "DescribeDomainRecords", params)
	if err != nil {
		return nil, 0, err
	}

	records, err := extractRecords(result)
	if err != nil {
		return nil, 0, err
	}

	return toBaseRecords(records), getIntFromResult(result, "TotalCount"), nil
}
