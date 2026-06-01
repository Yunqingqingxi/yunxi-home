package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/yxd/yunxi-home/internal/database"
)

// HistoryHandler 历史记录 Handler
type HistoryHandler struct {
	historyRepo database.HistoryRepository
}

// NewHistoryHandler 创建历史记录 Handler
func NewHistoryHandler(historyRepo database.HistoryRepository) *HistoryHandler {
	return &HistoryHandler{historyRepo: historyRepo}
}

// List 分页获取历史记录
// GET /api/history?page=1&size=20&domain=example.com
func (h *HistoryHandler) List(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	size, _ := strconv.Atoi(c.QueryParam("size"))
	domain := c.QueryParam("domain")

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	params := database.ListParams{
		Domain: domain,
		Page:   page,
		Size:   size,
	}

	result, err := h.historyRepo.List(c.Request().Context(), params)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("获取历史记录失败"))
	}

	return c.JSON(http.StatusOK, successResp(result))
}

// GetStats 获取历史统计（供图表使用）
// GET /api/history/stats?days=7
func (h *HistoryHandler) GetStats(c echo.Context) error {
	days, _ := strconv.Atoi(c.QueryParam("days"))
	if days < 1 || days > 365 { days = 7 }
	stats, err := h.historyRepo.GetStats(c.Request().Context(), days)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("获取统计失败"))
	}
	return c.JSON(http.StatusOK, successResp(stats))
}

// CleanOld 清理旧记录
// DELETE /api/history/clean?days=90
func (h *HistoryHandler) CleanOld(c echo.Context) error {
	days, _ := strconv.Atoi(c.QueryParam("days"))
	if days < 1 {
		days = 30
	}

	n, err := h.historyRepo.CleanOld(c.Request().Context(), days)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("清理失败"))
	}

	return c.JSON(http.StatusOK, successResp(map[string]int64{"deleted": n}))
}
