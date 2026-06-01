package database

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/yxd/yunxi-home/internal/models"
	"github.com/yxd/yunxi-home/internal/nas"
)

// ── Shared file-backed ID generator ────────────────────────────────

var fileIDMu sync.Mutex

func nextID(store *FileStore, counterFile string) int64 {
	fileIDMu.Lock()
	defer fileIDMu.Unlock()
	var id int64
	_ = store.Load(counterFile, &id)
	id++
	_ = store.Save(counterFile, id)
	return id
}

// ── FileDomainRepo ─────────────────────────────────────────────────

type fileDomains struct {
	Records []models.DomainRecord `json:"records"`
}

type FileDomainRepo struct {
	store *FileStore
}

func NewFileDomainRepo(store *FileStore) *FileDomainRepo {
	store.EnsureFile("domains.json", `{"records":[]}`)
	return &FileDomainRepo{store: store}
}

func (r *FileDomainRepo) load() (*fileDomains, error) {
	var d fileDomains
	if err := r.store.Load("domains.json", &d); err != nil {
		return &fileDomains{}, nil
	}
	return &d, nil
}

func (r *FileDomainRepo) save(d *fileDomains) error { return r.store.Save("domains.json", d) }

func (r *FileDomainRepo) Create(ctx context.Context, rec *models.DomainRecord) (int64, error) {
	d, _ := r.load()
	rec.ID = nextID(r.store, "domains_id.seq")
	now := time.Now()
	rec.CreatedAt = now
	rec.UpdatedAt = now
	d.Records = append(d.Records, *rec)
	return rec.ID, r.save(d)
}

func (r *FileDomainRepo) GetByID(ctx context.Context, id int64) (*models.DomainRecord, error) {
	d, _ := r.load()
	for i := range d.Records {
		if d.Records[i].ID == id {
			rec := d.Records[i]
			return &rec, nil
		}
	}
	return nil, os.ErrNotExist
}

func (r *FileDomainRepo) GetByDomain(ctx context.Context, domain, rr, recType string) (*models.DomainRecord, error) {
	d, _ := r.load()
	for i := range d.Records {
		rec := &d.Records[i]
		if rec.Domain == domain && rec.RR == rr && rec.Type == recType {
			r := *rec
			return &r, nil
		}
	}
	return nil, os.ErrNotExist
}

func (r *FileDomainRepo) List(ctx context.Context) ([]models.DomainRecord, error) {
	d, _ := r.load()
	if d.Records == nil {
		return []models.DomainRecord{}, nil
	}
	return d.Records, nil
}

func (r *FileDomainRepo) ListEnabled(ctx context.Context) ([]models.DomainRecord, error) {
	d, _ := r.load()
	var out []models.DomainRecord
	for _, rec := range d.Records {
		if rec.Enabled {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (r *FileDomainRepo) Update(ctx context.Context, rec *models.DomainRecord) error {
	d, _ := r.load()
	for i := range d.Records {
		if d.Records[i].ID == rec.ID {
			rec.UpdatedAt = time.Now()
			d.Records[i] = *rec
			return r.save(d)
		}
	}
	return os.ErrNotExist
}

func (r *FileDomainRepo) UpdateValue(ctx context.Context, id int64, recordID, value string) error {
	d, _ := r.load()
	for i := range d.Records {
		if d.Records[i].ID == id {
			d.Records[i].RecordID = recordID
			d.Records[i].Value = value
			d.Records[i].UpdatedAt = time.Now()
			return r.save(d)
		}
	}
	return os.ErrNotExist
}

func (r *FileDomainRepo) Delete(ctx context.Context, id int64) error {
	d, _ := r.load()
	for i := range d.Records {
		if d.Records[i].ID == id {
			d.Records = append(d.Records[:i], d.Records[i+1:]...)
			return r.save(d)
		}
	}
	return os.ErrNotExist
}

func (r *FileDomainRepo) Upsert(ctx context.Context, rec *models.DomainRecord) error {
	d, _ := r.load()
	for i := range d.Records {
		if d.Records[i].Domain == rec.Domain && d.Records[i].RR == rec.RR && d.Records[i].Type == rec.Type {
			rec.ID = d.Records[i].ID
			rec.CreatedAt = d.Records[i].CreatedAt
			rec.UpdatedAt = time.Now()
			d.Records[i] = *rec
			return r.save(d)
		}
	}
	rec.ID = nextID(r.store, "domains_id.seq")
	now := time.Now()
	rec.CreatedAt = now
	rec.UpdatedAt = now
	d.Records = append(d.Records, *rec)
	return r.save(d)
}

// ── FileHistoryRepo ────────────────────────────────────────────────

type fileHistories struct {
	Records []models.HistoryRecord `json:"records"`
}

type FileHistoryRepo struct {
	store *FileStore
}

func NewFileHistoryRepo(store *FileStore) *FileHistoryRepo {
	store.EnsureFile("histories.json", `{"records":[]}`)
	return &FileHistoryRepo{store: store}
}

func (r *FileHistoryRepo) load() (*fileHistories, error) {
	var h fileHistories
	if err := r.store.Load("histories.json", &h); err != nil {
		return &fileHistories{}, nil
	}
	return &h, nil
}

func (r *FileHistoryRepo) save(h *fileHistories) error { return r.store.Save("histories.json", h) }

func (r *FileHistoryRepo) Create(ctx context.Context, rec *models.HistoryRecord) (int64, error) {
	h, _ := r.load()
	rec.ID = nextID(r.store, "histories_id.seq")
	rec.CreatedAt = time.Now()
	h.Records = append(h.Records, *rec)
	return rec.ID, r.save(h)
}

func (r *FileHistoryRepo) GetByID(ctx context.Context, id int64) (*models.HistoryRecord, error) {
	h, _ := r.load()
	for i := range h.Records {
		if h.Records[i].ID == id {
			rec := h.Records[i]
			return &rec, nil
		}
	}
	return nil, os.ErrNotExist
}

func (r *FileHistoryRepo) List(ctx context.Context, params ListParams) (*ListResult, error) {
	h, _ := r.load()
	var filtered []models.HistoryRecord
	for _, rec := range h.Records {
		if params.Domain != "" && rec.Domain != params.Domain {
			continue
		}
		filtered = append(filtered, rec)
	}
	total := int64(len(filtered))
	// Simple pagination
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Size < 1 || params.Size > 100 {
		params.Size = 20
	}
	offset := (params.Page - 1) * params.Size
	end := offset + params.Size
	if offset > len(filtered) {
		return &ListResult{Records: []models.HistoryRecord{}, Total: total, Page: params.Page, Size: params.Size}, nil
	}
	if end > len(filtered) {
		end = len(filtered)
	}
	return &ListResult{Records: filtered[offset:end], Total: total, Page: params.Page, Size: params.Size}, nil
}

func (r *FileHistoryRepo) GetStats(ctx context.Context, days int) ([]HistoryStats, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	h, _ := r.load()
	dayMap := make(map[string]*HistoryStats)
	for _, rec := range h.Records {
		if rec.CreatedAt.Before(cutoff) {
			continue
		}
		date := rec.CreatedAt.Format("2006-01-02")
		s, ok := dayMap[date]
		if !ok {
			s = &HistoryStats{Date: date}
			dayMap[date] = s
		}
		s.Total++
		if rec.Status == "success" {
			s.Success++
		} else {
			s.Failed++
		}
	}
	var stats []HistoryStats
	for _, s := range dayMap {
		stats = append(stats, *s)
	}
	if stats == nil {
		stats = []HistoryStats{}
	}
	return stats, nil
}

func (r *FileHistoryRepo) CleanOld(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	h, _ := r.load()
	var kept []models.HistoryRecord
	var removed int64
	for _, rec := range h.Records {
		if rec.CreatedAt.Before(cutoff) {
			removed++
		} else {
			kept = append(kept, rec)
		}
	}
	h.Records = kept
	return removed, r.save(h)
}

// ── FileUserRepo ───────────────────────────────────────────────────

type fileUsers struct {
	Records []models.User `json:"records"`
}

type FileUserRepo struct {
	store *FileStore
}

func NewFileUserRepo(store *FileStore) *FileUserRepo {
	store.EnsureFile("users.json", `{"records":[]}`)
	return &FileUserRepo{store: store}
}

func (r *FileUserRepo) load() (*fileUsers, error) {
	var u fileUsers
	if err := r.store.Load("users.json", &u); err != nil {
		return &fileUsers{}, nil
	}
	return &u, nil
}

func (r *FileUserRepo) save(u *fileUsers) error { return r.store.Save("users.json", u) }

func (r *FileUserRepo) Create(ctx context.Context, user *models.User) (int64, error) {
	u, _ := r.load()
	user.ID = nextID(r.store, "users_id.seq")
	user.CreatedAt = time.Now()
	u.Records = append(u.Records, *user)
	return user.ID, r.save(u)
}

func (r *FileUserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	u, _ := r.load()
	for i := range u.Records {
		if u.Records[i].Username == username {
			rec := u.Records[i]
			return &rec, nil
		}
	}
	return nil, os.ErrNotExist
}

func (r *FileUserRepo) GetByID(ctx context.Context, id int64) (*models.User, error) {
	u, _ := r.load()
	for i := range u.Records {
		if u.Records[i].ID == id {
			rec := u.Records[i]
			return &rec, nil
		}
	}
	return nil, os.ErrNotExist
}

func (r *FileUserRepo) List(ctx context.Context) ([]models.User, error) {
	u, _ := r.load()
	if u.Records == nil {
		return []models.User{}, nil
	}
	return u.Records, nil
}

func (r *FileUserRepo) UpdatePassword(ctx context.Context, id int64, hash string) error {
	u, _ := r.load()
	for i := range u.Records {
		if u.Records[i].ID == id {
			u.Records[i].PasswordHash = hash
			return r.save(u)
		}
	}
	return os.ErrNotExist
}

func (r *FileUserRepo) Delete(ctx context.Context, id int64) error {
	u, _ := r.load()
	for i := range u.Records {
		if u.Records[i].ID == id {
			u.Records = append(u.Records[:i], u.Records[i+1:]...)
			return r.save(u)
		}
	}
	return os.ErrNotExist
}

func (r *FileUserRepo) UpdateRole(ctx context.Context, id int64, role string) error {
	u, _ := r.load()
	for i := range u.Records {
		if u.Records[i].ID == id {
			u.Records[i].Role = models.UserRole(role)
			return r.save(u)
		}
	}
	return os.ErrNotExist
}

func (r *FileUserRepo) UpdateQuota(ctx context.Context, id int64, quota int64) error {
	u, _ := r.load()
	for i := range u.Records {
		if u.Records[i].ID == id {
			u.Records[i].StorageQuota = quota
			return r.save(u)
		}
	}
	return os.ErrNotExist
}

func (r *FileUserRepo) AddStorageUsed(ctx context.Context, id int64, delta int64) error {
	u, _ := r.load()
	for i := range u.Records {
		if u.Records[i].ID == id {
			u.Records[i].StorageUsed += delta
			return r.save(u)
		}
	}
	return os.ErrNotExist
}

func (r *FileUserRepo) InitDefaultAdmin(ctx context.Context, username, password string) error {
	u, _ := r.load()
	if len(u.Records) > 0 {
		return nil
	}
	// Factory handles password hashing in fileInitDefaultAdmin
	return nil
}

// ── FileChatRepo ───────────────────────────────────────────────────

type fileChatSessions struct {
	Sessions []models.ChatSession `json:"sessions"`
}

type FileChatRepo struct {
	store *FileStore
}

func NewFileChatRepo(store *FileStore) *FileChatRepo {
	store.EnsureFile("chat_sessions.json", `{"sessions":[]}`)
	return &FileChatRepo{store: store}
}

func (r *FileChatRepo) load() (*fileChatSessions, error) {
	var c fileChatSessions
	if err := r.store.Load("chat_sessions.json", &c); err != nil {
		return &fileChatSessions{}, nil
	}
	return &c, nil
}

func (r *FileChatRepo) save(c *fileChatSessions) error { return r.store.Save("chat_sessions.json", c) }

func (r *FileChatRepo) List(ctx context.Context) ([]models.ChatSession, error) {
	c, _ := r.load()
	if c.Sessions == nil {
		return []models.ChatSession{}, nil
	}
	return c.Sessions, nil
}

func (r *FileChatRepo) ListByType(ctx context.Context, st string) ([]models.ChatSession, error) {
	c, _ := r.load()
	var out []models.ChatSession
	for _, s := range c.Sessions {
		if s.Type == st {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *FileChatRepo) Upsert(ctx context.Context, s *models.ChatSession) error {
	c, _ := r.load()
	now := time.Now()
	s.UpdatedAt = now
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	if s.Type == "" {
		s.Type = "chat"
	}
	for i := range c.Sessions {
		if c.Sessions[i].ID == s.ID {
			c.Sessions[i] = *s
			return r.save(c)
		}
	}
	c.Sessions = append(c.Sessions, *s)
	return r.save(c)
}

func (r *FileChatRepo) Delete(ctx context.Context, id string) error {
	c, _ := r.load()
	for i := range c.Sessions {
		if c.Sessions[i].ID == id {
			c.Sessions = append(c.Sessions[:i], c.Sessions[i+1:]...)
			return r.save(c)
		}
	}
	return nil
}

func (r *FileChatRepo) DeleteByType(ctx context.Context, st string) (int64, error) {
	c, _ := r.load()
	var kept []models.ChatSession
	var removed int64
	for _, s := range c.Sessions {
		if s.Type == st {
			removed++
		} else {
			kept = append(kept, s)
		}
	}
	c.Sessions = kept
	return removed, r.save(c)
}

func (r *FileChatRepo) DeleteStale(ctx context.Context, st string, olderThan time.Duration) (int64, error) {
	c, _ := r.load()
	cutoff := time.Now().Add(-olderThan)
	var kept []models.ChatSession
	var removed int64
	for _, s := range c.Sessions {
		if s.Type == st && s.UpdatedAt.Before(cutoff) {
			removed++
		} else {
			kept = append(kept, s)
		}
	}
	c.Sessions = kept
	return removed, r.save(c)
}

func (r *FileChatRepo) DeleteAll(ctx context.Context) error {
	return r.save(&fileChatSessions{Sessions: []models.ChatSession{}})
}

// ── FileConfigRepo ─────────────────────────────────────────────────

type FileConfigRepo struct {
	store *FileStore
}

func NewFileConfigRepo(store *FileStore) *FileConfigRepo {
	store.EnsureFile("config.json", `{}`)
	return &FileConfigRepo{store: store}
}

func (r *FileConfigRepo) GetAll(ctx context.Context) (map[string]string, error) {
	var raw map[string]json.RawMessage
	if err := r.store.Load("config.json", &raw); err != nil {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = string(v)
	}
	return out, nil
}

func (r *FileConfigRepo) GetSection(ctx context.Context, section string) (string, error) {
	var raw map[string]json.RawMessage
	if err := r.store.Load("config.json", &raw); err != nil {
		return "", nil
	}
	data, ok := raw[section]
	if !ok {
		return "", nil
	}
	return string(data), nil
}

func (r *FileConfigRepo) SetSection(ctx context.Context, section, data string) error {
	var raw map[string]json.RawMessage
	_ = r.store.Load("config.json", &raw)
	if raw == nil {
		raw = make(map[string]json.RawMessage)
	}
	raw[section] = json.RawMessage(data)
	return r.store.Save("config.json", raw)
}

func (r *FileConfigRepo) InitDefaults(ctx context.Context, defaults map[string]string) error {
	var raw map[string]json.RawMessage
	_ = r.store.Load("config.json", &raw)
	if len(raw) > 0 {
		return nil
	}
	if raw == nil {
		raw = make(map[string]json.RawMessage)
	}
	for k, v := range defaults {
		raw[k] = json.RawMessage(v)
	}
	return r.store.Save("config.json", raw)
}

// ── FileFilePermRepo ───────────────────────────────────────────────

type filePerms struct {
	Perms []models.FilePermission `json:"perms"`
}

type FileFilePermRepo struct {
	store *FileStore
}

func NewFileFilePermRepo(store *FileStore) *FileFilePermRepo {
	store.EnsureFile("file_permissions.json", `{"perms":[]}`)
	return &FileFilePermRepo{store: store}
}

func (r *FileFilePermRepo) load() (*filePerms, error) {
	var p filePerms
	if err := r.store.Load("file_permissions.json", &p); err != nil {
		return &filePerms{}, nil
	}
	return &p, nil
}

func (r *FileFilePermRepo) save(p *filePerms) error { return r.store.Save("file_permissions.json", p) }

func (r *FileFilePermRepo) GetByUserAndPath(ctx context.Context, userID int64, filePath string) (*models.FilePermission, error) {
	p, _ := r.load()
	var best *models.FilePermission
	bestLen := 0
	for i := range p.Perms {
		perm := &p.Perms[i]
		if perm.UserID == userID && len(filePath) >= len(perm.Path) && filePath[:len(perm.Path)] == perm.Path {
			if len(perm.Path) > bestLen {
				bestLen = len(perm.Path)
				best = perm
			}
		}
	}
	return best, nil
}

func (r *FileFilePermRepo) ListByUser(ctx context.Context, userID int64) ([]models.FilePermission, error) {
	p, _ := r.load()
	var out []models.FilePermission
	for _, perm := range p.Perms {
		if perm.UserID == userID {
			out = append(out, perm)
		}
	}
	return out, nil
}

func (r *FileFilePermRepo) ListAll(ctx context.Context) ([]models.FilePermission, error) {
	p, _ := r.load()
	if p.Perms == nil {
		return []models.FilePermission{}, nil
	}
	return p.Perms, nil
}

func (r *FileFilePermRepo) Upsert(ctx context.Context, perm *models.FilePermission) error {
	perm.CreatedAt = time.Now()
	perm.UpdatedAt = time.Now()
	p, _ := r.load()
	// update existing or append
	for i := range p.Perms {
		if p.Perms[i].UserID == perm.UserID && p.Perms[i].Path == perm.Path {
			perm.ID = p.Perms[i].ID
			perm.CreatedAt = p.Perms[i].CreatedAt
			p.Perms[i] = *perm
			return r.save(p)
		}
	}
	perm.ID = nextID(r.store, "file_permissions_id.seq")
	p.Perms = append(p.Perms, *perm)
	return r.save(p)
}

func (r *FileFilePermRepo) Delete(ctx context.Context, id int64) error {
	p, _ := r.load()
	for i := range p.Perms {
		if p.Perms[i].ID == id {
			p.Perms = append(p.Perms[:i], p.Perms[i+1:]...)
			return r.save(p)
		}
	}
	return nil
}

// ── FileShareRepo ──────────────────────────────────────────────────

type fileShares struct {
	Shares []nas.Share `json:"shares"`
}

type FileShareRepo struct {
	store *FileStore
}

func NewFileShareRepo(store *FileStore) *FileShareRepo {
	store.EnsureFile("shares.json", `{"shares":[]}`)
	return &FileShareRepo{store: store}
}

func (r *FileShareRepo) load() (*fileShares, error) {
	var s fileShares
	if err := r.store.Load("shares.json", &s); err != nil {
		return &fileShares{}, nil
	}
	return &s, nil
}

func (r *FileShareRepo) save(s *fileShares) error { return r.store.Save("shares.json", s) }

func (r *FileShareRepo) Create(ctx context.Context, share *nas.Share) (int64, error) {
	s, _ := r.load()
	share.ID = nextID(r.store, "shares_id.seq")
	share.CreatedAt = time.Now()
	s.Shares = append(s.Shares, *share)
	return share.ID, r.save(s)
}

func (r *FileShareRepo) GetByToken(ctx context.Context, token string) (*nas.Share, error) {
	s, _ := r.load()
	for i := range s.Shares {
		if s.Shares[i].Token == token {
			rec := s.Shares[i]
			return &rec, nil
		}
	}
	return nil, nil
}

func (r *FileShareRepo) List(ctx context.Context, limit, offset int) ([]nas.Share, int64, error) {
	s, _ := r.load()
	total := int64(len(s.Shares))
	if offset >= len(s.Shares) {
		return []nas.Share{}, total, nil
	}
	end := offset + limit
	if end > len(s.Shares) {
		end = len(s.Shares)
	}
	return s.Shares[offset:end], total, nil
}

func (r *FileShareRepo) Delete(ctx context.Context, id int64) error {
	s, _ := r.load()
	for i := range s.Shares {
		if s.Shares[i].ID == id {
			s.Shares = append(s.Shares[:i], s.Shares[i+1:]...)
			return r.save(s)
		}
	}
	return nil
}

func (r *FileShareRepo) IncrementDownloads(ctx context.Context, id int64) error {
	s, _ := r.load()
	for i := range s.Shares {
		if s.Shares[i].ID == id {
			s.Shares[i].Downloads++
			return r.save(s)
		}
	}
	return nil
}

func (r *FileShareRepo) CleanExpired(ctx context.Context) (int64, error) {
	now := time.Now()
	s, _ := r.load()
	var kept []nas.Share
	var removed int64
	for _, sh := range s.Shares {
		if !sh.ExpiresAt.IsZero() && sh.ExpiresAt.Before(now) {
			removed++
		} else {
			kept = append(kept, sh)
		}
	}
	s.Shares = kept
	return removed, r.save(s)
}

// ── File goal/todo repos ──────────────────────────────────────────

type fileGoalRepo struct {
	store *FileStore
	mu    sync.Mutex
}

func (r *fileGoalRepo) load() (map[string]string, error) {
	var m map[string]string
	if err := r.store.Load("session_goals.json", &m); err != nil {
		return make(map[string]string), nil
	}
	if m == nil {
		m = make(map[string]string)
	}
	return m, nil
}

func (r *fileGoalRepo) Upsert(ctx context.Context, sessionID, goalsJSON string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, _ := r.load()
	m[sessionID] = goalsJSON
	return r.store.Save("session_goals.json", m)
}

func (r *fileGoalRepo) Get(ctx context.Context, sessionID string) (string, error) {
	m, _ := r.load()
	return m[sessionID], nil
}

func (r *fileGoalRepo) Delete(ctx context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, _ := r.load()
	delete(m, sessionID)
	return r.store.Save("session_goals.json", m)
}

type fileTodoRepo struct {
	store *FileStore
	mu    sync.Mutex
}

func (r *fileTodoRepo) load() (map[string]string, error) {
	var m map[string]string
	if err := r.store.Load("session_todos.json", &m); err != nil {
		return make(map[string]string), nil
	}
	if m == nil {
		m = make(map[string]string)
	}
	return m, nil
}

func (r *fileTodoRepo) Upsert(ctx context.Context, sessionID, itemsJSON string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, _ := r.load()
	m[sessionID] = itemsJSON
	return r.store.Save("session_todos.json", m)
}

func (r *fileTodoRepo) Get(ctx context.Context, sessionID string) (string, error) {
	m, _ := r.load()
	return m[sessionID], nil
}

func (r *fileTodoRepo) Delete(ctx context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, _ := r.load()
	delete(m, sessionID)
	return r.store.Save("session_todos.json", m)
}
