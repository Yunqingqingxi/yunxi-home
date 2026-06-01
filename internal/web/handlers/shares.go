package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/nas"
)

// SharesHandler 分享管理 Handler
type SharesHandler struct {
	repo    database.ShareRepository
	shareSvc *nas.ShareService
}

// NewSharesHandler 创建分享 Handler
func NewSharesHandler(repo database.ShareRepository, shareSvc *nas.ShareService) *SharesHandler {
	return &SharesHandler{repo: repo, shareSvc: shareSvc}
}

// CreateShare 创建分享
// POST /api/nas/shares
func (h *SharesHandler) CreateShare(c echo.Context) error {
	var req struct {
		FilePath   string `json:"file_path"`
		ExpireDays int    `json:"expire_days"`
		Password   string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}

	if err := h.shareSvc.ValidatePath(req.FilePath); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp(err.Error()))
	}

	token, err := nas.GenerateToken()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("生成 token 失败"))
	}

	share := &nas.Share{
		Token:    token,
		FilePath: req.FilePath,
	}
	if req.Password != "" {
		share.Password = nas.HashPassword(req.Password)
		share.HasPass = true
	}
	if req.ExpireDays > 0 {
		share.ExpiresAt = time.Now().Add(time.Duration(req.ExpireDays) * 24 * time.Hour)
	}

	id, err := h.repo.Create(c.Request().Context(), share)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("创建分享失败"))
	}
	share.ID = id

	return c.JSON(http.StatusOK, successResp(map[string]interface{}{
		"id":        id,
		"token":     token,
		"file_path": share.FilePath,
		"share_url": "/s/" + token,
		"has_pass":  share.HasPass,
		"expires":   share.ExpiresAt,
	}))
}

// ListShares 列出所有分享
// GET /api/nas/shares
func (h *SharesHandler) ListShares(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	size, _ := strconv.Atoi(c.QueryParam("size"))
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	offset := (page - 1) * size

	shares, total, err := h.repo.List(c.Request().Context(), size, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("获取分享列表失败"))
	}
	if shares == nil {
		shares = []nas.Share{}
	}

	return c.JSON(http.StatusOK, successResp(map[string]interface{}{
		"items": shares,
		"total": total,
		"page":  page,
		"size":  size,
	}))
}

// DeleteShare 删除分享
// DELETE /api/nas/shares/:id
func (h *SharesHandler) DeleteShare(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("无效的 ID"))
	}
	if err := h.repo.Delete(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("删除失败"))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "已删除"}))
}

// AccessShare 通过 token 访问分享 (公开)
// GET /s/:token
func (h *SharesHandler) AccessShare(c echo.Context) error {
	token := c.Param("token")
	share, err := h.repo.GetByToken(c.Request().Context(), token)
	if err != nil || share == nil {
		return c.JSON(http.StatusNotFound, errorResp("分享不存在或已过期"))
	}
	if !share.ExpiresAt.IsZero() && time.Now().After(share.ExpiresAt) {
		return c.JSON(http.StatusGone, errorResp("分享已过期"))
	}

	// 检查密码
	if share.HasPass {
		pass := c.QueryParam("pass")
		if pass == "" {
			return c.JSON(http.StatusUnauthorized, errorResp("需要密码"))
		}
		if !nas.VerifyPassword(pass, share.Password) {
			return c.JSON(http.StatusForbidden, errorResp("密码错误"))
		}
	}

	// 增加下载计数
	h.repo.IncrementDownloads(c.Request().Context(), share.ID)

	return c.JSON(http.StatusOK, successResp(map[string]interface{}{
		"file_path": share.FilePath,
		"downloads": share.Downloads,
	}))
}
