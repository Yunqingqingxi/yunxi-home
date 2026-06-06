package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// GlobalJWTSecret is set during server init for shared access.
var GlobalJWTSecret string

// JWTConfig JWT 认证配置
type JWTConfig struct {
	Secret     string
	Expiration time.Duration
}

// Claims JWT 声明
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT Token
func GenerateToken(cfg JWTConfig, userID int64, username, role string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.Expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "yunxi-home",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// ParseToken 解析 JWT Token
func ParseToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

// JWTAuth JWT 认证中间件
func JWTAuth(secret string) echo.MiddlewareFunc {
	GlobalJWTSecret = secret
	config := echomw.JWTConfig{
		SigningKey:  []byte(secret),
		TokenLookup: "header:Authorization:Bearer ,cookie:token",
		ContextKey:  "user",
		ErrorHandler: func(err error) error {
			return echo.NewHTTPError(http.StatusUnauthorized, map[string]string{
				"error": "未授权访问，请先登录",
			})
		},
	}

	return echomw.JWTWithConfig(config)
}

// GetClaims 从 echo.Context 获取 JWT claims
func GetClaims(c echo.Context) *Claims {
	token, ok := c.Get("user").(*jwt.Token)
	if !ok {
		return nil
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// BasicAuth Basic Auth 中间件（兼容旧版）
func BasicAuth(username, password string) echo.MiddlewareFunc {
	return echomw.BasicAuth(func(u, p string, c echo.Context) (bool, error) {
		return u == username && p == password, nil
	})
}

// extractToken 从请求头提取 Token
func extractToken(c echo.Context) string {
	auth := c.Request().Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
