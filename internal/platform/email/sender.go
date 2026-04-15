package email

import (
	"fmt"

	"cixing/internal/config"
)

// 工厂函数
// 构建 Sender 实现，目前仅支持 SMTP，未来可根据 config 构建不同类型的 Sender
func NewSender(cfg config.EmailConfig) (Sender, error) {
	// 转 config.EmailConfig 到 SMTPConfig，创建 SMTPSender
	sender, err := NewSMTPSender(SMTPConfig{
		Addr:     cfg.Addr, // 决定使用 Mailpit 或 阿里云 SMTP
		Username: cfg.Username,
		Password: cfg.Password,
		From:     cfg.From,
		ReplyTo:  cfg.ReplyTo,
		UseTLS:   cfg.UseTLS,
	})
	if err != nil {
		return nil, fmt.Errorf("build smtp sender: %w", err)
	}
	return sender, nil
}
