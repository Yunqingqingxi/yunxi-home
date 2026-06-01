package nas

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

// HashPassword 哈希密码
func HashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}

// VerifyPassword 验证密码
func VerifyPassword(password, hash string) bool {
	return HashPassword(password) == hash
}

// ValidatePath 验证分享路径是否存在且可访问
func (s *ShareService) ValidatePath(filePath string) error {
	if !s.fs.Exists(filePath) {
		return fmt.Errorf("文件或目录不存在")
	}
	return nil
}
