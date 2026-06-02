package crypto

import (
        "context"
        "fmt"

        "github.com/Yunqingqingxi/yunxi-home/internal/database"
        "github.com/Yunqingqingxi/yunxi-home/internal/logger"
)

var log = logger.ForComponent("crypto")

type EncryptedConfigRepo struct {
        inner database.ConfigRepository
        key   []byte
}

func NewEncryptedConfigRepo(inner database.ConfigRepository, key []byte) *EncryptedConfigRepo {
        return &EncryptedConfigRepo{inner: inner, key: key}
}

func (r *EncryptedConfigRepo) GetSection(ctx context.Context, section string) (string, error) {
        data, err := r.inner.GetSection(ctx, section)
        if err != nil || data == "" {
                return data, err
        }
        decrypted, err := Decrypt(data, r.key)
        if err != nil {
                log.Debug("配置段未加密，作为明文处理", logger.KeyEvent, logger.EventConfig, "section", section)
                return data, nil
        }
        return decrypted, nil
}

func (r *EncryptedConfigRepo) GetAll(ctx context.Context) (map[string]string, error) {
        raw, err := r.inner.GetAll(ctx)
        if err != nil {
                return nil, err
        }
        result := make(map[string]string, len(raw))
        for section, data := range raw {
                if data == "" {
                        result[section] = ""
                        continue
                }
                decrypted, err := Decrypt(data, r.key)
                if err != nil {
                        result[section] = data
                } else {
                        result[section] = decrypted
                }
        }
        return result, nil
}

func (r *EncryptedConfigRepo) SetSection(ctx context.Context, section, data string) error {
        encrypted, err := Encrypt(data, r.key)
        if err != nil {
                return fmt.Errorf("encrypt config section %s: %w", section, err)
        }
        return r.inner.SetSection(ctx, section, encrypted)
}

func (r *EncryptedConfigRepo) InitDefaults(ctx context.Context, defaults map[string]string) error {
        encrypted := make(map[string]string, len(defaults))
        for section, data := range defaults {
                encData, err := Encrypt(data, r.key)
                if err != nil {
                        return fmt.Errorf("encrypt default section %s: %w", section, err)
                }
                encrypted[section] = encData
        }
        return r.inner.InitDefaults(ctx, encrypted)
}

var _ database.ConfigRepository = (*EncryptedConfigRepo)(nil)