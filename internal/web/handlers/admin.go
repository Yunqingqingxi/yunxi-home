package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/models"
)

// AdminHandler admin management endpoints
type AdminHandler struct {
	userRepo database.UserRepository
	permRepo database.FilePermissionRepository
}

func NewAdminHandler(userRepo database.UserRepository, permRepo database.FilePermissionRepository) *AdminHandler {
	return &AdminHandler{userRepo: userRepo, permRepo: permRepo}
}

// ── User Management ────────────────────────────

// ListUsers GET /api/admin/users
func (h *AdminHandler) ListUsers(c echo.Context) error {
	users, err := h.userRepo.List(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(users))
}

// CreateUser POST /api/admin/users
func (h *AdminHandler) CreateUser(c echo.Context) error {
	var req struct {
		Username     string `json:"username"`
		Password     string `json:"password"`
		Role         string `json:"role"`
		StorageQuota int64  `json:"storage_quota"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if req.Username == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, errorResp("用户名和密码不能为空"))
	}
	if req.Role != "admin" && req.Role != "user" {
		req.Role = "user"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("密码加密失败"))
	}

	user := &models.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         models.UserRole(req.Role),
		StorageQuota: req.StorageQuota,
	}
	id, err := h.userRepo.Create(c.Request().Context(), user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]interface{}{"id": id, "message": "用户已创建"}))
}

// DeleteUser DELETE /api/admin/users/:id
func (h *AdminHandler) DeleteUser(c echo.Context) error {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.userRepo.Delete(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "用户已删除"}))
}

// UpdateUser PUT /api/admin/users/:id
func (h *AdminHandler) UpdateUser(c echo.Context) error {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Password     string `json:"password"`
		Role         string `json:"role"`
		StorageQuota int64  `json:"storage_quota"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResp("密码加密失败"))
		}
		if err := h.userRepo.UpdatePassword(c.Request().Context(), id, string(hash)); err != nil {
			return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
		}
	}
	if req.Role == "admin" || req.Role == "user" {
		if err := h.userRepo.UpdateRole(c.Request().Context(), id, req.Role); err != nil {
			return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
		}
	}
	if req.StorageQuota > 0 {
		if err := h.userRepo.UpdateQuota(c.Request().Context(), id, req.StorageQuota); err != nil {
			return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
		}
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "用户已更新"}))
}

// ── File Permission Management ──────────────────

// ListFilePerms GET /api/admin/file-permissions?user_id=...
func (h *AdminHandler) ListFilePerms(c echo.Context) error {
	userIDStr := c.QueryParam("user_id")
	if userIDStr != "" {
		userID, _ := strconv.ParseInt(userIDStr, 10, 64)
		perms, err := h.permRepo.ListByUser(c.Request().Context(), userID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
		}
		return c.JSON(http.StatusOK, successResp(perms))
	}
	// List all
	perms, err := h.permRepo.ListAll(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(perms))
}

// UpsertFilePerm POST /api/admin/file-permissions
func (h *AdminHandler) UpsertFilePerm(c echo.Context) error {
	var req struct {
		UserID    int64  `json:"user_id"`
		Path      string `json:"path"`
		CanRead   bool   `json:"can_read"`
		CanWrite  bool   `json:"can_write"`
		CanDelete bool   `json:"can_delete"`
		CanShare  bool   `json:"can_share"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if req.UserID == 0 || req.Path == "" {
		return c.JSON(http.StatusBadRequest, errorResp("user_id 和 path 不能为空"))
	}

	perm := &models.FilePermission{
		UserID:    req.UserID,
		Path:      req.Path,
		CanRead:   req.CanRead,
		CanWrite:  req.CanWrite,
		CanDelete: req.CanDelete,
		CanShare:  req.CanShare,
	}
	if err := h.permRepo.Upsert(c.Request().Context(), perm); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "权限已保存"}))
}

// DeleteFilePerm DELETE /api/admin/file-permissions/:id
func (h *AdminHandler) DeleteFilePerm(c echo.Context) error {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.permRepo.Delete(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "权限已删除"}))
}