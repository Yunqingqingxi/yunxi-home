// Package crypto 统一加密模块入口。
//
// 消费者只需导入此包即可使用加密功能。
//
//	import "github.com/yxd/yunxi-home/internal/crypto"
//	c := crypto.New()
//	encrypted, _ := c.Encrypt("secret", key)
package crypto

import "github.com/yxd/yunxi-home/internal/crypto/base"

// Cipher 对称加密接口
type Cipher = base.Cipher
