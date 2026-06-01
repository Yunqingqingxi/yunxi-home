package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/Yunqingqingxi/yunxi-home/internal/docker"
)

// DockerHandler Docker 管理 Handler
type DockerHandler struct {
	mgr *docker.Manager
}

// NewDockerHandler 创建 Docker Handler
func NewDockerHandler(mgr *docker.Manager) *DockerHandler {
	return &DockerHandler{mgr: mgr}
}

// ListContainers 列出容器
// GET /api/docker/containers?all=true
func (h *DockerHandler) ListContainers(c echo.Context) error {
	all := c.QueryParam("all") == "true"
	containers, err := h.mgr.ListContainers(c.Request().Context(), all)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	if containers == nil {
		containers = []docker.ContainerInfo{}
	}
	return c.JSON(http.StatusOK, successResp(containers))
}

// ContainerAction 容器操作
// POST /api/docker/containers/:name/:action
func (h *DockerHandler) ContainerAction(c echo.Context) error {
	name := c.Param("name")
	action := c.Param("action")
	out, err := h.mgr.ContainerAction(c.Request().Context(), name, action)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{
		"message": "操作成功",
		"output":  out,
	}))
}

// GetLogs 获取容器日志
// GET /api/docker/containers/:name/logs?tail=100
func (h *DockerHandler) GetLogs(c echo.Context) error {
	name := c.Param("name")
	tail, _ := strconv.Atoi(c.QueryParam("tail"))
	if tail <= 0 {
		tail = 100
	}
	logs, err := h.mgr.GetLogs(c.Request().Context(), name, tail)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"logs": logs}))
}

// ComposeAction Docker Compose 操作
// POST /api/docker/compose/:action
func (h *DockerHandler) ComposeAction(c echo.Context) error {
	action := c.Param("action")
	projectDir := c.QueryParam("dir")
	if projectDir == "" {
		projectDir = "/app/deploy"
	}
	out, err := h.mgr.ComposeAction(c.Request().Context(), projectDir, action)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{
		"message": "compose " + action + " 成功",
		"output":  out,
	}))
}

// Stats 容器资源统计
// GET /api/docker/containers/:name/stats
func (h *DockerHandler) Stats(c echo.Context) error {
	name := c.Param("name")
	stats, err := h.mgr.Stats(c.Request().Context(), name)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(stats))
}
