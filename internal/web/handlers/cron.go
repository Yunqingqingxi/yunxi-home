package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai"
)

// CronHandler 定时任务 API
type CronHandler struct {
	aiService *ai.Service
}

// NewCronHandler creates a CronHandler.
func NewCronHandler(svc *ai.Service) *CronHandler {
	return &CronHandler{aiService: svc}
}

// ListTasks 列出指定会话的定时任务 GET /api/cron/tasks?session_id=xxx
func (h *CronHandler) ListTasks(c echo.Context) error {
	sessionID := c.QueryParam("session_id")
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id"))
	}
	tasks := h.aiService.ListCronTasks(sessionID)
	if tasks == nil {
		return c.JSON(http.StatusOK, successResp([]any{}))
	}
	return c.JSON(http.StatusOK, successResp(tasks))
}

// DeleteTask 删除单个定时任务 DELETE /api/cron/tasks/:id
func (h *CronHandler) DeleteTask(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少任务 ID"))
	}
	if ok := h.aiService.DeleteCronTask(id); !ok {
		return c.JSON(http.StatusNotFound, errorResp("任务不存在"))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "已删除"}))
}
