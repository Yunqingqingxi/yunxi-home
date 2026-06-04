package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/golang-lru/v2"
	"github.com/labstack/echo/v4"

	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/models"
	echomw "github.com/Yunqingqingxi/yunxi-home/internal/web/middleware"
)

// ── Token 提取 ─────────────────────────────────────

// getClaimsFromHeader extracts *echomw.Claims from Authorization header or ?token= query.
func getClaimsFromHeader(c echo.Context, jwtSecret string) *echomw.Claims {
	auth := c.Request().Header.Get("Authorization")
	var tokenStr string
	if strings.HasPrefix(auth, "Bearer ") {
		tokenStr = strings.TrimPrefix(auth, "Bearer ")
	}
	if tokenStr == "" {
		tokenStr = c.QueryParam("token")
	}
	if tokenStr == "" {
		return nil
	}
	claims, err := echomw.ParseToken(tokenStr, jwtSecret)
	if err != nil || claims == nil {
		return nil
	}
	return claims
}

// ── 权限缓存（LRU，防内存泄漏） ──────────────────

const (
	permCacheMaxSize = 10000 // 最大缓存条目数
	permCacheTTL     = 30   // 秒
)

type permCacheEntry struct {
	perm     *models.FilePermission
	expireAt time.Time
}

var permCache *lru.Cache[string, *permCacheEntry]

func init() {
	var err error
	permCache, err = lru.New[string, *permCacheEntry](permCacheMaxSize)
	if err != nil {
		panic(fmt.Sprintf("初始化权限缓存失败: %v", err))
	}
}

func cacheKey(userID int64, path string) string {
	return fmt.Sprintf("%d:%s", userID, path)
}

func getCachedPerm(userID int64, path string) *models.FilePermission {
	e, ok := permCache.Get(cacheKey(userID, path))
	if ok && time.Now().Before(e.expireAt) {
		return e.perm
	}
	return nil
}

func setCachedPerm(userID int64, path string, perm *models.FilePermission) {
	permCache.Add(cacheKey(userID, path), &permCacheEntry{
		perm:     perm,
		expireAt: time.Now().Add(permCacheTTL * time.Second),
	})
}

// ── 文件访问中间件 ────────────────────────────────

// FileAccess creates middleware that checks file-level permissions.
func FileAccess(permRepo database.FilePermissionRepository, userRepo database.UserRepository, jwtSecret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := getClaimsFromHeader(c, jwtSecret)
			if claims == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "未授权访问，请先登录"})
			}

			// 管理员跳过所有权限检查
			if claims.Role == string(models.RoleAdmin) {
				return next(c)
			}

			filePath := extractFilePath(c)
			if filePath == "" {
				// 无法提取路径的请求放行（如部分 GET 请求）
				return next(c)
			}

			filePath = NormalizePath(filePath)

			// 用户对自己 home 目录有完整权限
			if homePerm := database.GetUserHomePerm(claims.Username, filePath); homePerm != nil && homePerm.HasAccess() {
				return next(c)
			}

			// 查询用户 + 权限（带缓存）
			dbUser, err := userRepo.GetByUsername(c.Request().Context(), claims.Username)
			if err != nil {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": fmt.Sprintf("用户不存在: %s", claims.Username),
				})
			}

			perm := getCachedPerm(dbUser.ID, filePath)
			if perm == nil {
				perm, err = permRepo.GetByUserAndPath(c.Request().Context(), dbUser.ID, filePath)
				if err != nil || perm == nil {
					return c.JSON(http.StatusForbidden, map[string]string{
						"error": fmt.Sprintf("无访问权限: %s (用户 %s 没有该路径的权限)", filePath, claims.Username),
						"path":  filePath,
					})
				}
				setCachedPerm(dbUser.ID, filePath, perm)
			}

			method := c.Request().Method
			needed := ""
			switch method {
			case "GET":
				if !perm.CanRead {
					needed = "读取(read)"
				}
			case "POST", "PUT":
				if !perm.CanWrite {
					needed = "写入(write)"
				}
			case "DELETE":
				if !perm.CanDelete {
					needed = "删除(delete)"
				}
			}
			if needed != "" {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": fmt.Sprintf("权限不足: %s 需要 %s 权限", filePath, needed),
					"path":  filePath,
					"need":  needed,
				})
			}

			c.Set("file_perm", perm)
			return next(c)
		}
	}
}

// ── 路径提取 ──────────────────────────────────────

func extractFilePath(c echo.Context) string {
	// 1) Query param (最常用)
	if p := c.QueryParam("path"); p != "" && p != "/" {
		return p
	}

	// 2) Multipart 上传 — 只从 form field 提取，不解析 body
	ct := c.Request().Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if p := c.FormValue("dir"); p != "" && p != "/" {
			return p
		}
		return ""
	}

	// 3) GET/HEAD — 路径只在 query param 中
	method := c.Request().Method
	if method == "GET" || method == "HEAD" {
		return ""
	}

	// 4) JSON body: 提取 path / old_path / src / dst / paths
	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil || len(bodyBytes) == 0 {
		return ""
	}
	c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var body map[string]interface{}
	if json.Unmarshal(bodyBytes, &body) != nil {
		return ""
	}
	// 按优先级检查各字段
	for _, key := range []string{"path", "old_path", "src", "dst"} {
		if p, ok := body[key].(string); ok && p != "" && p != "/" {
			return p
		}
	}
	// 批量删除的 paths 数组 — 取第一个
	if paths, ok := body["paths"].([]interface{}); ok && len(paths) > 0 {
		if p, ok := paths[0].(string); ok {
			return p
		}
	}
	return ""
}

// ── 管理员中间件 ──────────────────────────────────

func RequireAdmin(jwtSecret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := getClaimsFromHeader(c, jwtSecret)
			if claims == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "未授权访问，请先登录"})
			}
			if claims.Role != string(models.RoleAdmin) {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error":   "需要管理员权限",
					"current": claims.Role,
				})
			}
			return next(c)
		}
	}
}

// ── 工具函数 ──────────────────────────────────────

// NormalizePath ensures paths start with "/" and removes trailing "/"
func NormalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}
