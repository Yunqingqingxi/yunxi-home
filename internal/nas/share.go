package nas

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// ShareService 分享服务
type ShareService struct {
	fs FileService
}

// NewShareService 创建分享服务
func NewShareService(fs FileService) *ShareService {
	return &ShareService{fs: fs}
}

// GenerateToken 生成随机 token
func GenerateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashPassword 哈希密码（bcrypt）
func HashPassword(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		// 回退到 SHA-256（不应发生）
		h := sha256.Sum256([]byte(password))
		return hex.EncodeToString(h[:])
	}
	return string(hash)
}

// VerifyPassword 验证密码（bcrypt 优先，回退 SHA-256 兼容旧数据）
func VerifyPassword(password, hash string) bool {
	// bcrypt hash（$2a$/$2b$ 开头）
	if strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$") {
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	}
	// 旧 SHA-256 hash：兼容验证
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:]) == hash
}

// ValidatePath 验证分享路径是否存在且可访问
func (s *ShareService) ValidatePath(filePath string) error {
	if !s.fs.Exists(filePath) {
		return fmt.Errorf("文件或目录不存在")
	}
	return nil
}
