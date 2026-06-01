package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/yxd/yunxi-home/internal/sysctl"
)

// SysctlHandler 系统控制 Handler
type SysctlHandler struct {
	ctrl sysctl.Controller
}

// NewSysctlHandler 创建系统控制 Handler
func NewSysctlHandler(ctrl sysctl.Controller) *SysctlHandler {
	return &SysctlHandler{ctrl: ctrl}
}

// GetSystemInfo 获取系统信息
// GET /api/sysctl/info
func (h *SysctlHandler) GetSystemInfo(c echo.Context) error {
	info, err := h.ctrl.GetSystemInfo()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(info))
}

// ListProcesses 列出进程
// GET /api/sysctl/processes?limit=50
func (h *SysctlHandler) ListProcesses(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 50
	}
	procs, err := h.ctrl.ListProcesses(limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	if procs == nil {
		procs = []sysctl.ProcessInfo{}
	}
	return c.JSON(http.StatusOK, successResp(procs))
}

// KillProcess 终止进程
// POST /api/sysctl/processes/:pid/kill
func (h *SysctlHandler) KillProcess(c echo.Context) error {
	pid, err := strconv.Atoi(c.Param("pid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("无效的 PID"))
	}
	var req struct {
		Force bool `json:"force"`
	}
	c.Bind(&req)

	if err := h.ctrl.KillProcess(pid, req.Force); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": fmt.Sprintf("进程 %d 已终止", pid)}))
}

// ListServices 列出服务
// GET /api/sysctl/services
func (h *SysctlHandler) ListServices(c echo.Context) error {
	services, err := h.ctrl.ListServices()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	if services == nil {
		services = []sysctl.ServiceInfo{}
	}
	return c.JSON(http.StatusOK, successResp(services))
}

// ControlService 控制服务
// POST /api/sysctl/services/:name/:action
func (h *SysctlHandler) ControlService(c echo.Context) error {
	name := c.Param("name")
	action := c.Param("action")
	if err := h.ctrl.ControlService(name, action); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{
		"message": fmt.Sprintf("服务 %s %s 成功", name, action),
	}))
}
