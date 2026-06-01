package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Yunqingqingxi/yunxi-home/internal/dns"
	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/models"
	"github.com/Yunqingqingxi/yunxi-home/internal/scheduler"
)

// DomainHandler 域名管理 Handler
type DomainHandler struct {
	domainRepo database.DomainRepository
	sched      *scheduler.Scheduler
	dnsSvc     dns.Provider
}

// NewDomainHandler 创建域名管理 Handler
func NewDomainHandler(domainRepo database.DomainRepository, sched *scheduler.Scheduler, dnsSvc dns.Provider) *DomainHandler {
	return &DomainHandler{domainRepo: domainRepo, sched: sched, dnsSvc: dnsSvc}
}

// CreateDomainRequest 创建域名记录请求
type CreateDomainRequest struct {
	Domain   string `json:"domain" validate:"required"`
	RR       string `json:"rr" validate:"required"`
	Type     string `json:"type" validate:"required"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	CronExpr string `json:"cron_expr"`
	Enabled  bool   `json:"enabled"`
}

// UpdateDomainRequest 更新域名记录请求
type UpdateDomainRequest struct {
	Domain   string `json:"domain"`
	RR       string `json:"rr"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	CronExpr string `json:"cron_expr"`
	Enabled  *bool  `json:"enabled"`
}

// List 获取所有域名记录
// GET /api/domains
func (h *DomainHandler) List(c echo.Context) error {
	records, err := h.domainRepo.List(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("获取域名列表失败"))
	}
	if records == nil {
		records = []models.DomainRecord{}
	}
	return c.JSON(http.StatusOK, successResp(records))
}

// Get 获取单条域名记录
// GET /api/domains/:id
func (h *DomainHandler) Get(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("无效的 ID"))
	}

	rec, err := h.domainRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, errorResp("记录不存在"))
		}
		return c.JSON(http.StatusInternalServerError, errorResp("获取记录失败"))
	}

	return c.JSON(http.StatusOK, successResp(rec))
}

// Create 创建域名记录，并自动触发一次更新
// POST /api/domains
func (h *DomainHandler) Create(c echo.Context) error {
	var req CreateDomainRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("请求参数无效"))
	}
	if req.Domain == "" || req.RR == "" || req.Type == "" {
		return c.JSON(http.StatusBadRequest, errorResp("domain, rr, type 为必填项"))
	}
	if req.Type != "A" && req.Type != "AAAA" {
		return c.JSON(http.StatusBadRequest, errorResp("type 必须为 A 或 AAAA"))
	}
	if req.TTL <= 0 {
		req.TTL = 600
	}
	if req.CronExpr == "" {
		req.CronExpr = "0 */5 * * * *"
	}

	rec := &models.DomainRecord{
		Domain:   req.Domain,
		RR:       req.RR,
		Type:     req.Type,
		TTL:      req.TTL,
		CronExpr: req.CronExpr,
		Enabled:  req.Enabled,
	}

	id, err := h.domainRepo.Create(c.Request().Context(), rec)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("创建记录失败"))
	}

	rec.ID = id

	// 注册独立 cron 并立即触发一次更新
	if h.sched != nil {
		h.sched.RegisterRecord(*rec)
		if rec.Enabled {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_ = h.sched.TriggerUpdate(ctx)
			}()
		}
	}

	return c.JSON(http.StatusCreated, successResp(rec))
}

// Update 更新域名记录，如果启用了则触发更新
// PUT /api/domains/:id
func (h *DomainHandler) Update(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("无效的 ID"))
	}

	var req UpdateDomainRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("请求参数无效"))
	}

	rec, err := h.domainRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, errorResp("记录不存在"))
		}
		return c.JSON(http.StatusInternalServerError, errorResp("获取记录失败"))
	}

	wasEnabled := rec.Enabled

	if req.Domain != "" {
		rec.Domain = req.Domain
	}
	if req.RR != "" {
		rec.RR = req.RR
	}
	if req.Type != "" {
		rec.Type = req.Type
	}
	if req.Value != "" {
		rec.Value = req.Value
	}
	if req.TTL > 0 {
		rec.TTL = req.TTL
	}
	if req.CronExpr != "" {
		rec.CronExpr = req.CronExpr
	}
	if req.Enabled != nil {
		rec.Enabled = *req.Enabled
	}

	if err := h.domainRepo.Update(c.Request().Context(), rec); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("更新记录失败"))
	}

	// 更新 cron 注册，必要时触发更新
	if h.sched != nil {
		h.sched.RegisterRecord(*rec)
		if rec.Enabled && (!wasEnabled || req.Domain != "" || req.RR != "" || req.Type != "") {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_ = h.sched.TriggerUpdate(ctx)
			}()
		}
	}

	return c.JSON(http.StatusOK, successResp(rec))
}

// Delete 删除域名记录
// DELETE /api/domains/:id
func (h *DomainHandler) Delete(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("无效的 ID"))
	}

	if err := h.domainRepo.Delete(c.Request().Context(), id); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, errorResp("记录不存在"))
		}
		return c.JSON(http.StatusInternalServerError, errorResp("删除记录失败"))
	}

	if h.sched != nil {
		h.sched.UnregisterRecord(id)
	}

	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "删除成功"}))
}

// ListCloudRecords 获取阿里云上某域名的所有解析记录
// GET /api/domains/cloud/records?domain=xxx&page=1&size=50
func (h *DomainHandler) ListCloudRecords(c echo.Context) error {
	domain := c.QueryParam("domain")
	if domain == "" {
		return c.JSON(http.StatusBadRequest, errorResp("domain 参数必填"))
	}
	page, _ := strconv.Atoi(c.QueryParam("page"))
	size, _ := strconv.Atoi(c.QueryParam("size"))

	records, total, err := h.dnsSvc.ListAllRecords(c.Request().Context(), domain, page, size)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("获取云记录失败"))
	}

	return c.JSON(http.StatusOK, successResp(map[string]interface{}{
		"records": records,
		"total":   total,
		"page":    page,
		"size":    size,
	}))
}

// CreateCloudRecord 在阿里云上添加解析记录
// POST /api/domains/cloud/records
func (h *DomainHandler) CreateCloudRecord(c echo.Context) error {
	var req CreateDomainRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("请求参数无效"))
	}
	if req.Domain == "" || req.RR == "" || req.Type == "" {
		return c.JSON(http.StatusBadRequest, errorResp("domain, rr, type 为必填项"))
	}
	recordID, err := h.dnsSvc.AddRecord(c.Request().Context(), req.Domain, req.RR, req.Type, req.Value, req.TTL)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("添加云记录失败"))
	}
	return c.JSON(http.StatusCreated, successResp(map[string]string{"record_id": recordID, "message": "已创建"}))
}

// UpdateCloudRecord 更新阿里云解析记录
// PUT /api/domains/cloud/records/:recordId
func (h *DomainHandler) UpdateCloudRecord(c echo.Context) error {
	recordID := c.Param("recordId")
	if recordID == "" {
		return c.JSON(http.StatusBadRequest, errorResp("recordId 不能为空"))
	}
	var req UpdateDomainRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("请求参数无效"))
	}
	if err := h.dnsSvc.UpdateRecord(c.Request().Context(), recordID, req.RR, req.Type, req.Value, req.TTL); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("更新云记录失败"))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "已更新"}))
}

// DeleteCloudRecord 删除阿里云解析记录
// DELETE /api/domains/cloud/records/:recordId
func (h *DomainHandler) DeleteCloudRecord(c echo.Context) error {
	recordID := c.Param("recordId")
	if recordID == "" {
		return c.JSON(http.StatusBadRequest, errorResp("recordId 不能为空"))
	}
	if err := h.dnsSvc.DeleteRecord(c.Request().Context(), recordID); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("删除云记录失败"))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "已删除"}))
}

// ListCloudDomains 获取阿里云账号下的域名列表
// GET /api/domains/cloud?keyword=&page=1&size=20
func (h *DomainHandler) ListCloudDomains(c echo.Context) error {
	keyword := c.QueryParam("keyword")
	page, _ := strconv.Atoi(c.QueryParam("page"))
	size, _ := strconv.Atoi(c.QueryParam("size"))

	result, err := h.dnsSvc.ListDomains(c.Request().Context(), keyword, page, size)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("获取云域名失败"))
	}

	return c.JSON(http.StatusOK, successResp(result))
}
