package handlers

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/models"
	"github.com/Yunqingqingxi/yunxi-home/internal/web/middleware"
)

type loginAttempt struct {
	count     int
	firstSeen time.Time
}

type AuthHandler struct {
	userRepo     database.UserRepository
	jwtCfg       middleware.JWTConfig
	loginLimiter sync.Map // map[string]*loginAttempt (per-IP)
}

func NewAuthHandler(userRepo database.UserRepository, jwtCfg middleware.JWTConfig) *AuthHandler {
	return &AuthHandler{userRepo: userRepo, jwtCfg: jwtCfg}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// POST /api/auth/login
func (h *AuthHandler) Login(c echo.Context) error {
	// 登录限流：每 IP 每分钟最多 5 次，超过锁定 15 分钟
	ip := c.RealIP()
	now := time.Now()
	if v, ok := h.loginLimiter.Load(ip); ok {
		attempt := v.(*loginAttempt)
		if now.Sub(attempt.firstSeen) > 15*time.Minute {
			h.loginLimiter.Delete(ip)
		} else if attempt.count >= 5 {
			if now.Sub(attempt.firstSeen) < 15*time.Minute {
				return c.JSON(http.StatusTooManyRequests, errorResp("登录尝试过于频繁，请 15 分钟后重试"))
			}
			h.loginLimiter.Delete(ip)
		}
	}

	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("请求参数无效"))
	}
	if req.Username == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, errorResp("用户名和密码不能为空"))
	}

	user, err := h.userRepo.GetByUsername(c.Request().Context(), req.Username)
	if err != nil {
		h.recordLoginFailure(ip, now)
		return c.JSON(http.StatusUnauthorized, errorResp("用户名或密码错误"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		h.recordLoginFailure(ip, now)
		return c.JSON(http.StatusUnauthorized, errorResp("用户名或密码错误"))
	}

	// 登录成功，清除限流记录
	h.loginLimiter.Delete(ip)

	token, err := middleware.GenerateToken(h.jwtCfg, user.ID, user.Username, string(user.Role))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("生成 Token 失败"))
	}

	return c.JSON(http.StatusOK, successResp(LoginResponse{
		Token:    token,
		Username: user.Username,
		Role:     string(user.Role),
	}))
}

// GET /api/auth/status — check if system needs initial setup
func (h *AuthHandler) Status(c echo.Context) error {
	// 标记文件方式：兼容双库模式（SQLite+MySQL），避免数据库不同步问题
	if _, err := os.Stat("/opt/yunxi-home/data/.needs_setup"); err == nil {
		return c.JSON(http.StatusOK, successResp(map[string]bool{"needs_setup": true}))
	}
	admin, err := h.userRepo.GetByUsername(c.Request().Context(), "admin")
	if err != nil || admin.PasswordHash == "" || admin.PasswordHash == "$2a$10$default" {
		return c.JSON(http.StatusOK, successResp(map[string]bool{"needs_setup": true}))
	}
	return c.JSON(http.StatusOK, successResp(map[string]bool{"needs_setup": false}))
}

// POST /api/auth/setup — initial admin password setup
func (h *AuthHandler) Setup(c echo.Context) error {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil || req.Password == "" {
		return c.JSON(http.StatusBadRequest, errorResp("密码不能为空"))
	}
	if len(req.Password) < 6 {
		return c.JSON(http.StatusBadRequest, errorResp("密码至少 6 位"))
	}
	adminUser, err := h.userRepo.GetByUsername(c.Request().Context(), "admin")
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		// Admin user doesn't exist yet — create it
		_, err = h.userRepo.Create(c.Request().Context(), &models.User{
				Username:     "admin",
				PasswordHash: string(hash),
				Role:         "admin",
			})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResp("创建管理员失败: "+err.Error()))
		}
	} else {
		if err := h.userRepo.UpdatePassword(c.Request().Context(), adminUser.ID, string(hash)); err != nil {
			return c.JSON(http.StatusInternalServerError, errorResp("设置失败: "+err.Error()))
		}
	}
	os.Remove("/opt/yunxi-home/data/.needs_setup")
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "管理员密码已设置"}))
}

// POST /api/auth/change-password — change password (requires auth)
func (h *AuthHandler) ChangePassword(c echo.Context) error {
	claims := middleware.GetClaims(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, errorResp("未登录"))
	}
	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := c.Bind(&req); err != nil || req.New == "" {
		return c.JSON(http.StatusBadRequest, errorResp("新密码不能为空"))
	}
	if len(req.New) < 6 {
		return c.JSON(http.StatusBadRequest, errorResp("新密码至少 6 位"))
	}
	user, err := h.userRepo.GetByID(c.Request().Context(), claims.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("用户不存在"))
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Current)); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("当前密码错误"))
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.New), 12)
	if err := h.userRepo.UpdatePassword(c.Request().Context(), user.ID, string(hash)); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("修改失败: "+err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "密码已修改"}))
}

// POST /api/auth/refresh
func (h *AuthHandler) Refresh(c echo.Context) error {
	auth := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return c.JSON(http.StatusUnauthorized, errorResp("缺少认证信息"))
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")

	claims, err := middleware.ParseToken(tokenStr, h.jwtCfg.Secret)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, errorResp("Token 无效或已过期"))
	}

	token, err := middleware.GenerateToken(h.jwtCfg, claims.UserID, claims.Username, claims.Role)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("刷新 Token 失败"))
	}

	return c.JSON(http.StatusOK, successResp(LoginResponse{
		Token:    token,
		Username: claims.Username,
		Role:     claims.Role,
	}))
}

// recordLoginFailure 记录登录失败，per-IP 限流
func (h *AuthHandler) recordLoginFailure(ip string, now time.Time) {
	v, _ := h.loginLimiter.LoadOrStore(ip, &loginAttempt{count: 0, firstSeen: now})
	attempt := v.(*loginAttempt)
	attempt.count++
}
