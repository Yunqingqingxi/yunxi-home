// Package base 定义加密模块的通用接口，零外部依赖。
package base

// Cipher 对称加密接口，支持 AES-GCM 等实现。
type Cipher interface {
	Encrypt(plaintext string, key []byte) (string, error)
	Decrypt(encoded string, key []byte) (string, error)
	GenerateKey() (string, error)
}
