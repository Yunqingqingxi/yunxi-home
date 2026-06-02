package scheduler

import (
	"context"
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/dns"
	"github.com/Yunqingqingxi/yunxi-home/internal/ipdetect"
	"github.com/Yunqingqingxi/yunxi-home/internal/models"
	"github.com/Yunqingqingxi/yunxi-home/internal/notifier"
)

var log = logger.ForComponent("scheduler")

// Scheduler DNS 更新调度器
type Scheduler struct {
	detector ipdetect.Detector
	dnsSvc   dns.Provider
	notifier *notifier.Manager

	domainRepo  database.DomainRepository
	historyRepo database.HistoryRepository

	cron     *cron.Cron
	interval string
	entryIDs map[int64]cron.EntryID // record ID → cron entry
	mu       sync.RWMutex

	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
}

// New 创建调度器
func New(
	detector ipdetect.Detector,
	dnsSvc dns.Provider,
	domainRepo database.DomainRepository,
	historyRepo database.HistoryRepository,
	nm *notifier.Manager,
	interval string,
) *Scheduler {
	return &Scheduler{
		detector:    detector,
		dnsSvc:      dnsSvc,
		notifier:    nm,
		domainRepo:  domainRepo,
		historyRepo: historyRepo,
		interval:    interval,
		entryIDs:    make(map[int64]cron.EntryID),
	}
}

// Start 启动定时任务，为每条记录注册独立的 cron
func (s *Scheduler) Start() error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.cron = cron.New(cron.WithSeconds())

	// 加载所有已启用的记录，各自注册 cron
	records, err := s.domainRepo.ListEnabled(s.ctx)
	if err != nil {
		return fmt.Errorf("加载域名记录失败: %w", err)
	}

	for _, rec := range records {
		s.registerRecord(rec)
	}

	// 立即执行一次全量检查
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.checkAndUpdate(s.ctx)
	}()

	s.cron.Start()
	s.running = true
	log.Info("调度器已启动",
		"global_interval", s.interval,
		"per_record_schedules", len(s.entryIDs),
	)

	return nil
}

func (s *Scheduler) registerRecord(rec models.DomainRecord) {
	expr := rec.CronExpr
	if expr == "" {
		expr = s.interval
	}
	if expr == "" {
		expr = "0 */5 * * * *"
	}

	recID := rec.ID
	entryID, err := s.cron.AddFunc(expr, func() {
		s.wg.Add(1)
		defer s.wg.Done()
		// 重新从 DB 获取最新记录（可能已被修改/删除）
		current, err := s.domainRepo.GetByID(s.ctx, recID)
		if err != nil || current == nil || !current.Enabled {
			return
		}
		if err := s.updateSingleRecord(s.ctx, *current); err != nil {
			log.Error("更新记录失败", "domain", current.Domain, "rr", current.RR, "error", err)
		}
	})
	if err != nil {
		log.Warn("注册 cron 失败", "record_id", rec.ID, "cron", expr, "error", err)
		return
	}
	s.entryIDs[rec.ID] = entryID
	log.Debug("已注册定时任务", "domain", rec.Domain, "rr", rec.RR, "cron", expr)
}

func (s *Scheduler) unregisterRecord(recID int64) {
	if entryID, ok := s.entryIDs[recID]; ok {
		s.cron.Remove(entryID)
		delete(s.entryIDs, recID)
	}
}

// RegisterRecord 外部调用：为新增/更新的记录注册 cron，先移除旧的再注册
func (s *Scheduler) RegisterRecord(rec models.DomainRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.unregisterRecord(rec.ID)
	if rec.Enabled {
		s.registerRecord(rec)
	}
}

// UnregisterRecord 外部调用：删除记录时移除 cron
func (s *Scheduler) UnregisterRecord(recID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unregisterRecord(recID)
}

// Stop 优雅关闭调度器
func (s *Scheduler) Stop() {
	log.Info("调度器正在停止...")

	if s.cancel != nil {
		s.cancel()
	}

	if s.cron != nil {
		stopCtx := s.cron.Stop()
		<-stopCtx.Done()
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("所有任务已完成")
	case <-time.After(30 * time.Second):
		log.Warn("等待任务完成超时（30s），强制退出")
	}

	s.running = false
}

// IsRunning 是否正在运行
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// TriggerUpdate 手动触发更新（供 API 调用）
func (s *Scheduler) TriggerUpdate(ctx context.Context) error {
	if !s.IsRunning() {
		return fmt.Errorf("调度器未运行")
	}
	s.wg.Add(1)
	defer s.wg.Done()
	return s.checkAndUpdate(ctx)
}

// checkAndUpdate 核心业务逻辑：遍历所有启用的域名记录，检测并更新 IP
func (s *Scheduler) checkAndUpdate(ctx context.Context) error {
	records, err := s.domainRepo.ListEnabled(ctx)
	if err != nil {
		log.Error("获取域名记录失败", "error", err)
		return err
	}

	if len(records) == 0 {
		log.Debug("没有已启用的域名记录，跳过")
		return nil
	}

	log.Debug("开始检测 DNS 记录", "count", len(records))

	var wg sync.WaitGroup
	errCh := make(chan error, len(records))

	for _, rec := range records {
		wg.Add(1)
		go func(r models.DomainRecord) {
			defer wg.Done()
			if err := s.updateSingleRecord(ctx, r); err != nil {
				log.Error("更新记录失败", "domain", r.Domain, "rr", r.RR, "error", err)
				errCh <- err
			}
		}(rec)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d 条记录更新失败", len(errs))
	}

	return nil
}

// updateSingleRecord 更新单条 DNS 记录
func (s *Scheduler) updateSingleRecord(ctx context.Context, rec models.DomainRecord) error {
	key := fmt.Sprintf("%s/%s/%s", rec.Domain, rec.RR, rec.Type)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var currentIP string
	var err error

	switch rec.Type {
	case "AAAA":
		currentIP, err = s.detector.GetCurrentIPv6(ctx)
	case "A":
		currentIP, err = s.detector.GetCurrentIPv4(ctx)
	default:
		return fmt.Errorf("[%s] 不支持的记录类型: %s", key, rec.Type)
	}

	if err != nil {
		s.logHistory(ctx, rec, "", "failed", fmt.Sprintf("获取公网 IP 失败: %v", err))
		return fmt.Errorf("[%s] 获取公网 IP 失败: %w", key, err)
	}

	if !ipdetect.IsIPChanged(rec.Value, currentIP) {
		return nil
	}

	oldIP := rec.Value
	log.Info("IP 已变化", "key", key, "old", oldIP, "new", currentIP)

	record, err := s.dnsSvc.FindRecord(ctx, rec.Domain, rec.RR, rec.Type)
	if err != nil {
		s.logHistory(ctx, rec, currentIP, "failed", fmt.Sprintf("查询已有记录失败: %v", err))
		return fmt.Errorf("[%s] 查询已有记录失败: %w", key, err)
	}

	var recordID string
	if record != nil {
		if record.Value == currentIP {
			log.Debug("DNS 记录未变化，跳过更新", "key", key, "ip", currentIP)
			if rec.Value != currentIP {
				_ = s.domainRepo.UpdateValue(ctx, rec.ID, record.RecordID, currentIP)
			}
			return nil
		}
		if err := s.dnsSvc.UpdateRecord(ctx, record.RecordID, rec.RR, rec.Type, currentIP, rec.TTL); err != nil {
			s.logHistory(ctx, rec, currentIP, "failed", fmt.Sprintf("更新 DNS 记录失败: %v", err))
			return fmt.Errorf("[%s] 更新 DNS 记录失败: %w", key, err)
		}
		recordID = record.RecordID
		log.Info("DNS 记录已更新", "key", key, "recordID", recordID, "ip", currentIP)
	} else {
		rid, err := s.dnsSvc.AddRecord(ctx, rec.Domain, rec.RR, rec.Type, currentIP, rec.TTL)
		if err != nil {
			s.logHistory(ctx, rec, currentIP, "failed", fmt.Sprintf("添加 DNS 记录失败: %v", err))
			return fmt.Errorf("[%s] 添加 DNS 记录失败: %w", key, err)
		}
		recordID = rid
		log.Info("DNS 记录已创建", "key", key, "recordID", recordID, "ip", currentIP)
	}

	rec.Value = currentIP
	rec.RecordID = recordID
	if err := s.domainRepo.UpdateValue(ctx, rec.ID, recordID, currentIP); err != nil {
		log.Warn("更新缓存失败", "key", key, "error", err)
	}

	s.logHistory(ctx, rec, currentIP, "success", "")
	s.notifier.SendNotification(ctx, rec.Domain, rec.RR, rec.Type, oldIP, currentIP)

	return nil
}

// logHistory 记录更新历史
func (s *Scheduler) logHistory(ctx context.Context, rec models.DomainRecord, newIP, status, errMsg string) {
	history := &models.HistoryRecord{
		Domain:   rec.Domain,
		RR:       rec.RR,
		OldIP:    rec.Value,
		NewIP:    newIP,
		Type:     rec.Type,
		Status:   status,
		ErrorMsg: errMsg,
	}

	if _, err := s.historyRepo.Create(ctx, history); err != nil {
		log.Warn("记录历史失败", "error", err)
	}
}

// GetStatus 返回调度器状态
func (s *Scheduler) GetStatus(ctx context.Context) (map[string]interface{}, error) {
	records, err := s.domainRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	entryCount := len(s.entryIDs)
	s.mu.RUnlock()

	return map[string]interface{}{
		"running":      s.running,
		"interval":     s.interval,
		"total":        len(records),
		"notifiers":    s.notifier.Count(),
		"records":      records,
		"cron_entries": entryCount,
	}, nil
}
