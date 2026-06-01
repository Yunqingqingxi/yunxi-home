package toolreg

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// ToJSON 将任意值序列化为 JSON 字符串
func ToJSON(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("序列化失败: %w", err)
	}
	return string(data), nil
}

// GetInt 从 map 中获取整数值，不存在则返回默认值
func GetInt(args map[string]any, key string, defaultVal int) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return defaultVal
		}
		return n
	}
	return defaultVal
}

// GetBool 从 map 中获取布尔值，不存在则返回默认值
func GetBool(args map[string]any, key string, defaultVal bool) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return defaultVal
}

// MaskIfSet 如果字符串非空则返回"已设置"，否则返回"未设置"
func MaskIfSet(s string) string {
	if s != "" {
		return "已设置"
	}
	return "未设置"
}
