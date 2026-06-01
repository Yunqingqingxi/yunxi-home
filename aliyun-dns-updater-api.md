# 阿里云 DNS (Alidns) 域名管理 API 参考

> API 版本: 2015-01-09 | 产品: Alidns | 协议: RPC (HTTP GET/POST)

---

## 1. 概述

阿里云云解析 DNS 提供 RESTful RPC API，用于管理域名和解析记录。所有 API 请求通过 HTTPS 发送到 Endpoint，并使用 HMAC-SHA1 签名进行身份验证。

### Endpoint

```
https://alidns.aliyuncs.com/
```

### 支持的记录类型

| 类型 | 说明 |
|------|------|
| A | IPv4 地址记录 |
| AAAA | IPv6 地址记录 |
| CNAME | 别名记录 |
| MX | 邮件交换记录 |
| TXT | 文本记录 |
| NS | 名称服务器记录 |
| SRV | 服务定位记录 |
| CAA | 证书颁发机构授权记录 |
| PTR | 反向解析记录 (仅内网) |

---

## 2. 认证机制

### 2.1 AccessKey

所有 API 请求必须使用阿里云 AccessKey 进行签名认证。在 [RAM 控制台](https://ram.console.aliyun.com) 创建 AccessKey，获取:
- **AccessKeyId** — 标识用户
- **AccessKeySecret** — 用于签名

### 2.2 公共请求参数

每个请求必须包含以下公共参数:

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `AccessKeyId` | string | 是 | 访问密钥 ID |
| `Signature` | string | 是 | 请求签名 |
| `SignatureMethod` | string | 是 | 签名方式，值: `HMAC-SHA1` |
| `SignatureVersion` | string | 是 | 签名版本，值: `1.0` |
| `SignatureNonce` | string | 是 | 唯一随机数 (UUID)，防重放 |
| `Timestamp` | string | 是 | UTC 时间戳，ISO8601: `2006-01-02T15:04:05Z` |
| `Format` | string | 否 | 返回格式: `JSON` / `XML`，建议 `JSON` |
| `Version` | string | 是 | API 版本，值: `2015-01-09` |

### 2.3 签名算法

```
StringToSign = HTTPMethod + "&" +
               percentEncode("/") + "&" +
               percentEncode(CanonicalizedQueryString)

Signature = Base64(HMAC-SHA1(AccessKeySecret + "&", StringToSign))
```

**构造 CanonicalizedQueryString 的步骤:**

1. 将所有请求参数 (除 `Signature` 外) 按参数名字典序排列
2. 对参数名和参数值进行 Percent-Encoding (URL 编码):
   - `A-Z a-z 0-9 - _ . ~` 不编码
   - 空格编码为 `%20` (不是 `+`)
   - 其他字符编码为 `%XY` (UTF-8 字节的十六进制大写)
3. 用 `=` 连接编码后的参数名和值，用 `&` 连接各参数对

### 2.4 Go 语言签名实现

```go
import (
    "crypto/hmac"
    "crypto/sha1"
    "encoding/base64"
    "net/url"
    "sort"
    "strings"
)

func percentEncode(s string) string {
    encoded := url.QueryEscape(s)
    encoded = strings.ReplaceAll(encoded, "+", "%20")
    encoded = strings.ReplaceAll(encoded, "*", "%2A")
    encoded = strings.ReplaceAll(encoded, "%7E", "~")
    return encoded
}

func Sign(method string, params map[string]string, secret string) string {
    // 1. 按 key 排序
    keys := make([]string, 0, len(params))
    for k := range params {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    // 2. 构造规范化查询字符串
    var parts []string
    for _, k := range keys {
        parts = append(parts, percentEncode(k)+"="+percentEncode(params[k]))
    }
    canonicalized := strings.Join(parts, "&")

    // 3. 构造待签名字符串
    stringToSign := method + "&" + percentEncode("/") + "&" + percentEncode(canonicalized)

    // 4. HMAC-SHA1 + Base64
    mac := hmac.New(sha1.New, []byte(secret+"&"))
    mac.Write([]byte(stringToSign))
    return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
```

---

## 3. 域名管理 API

### 3.1 DescribeDomains — 获取域名列表

查询账号下的所有域名。

**Action:** `DescribeDomains`

#### 请求参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `KeyWord` | string | 否 | 域名关键字，前后模糊匹配，不区分大小写 |
| `GroupId` | string | 否 | 域名分组 ID，传 `defaultGroup` 查默认分组，不传查全部 |
| `PageNumber` | integer | 否 | 页码，从 1 开始，默认 1 |
| `PageSize` | integer | 否 | 每页数量，默认 20，最大 100 |
| `Lang` | string | 否 | 语言: `zh` / `en`，默认 `zh` |

#### 响应字段

| 字段 | 类型 | 描述 |
|------|------|------|
| `RequestId` | string | 请求唯一标识 |
| `TotalCount` | integer | 域名总数 |
| `PageNumber` | integer | 当前页码 |
| `PageSize` | integer | 当前每页数量 |
| `Domains` | array | 域名列表 |

**Domains 数组元素:**

| 字段 | 类型 | 描述 |
|------|------|------|
| `DomainId` | string | 域名 ID |
| `DomainName` | string | 域名名称 |
| `GroupId` | string | 域名分组 ID |
| `GroupName` | string | 域名分组名称 |
| `InstanceId` | string | 云解析产品实例 ID |
| `VersionCode` | string | 云解析版本: `free` / `v1` / `v2` |
| `RecordCount` | integer | 解析记录总数 |
| `CreateTime` | string | 创建时间 |
| `AliDomain` | boolean | 是否为阿里云注册域名 |

#### 请求示例

```http
GET /?Action=DescribeDomains
&KeyWord=example
&PageNumber=1
&PageSize=20
&Format=JSON
&Version=2015-01-09
&AccessKeyId=...
&Signature=...
&SignatureMethod=HMAC-SHA1
&SignatureNonce=...
&Timestamp=2024-01-01T12:00:00Z
&SignatureVersion=1.0 HTTP/1.1
Host: alidns.aliyuncs.com
```

#### 响应示例

```json
{
    "RequestId": "536E9CAD-DB30-4647-AC87-AA5CC38C5382",
    "TotalCount": 2,
    "PageNumber": 1,
    "PageSize": 20,
    "Domains": {
        "Domain": [
            {
                "DomainId": "00efd71f-770e-4255-b54e-bb8843f8b9c6",
                "DomainName": "example.com",
                "GroupId": "defaultGroup",
                "GroupName": "默认分组",
                "InstanceId": "",
                "VersionCode": "free",
                "RecordCount": 15,
                "AliDomain": true
            }
        ]
    }
}
```

---

## 4. 解析记录管理 API (DDNS 核心)

### 4.1 DescribeDomainRecords — 查询解析记录列表

根据域名查询其下所有解析记录，是 DDNS 更新流程中最先调用的接口。

**Action:** `DescribeDomainRecords`

#### 请求参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `DomainName` | string | **是** | 域名名称，如 `example.com` |
| `PageNumber` | integer | 否 | 页码，从 1 开始，默认 1 |
| `PageSize` | integer | 否 | 每页数量，默认 20，最大 500 |
| `RRKeyWord` | string | 否 | 主机记录关键字，**前后模糊匹配**，不区分大小写 |
| `KeyWord` | string | 否 | 关键字（全匹配搜索） |
| `TypeKeyWord` | string | 否 | 解析类型关键字，全匹配搜索 |
| `ValueKeyWord` | string | 否 | 记录值关键字，前后模糊匹配 |
| `Type` | string | 否 | 解析记录类型: `A`, `AAAA`, `CNAME`, `MX`, `TXT`, `NS`, `SRV`, `CAA` |
| `Line` | string | 否 | 解析线路，默认 `default` |
| `Status` | string | 否 | 记录状态: `Enable` / `Disable` |
| `SearchMode` | string | 否 | 搜索模式，见下方说明 |
| `OrderBy` | string | 否 | 排序字段 |
| `Direction` | string | 否 | 排序方向: `DESC` / `ASC` |
| `GroupId` | integer | 否 | 域名分组 ID |

**SearchMode 说明:**

| 值 | 行为 |
|------|------|
| `LIKE` | 与 `KeyWord` 配合，模糊搜索 |
| `EXACT` | 与 `KeyWord` 配合，精确搜索 |
| `ADVANCED` | 与 `RRKeyWord`、`TypeKeyWord`、`ValueKeyWord` 配合，支持模糊匹配 |
| `COMBINATION` | 同上但所有参数均为精确匹配 |

> 注意: 若传了 `KeyWord` 则默认搜索模式为 `LIKE`；若未传 `KeyWord`，则 `RRKeyWord` 和 `ValueKeyWord` 默认为模糊查询。

#### 响应字段

| 字段 | 类型 | 描述 |
|------|------|------|
| `RequestId` | string | 请求唯一标识 |
| `TotalCount` | integer | 记录总数 |
| `PageNumber` | integer | 当前页码 |
| `PageSize` | integer | 当前每页数量 |
| `DomainRecords` | object | 记录列表对象 |

**DomainRecords.Record 数组元素:**

| 字段 | 类型 | 描述 |
|------|------|------|
| `RecordId` | string | 解析记录唯一 ID (**更新/删除必需**) |
| `DomainName` | string | 域名名称 |
| `RR` | string | 主机记录 (如 `www`, `@`, `mail`) |
| `Type` | string | 记录类型 (A, AAAA, CNAME 等) |
| `Value` | string | 记录值 (如 IP 地址) |
| `TTL` | integer | 生存时间 (秒)，默认 600 |
| `Priority` | integer | MX 记录优先级 (1-50) |
| `Line` | string | 解析线路 |
| `Status` | string | 状态: `Enable` / `Disable` |
| `Locked` | boolean | 是否锁定 |
| `Weight` | integer | 权重 (1-100) |
| `Remark` | string | 备注 |

#### 请求示例

```http
GET /?Action=DescribeDomainRecords
&DomainName=example.com
&Type=A
&RRKeyWord=www
&PageSize=50
&Format=JSON
&Version=2015-01-09
&AccessKeyId=...
&Signature=...
&SignatureMethod=HMAC-SHA1
&SignatureNonce=...
&Timestamp=2024-01-01T12:00:00Z
&SignatureVersion=1.0 HTTP/1.1
Host: alidns.aliyuncs.com
```

#### 响应示例

```json
{
    "RequestId": "536E9CAD-DB30-4647-AC87-AA5CC38C5382",
    "TotalCount": 2,
    "PageNumber": 1,
    "PageSize": 50,
    "DomainRecords": {
        "Record": [
            {
                "RecordId": "999998544020218204",
                "DomainName": "example.com",
                "RR": "www",
                "Type": "A",
                "Value": "192.168.1.100",
                "TTL": 600,
                "Priority": 0,
                "Line": "default",
                "Status": "Enable",
                "Locked": false,
                "Weight": 1
            },
            {
                "RecordId": "999998544020218205",
                "DomainName": "example.com",
                "RR": "@",
                "Type": "A",
                "Value": "192.168.1.100",
                "TTL": 600,
                "Priority": 0,
                "Line": "default",
                "Status": "Enable",
                "Locked": false,
                "Weight": 1
            }
        ]
    }
}
```

#### Go 代码示例

```go
// QueryRecord 查询指定域名和主机记录的解析记录
func QueryRecord(client *http.Client, domainName, rr, recordType string,
    accessKeyId, accessKeySecret string) (recordId, value string, err error) {

    params := buildCommonParams(accessKeyId)
    params["Action"] = "DescribeDomainRecords"
    params["DomainName"] = domainName
    params["RRKeyWord"] = rr
    params["Type"] = recordType

    signature := Sign("GET", params, accessKeySecret)
    params["Signature"] = signature

    reqURL := "https://alidns.aliyuncs.com/?" + encodeParams(params)

    resp, err := client.Get(reqURL)
    if err != nil {
        return "", "", fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    var result struct {
        DomainRecords struct {
            Record []struct {
                RecordId string `json:"RecordId"`
                RR       string `json:"RR"`
                Type     string `json:"Type"`
                Value    string `json:"Value"`
            } `json:"Record"`
        } `json:"DomainRecords"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", "", fmt.Errorf("decode failed: %w", err)
    }

    for _, r := range result.DomainRecords.Record {
        if r.RR == rr && r.Type == recordType {
            return r.RecordId, r.Value, nil
        }
    }

    return "", "", fmt.Errorf("record not found: %s.%s (%s)", rr, domainName, recordType)
}
```

---

### 4.2 AddDomainRecord — 添加解析记录

**Action:** `AddDomainRecord`

#### 请求参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `DomainName` | string | **是** | 域名名称 |
| `RR` | string | **是** | 主机记录。要解析 `@` 则传 `@`，不能为空 |
| `Type` | string | **是** | 解析记录类型: `A`, `AAAA`, `CNAME`, `MX`, `TXT` 等 |
| `Value` | string | **是** | 记录值 (如 IP 地址 或 域名) |
| `TTL` | integer | 否 | 解析生效时间 (秒)，默认 600 |
| `Priority` | integer | 否 | MX 记录优先级 [1-50]，仅 Type=MX 时可用 |
| `Line` | string | 否 | 解析线路，默认 `default` |
| `Lang` | string | 否 | 语言: `zh` / `en` |

#### 响应字段

| 字段 | 类型 | 描述 |
|------|------|------|
| `RequestId` | string | 请求唯一标识 |
| `RecordId` | string | 新创建的解析记录 ID |

#### 请求示例

```http
GET /?Action=AddDomainRecord
&DomainName=example.com
&RR=home
&Type=A
&Value=203.0.113.50
&TTL=600
&Format=JSON
&Version=2015-01-09
&AccessKeyId=...
&Signature=...
&SignatureMethod=HMAC-SHA1
&SignatureNonce=...
&Timestamp=2024-01-01T12:00:00Z
&SignatureVersion=1.0 HTTP/1.1
Host: alidns.aliyuncs.com
```

#### 响应示例

```json
{
    "RequestId": "536E9CAD-DB30-4647-AC87-AA5CC38C5382",
    "RecordId": "999998544020218206"
}
```

---

### 4.3 UpdateDomainRecord — 修改解析记录 (DDNS 核心)

当公网 IP 变化时，调用此接口更新解析记录值。

**Action:** `UpdateDomainRecord`

#### 请求参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `RecordId` | string | **是** | 记录 ID，通过 `DescribeDomainRecords` 获取 |
| `RR` | string | **是** | 主机记录。主域名填 `@` |
| `Type` | string | **是** | 解析记录类型: `A`, `AAAA`, `CNAME` 等 |
| `Value` | string | **是** | 新的记录值 (如新 IP 地址) |
| `TTL` | integer | 否 | TTL 秒数，默认 600 |
| `Priority` | integer | 否 | MX 优先级 [1-50]，Type=MX 时必填 |
| `Line` | string | 否 | 解析线路，默认 `default` |
| `Lang` | string | 否 | 语言: `zh` / `en` |

#### 响应字段

| 字段 | 类型 | 描述 |
|------|------|------|
| `RequestId` | string | 请求唯一标识 |
| `RecordId` | string | 修改的记录 ID |

#### 请求示例

```http
GET /?Action=UpdateDomainRecord
&RecordId=999998544020218204
&RR=www
&Type=A
&Value=198.51.100.200
&TTL=600
&Format=JSON
&Version=2015-01-09
&AccessKeyId=...
&Signature=...
&SignatureMethod=HMAC-SHA1
&SignatureNonce=...
&Timestamp=2024-01-01T12:00:00Z
&SignatureVersion=1.0 HTTP/1.1
Host: alidns.aliyuncs.com
```

#### 响应示例

```json
{
    "RequestId": "536E9CAD-DB30-4647-AC87-AA5CC38C5382",
    "RecordId": "999998544020218204"
}
```

#### Go 代码示例

```go
// UpdateRecord 更新 DNS 记录值 (DDNS 核心操作)
func UpdateRecord(client *http.Client, recordId, rr, recordType, value string, ttl int,
    accessKeyId, accessKeySecret string) error {

    params := buildCommonParams(accessKeyId)
    params["Action"] = "UpdateDomainRecord"
    params["RecordId"] = recordId
    params["RR"] = rr
    params["Type"] = recordType
    params["Value"] = value
    if ttl > 0 {
        params["TTL"] = strconv.Itoa(ttl)
    }

    signature := Sign("GET", params, accessKeySecret)
    params["Signature"] = signature

    reqURL := "https://alidns.aliyuncs.com/?" + encodeParams(params)

    resp, err := client.Get(reqURL)
    if err != nil {
        return fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    var result struct {
        RequestId string `json:"RequestId"`
        RecordId  string `json:"RecordId"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return fmt.Errorf("decode failed: %w", err)
    }

    return nil
}
```

---

### 4.4 DeleteDomainRecord — 删除解析记录

**Action:** `DeleteDomainRecord`

#### 请求参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `RecordId` | string | **是** | 记录 ID，通过 `DescribeDomainRecords` 获取 |
| `Lang` | string | 否 | 语言: `zh` / `en` |

#### 响应字段

| 字段 | 类型 | 描述 |
|------|------|------|
| `RequestId` | string | 请求唯一标识 |
| `RecordId` | string | 被删除的记录 ID |

#### 请求示例

```http
GET /?Action=DeleteDomainRecord
&RecordId=999998544020218206
&Format=JSON
&Version=2015-01-09
&AccessKeyId=...
&Signature=...
&SignatureMethod=HMAC-SHA1
&SignatureNonce=...
&Timestamp=2024-01-01T12:00:00Z
&SignatureVersion=1.0 HTTP/1.1
Host: alidns.aliyuncs.com
```

#### 响应示例

```json
{
    "RequestId": "536E9CAD-DB30-4647-AC87-AA5CC38C5382",
    "RecordId": "999998544020218206"
}
```

---

### 4.5 SetDomainRecordStatus — 启用/停用解析记录

**Action:** `SetDomainRecordStatus`

#### 请求参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `RecordId` | string | **是** | 记录 ID |
| `Status` | string | **是** | `Enable` (启用) 或 `Disable` (暂停) |
| `Lang` | string | 否 | 语言: `zh` / `en` |

#### 响应字段

| 字段 | 类型 | 描述 |
|------|------|------|
| `RequestId` | string | 请求唯一标识 |
| `RecordId` | string | 记录 ID |
| `Status` | string | 当前状态: `Enable` / `Disable` |

---

### 4.6 DescribeDomainRecordInfo — 获取单条记录详细信息

**Action:** `DescribeDomainRecordInfo`

#### 请求参数

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `RecordId` | string | **是** | 记录 ID |
| `Lang` | string | 否 | 语言: `zh` / `en` |

#### 响应字段

返回单条记录的完整信息，字段与 `DescribeDomainRecords` 中的 Record 元素一致。

---

## 5. 公共辅助参数

所有 API 请求均可携带以下可选参数:

| 参数 | 类型 | 描述 |
|------|------|------|
| `UserClientIp` | string | 用户端 IP (用于安全审计) |
| `Lang` | string | 返回消息语言: `zh` (中文) 或 `en` (英文)，默认 `zh` |

---

## 6. 错误码

| HTTP 状态码 | 错误码 | 描述 |
|-------------|--------|------|
| 400 | `InvalidParameter` | 参数不合法 |
| 400 | `MissingParameter` | 缺少必填参数 |
| 403 | `Forbidden` | 无权限操作该资源 |
| 404 | `DomainRecordNotFound` | 解析记录不存在 |
| 404 | `DomainNotFound` | 域名不存在或不属于当前账号 |
| 500 | `InternalError` | 服务内部错误 |

---

## 7. 完整 DDNS 更新流程

典型的 DDNS (动态 DNS) 更新流程:

```
┌─────────────────────────────────────────────┐
│  1. 获取当前公网 IP                          │
│     (通过 ifconfig.co, ipify.org 等)          │
└──────────────────┬──────────────────────────┘
                   ▼
┌─────────────────────────────────────────────┐
│  2. DescribeDomainRecords                    │
│     查询目标域名和主机记录的当前 RecordId     │
│     和当前解析值 Value                        │
└──────────────────┬──────────────────────────┘
                   ▼
            ┌──────┴──────┐
            │  IP 是否变化? │
            └──────┬──────┘
             不变   │   变化
              ↓     │     ↓
            结束    │  ┌─────────────────────────────┐
                    │  │ 3. UpdateDomainRecord       │
                    │  │    用新 IP 更新 RecordId      │
                    │  └─────────────────────────────┘
                    │              │
                    ▼              ▼
                  ┌─────────────────────────────┐
                  │ 4. 记录日志，完成更新          │
                  └─────────────────────────────┘
```

### Go 完整实现

```go
package main

import (
    "crypto/hmac"
    "crypto/sha1"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "sort"
    "strconv"
    "strings"
    "time"

    "github.com/google/uuid"
)

// DNSUpdater 阿里云 DNS 更新器
type DNSUpdater struct {
    client     *http.Client
    accessKeyId string
    secret     string
}

func NewDNSUpdater(accessKeyId, accessKeySecret string) *DNSUpdater {
    return &DNSUpdater{
        client:     &http.Client{Timeout: 10 * time.Second},
        accessKeyId: accessKeyId,
        secret:     accessKeySecret,
    }
}

// GetCurrentIP 获取当前公网 IP
func (u *DNSUpdater) GetCurrentIP() (string, error) {
    resp, err := u.client.Get("https://ifconfig.co")
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    return strings.TrimSpace(string(body)), nil
}

// callAPI 通用 API 调用
func (u *DNSUpdater) callAPI(params map[string]string) (map[string]interface{}, error) {
    // 添加公共参数
    params["Format"] = "JSON"
    params["Version"] = "2015-01-09"
    params["AccessKeyId"] = u.accessKeyId
    params["SignatureMethod"] = "HMAC-SHA1"
    params["SignatureVersion"] = "1.0"
    params["SignatureNonce"] = uuid.New().String()
    params["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")

    // 签名
    signature := sign("GET", params, u.secret)

    // 构造 URL
    var parts []string
    for k, v := range params {
        parts = append(parts, percentEncode(k)+"="+percentEncode(v))
    }
    parts = append(parts, "Signature="+percentEncode(signature))

    reqURL := "https://alidns.aliyuncs.com/?" + strings.Join(parts, "&")

    resp, err := u.client.Get(reqURL)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    return result, nil
}

// QueryRecord 查询 DNS 记录
func (u *DNSUpdater) QueryRecord(domainName, rr, recordType string) (recordId, value string, err error) {
    params := map[string]string{
        "Action":     "DescribeDomainRecords",
        "DomainName": domainName,
        "RRKeyWord":  rr,
        "Type":       recordType,
    }

    result, err := u.callAPI(params)
    if err != nil {
        return "", "", err
    }

    records := result["DomainRecords"].(map[string]interface{})["Record"].([]interface{})
    for _, r := range records {
        rec := r.(map[string]interface{})
        if rec["RR"].(string) == rr && rec["Type"].(string) == recordType {
            return rec["RecordId"].(string), rec["Value"].(string), nil
        }
    }
    return "", "", fmt.Errorf("record not found")
}

// UpdateRecord 更新 DNS 记录
func (u *DNSUpdater) UpdateRecord(recordId, rr, recordType, value string, ttl int) error {
    params := map[string]string{
        "Action":   "UpdateDomainRecord",
        "RecordId": recordId,
        "RR":       rr,
        "Type":     recordType,
        "Value":    value,
        "TTL":      strconv.Itoa(ttl),
    }

    _, err := u.callAPI(params)
    return err
}

// SyncDDNS 执行一次 DDNS 同步
func (u *DNSUpdater) SyncDDNS(domainName, rr string) error {
    currentIP, err := u.GetCurrentIP()
    if err != nil {
        return fmt.Errorf("get current IP: %w", err)
    }

    recordId, currentValue, err := u.QueryRecord(domainName, rr, "A")
    if err != nil {
        return fmt.Errorf("query record: %w", err)
    }

    if currentValue == currentIP {
        fmt.Printf("[%s] IP unchanged: %s\n", time.Now().Format(time.RFC3339), currentIP)
        return nil
    }

    fmt.Printf("[%s] IP changed: %s -> %s, updating...\n",
        time.Now().Format(time.RFC3339), currentValue, currentIP)

    if err := u.UpdateRecord(recordId, rr, "A", currentIP, 600); err != nil {
        return fmt.Errorf("update record: %w", err)
    }

    fmt.Println("Update successful!")
    return nil
}
```

---

## 8. 使用阿里云 Go SDK (替代方案)

除了直接调用 HTTP API，也可以使用官方 Go SDK:

```bash
go get github.com/aliyun/alibaba-cloud-sdk-go
```

```go
import (
    "github.com/aliyun/alibaba-cloud-sdk-go/services/alidns"
)

func UpdateDNSWithSDK(accessKeyId, secret, regionId, recordId, rr, recordType, value string) error {
    client, err := alidns.NewClientWithAccessKey(regionId, accessKeyId, secret)
    if err != nil {
        return err
    }

    request := alidns.CreateUpdateDomainRecordRequest()
    request.RecordId = recordId
    request.RR = rr
    request.Type = recordType
    request.Value = value
    request.TTL = "600"

    _, err = client.UpdateDomainRecord(request)
    return err
}
```

---

## 9. 注意事项

1. **签名时间戳**: `Timestamp` 必须是 UTC 时间，与服务器时间差超过 15 分钟会拒绝请求
2. **SignatureNonce**: 每次请求必须不同，建议使用 UUID
3. **Percent-Encoding**: Go 的 `url.QueryEscape` 会将空格编码为 `+`，需替换为 `%20`
4. **TTL 限制**: 免费版 DNS 最低 TTL 为 600 秒 (10 分钟)，付费版可设更低 (最低 1 秒)
5. **请求频率**: 阿里云 API 有流控限制，建议 DDNS 检查间隔不低于 60 秒
6. **RecordId 持久性**: 解析记录的 RecordId 是稳定的，可以缓存以避免每次都查询
