package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Sender 接口，当前仅有发送验证码的方法
type Sender interface {
	SendAuthCode(ctx context.Context, toEmail string, code string) error
}

type SMTPConfig struct {
	Addr     string
	Username string
	Password string
	From     string
	ReplyTo  string
	UseTLS   bool
}

type SMTPSender struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	ReplyTo  string
	UseTLS   bool
}

func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	// 拆 Addr
	host, port, err := splitSMTPAddr(cfg.Addr)
	if err != nil {
		return nil, err
	}
	// 构造 SMTPSender
	return &SMTPSender{
		Host:     host,
		Port:     port,
		Username: cfg.Username,
		Password: cfg.Password,
		From:     strings.TrimSpace(cfg.From),
		ReplyTo:  strings.TrimSpace(cfg.ReplyTo),
		UseTLS:   cfg.UseTLS,
	}, nil
}

func (s *SMTPSender) SendAuthCode(ctx context.Context, toEmail string, code string) error {
	// 发件人为空，error
	if strings.TrimSpace(s.From) == "" {
		return errors.New("smtp sender missing from address")
	}
	// 清洗 header 值，去掉首尾空格、拒绝空值、拒绝 \r \n
	from, err := sanitizeHeaderValue("from", s.From)
	if err != nil {
		return err
	}
	replyTo, err := sanitizeOptionalHeaderValue("reply_to", s.ReplyTo)
	if err != nil {
		return err
	}
	toEmail, err = sanitizeHeaderValue("to", toEmail)
	if err != nil {
		return err
	}
	// 生成邮件主题和正文
	subject := "Cixing verification code"
	body := fmt.Sprintf("Your 6-digit verification code is: %s\nThis code will expire soon.", code)
	// 构建原始邮件内容
	msg, err := buildMessage(from, replyTo, toEmail, subject, body)
	if err != nil {
		return err
	}
	// 发送
	return s.send(ctx, from, toEmail, msg)
}

func (s *SMTPSender) send(ctx context.Context, from, toEmail string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	// 建立 TCP 连接，连接 SMTP 服务器
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp: dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// TLS 模式时，做 TLS 握手
	if s.UseTLS {
		// 把连接包成 TLS 连接
		tlsConn := tls.Client(conn, &tls.Config{ServerName: s.Host})
		// 做 TLS 握手
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("smtp: tls handshake: %w", err)
		}
		conn = tlsConn
	}

	// 如果 ctx 被取消，主动关闭连接
	stopOnCancel := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopOnCancel()
	// 如果有 deadline，设置 deadline
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	// 把已建立的 conn 包装成 SMTP 协议客户端对象
	client, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		return fmt.Errorf("smtp: new client: %w", err)
	}
	defer client.Close()

	// 不使用 TLS 时，如果服支持 STARTTLS，尝试 STARTTLS
	if !s.UseTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: s.Host}); err != nil {
				return fmt.Errorf("smtp: starttls: %w", err)
			}
		}
	}

	// 如果配置了用户名，尝试登录 SMTP 服务器
	if s.Username != "" {
		auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp: auth: %w", err)
			}
		}
	}

	// SMTP 标准流程
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp: mail from: %w", err)
	}
	if err := client.Rcpt(toEmail); err != nil {
		return fmt.Errorf("smtp: rcpt to: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp: data: %w", err)
	}
	if _, err := writer.Write(msg); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp: write message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp: close data writer: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp: quit: %w", err)
	}
	return nil
}

// 构建邮件内容，header 清洗
func buildMessage(from, replyTo, to, subject, body string) ([]byte, error) {
	var err error
	from, err = sanitizeHeaderValue("from", from)
	if err != nil {
		return nil, err
	}
	replyTo, err = sanitizeOptionalHeaderValue("reply_to", replyTo)
	if err != nil {
		return nil, err
	}
	to, err = sanitizeHeaderValue("to", to)
	if err != nil {
		return nil, err
	}
	subject, err = sanitizeHeaderValue("subject", subject)
	if err != nil {
		return nil, err
	}
	headers := []string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		fmt.Sprintf("Date: %s", time.Now().Format(time.RFC1123Z)),
		"Content-Type: text/plain; charset=UTF-8",
	}
	if strings.TrimSpace(replyTo) != "" {
		headers = append(headers, fmt.Sprintf("Reply-To: %s", replyTo))
	}
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body + "\r\n"), nil
}

// 把 Addr 拆成 host + port 的 helper
func splitSMTPAddr(addr string) (string, int, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", 0, errors.New("smtp addr is empty")
	}
	if strings.Contains(addr, "://") {
		u, err := url.Parse(addr)
		if err != nil {
			return "", 0, err
		}
		if u.Host == "" {
			return "", 0, errors.New("smtp host is empty")
		}
		addr = u.Host
	}

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, err
	}
	if strings.TrimSpace(host) == "" || port <= 0 {
		return "", 0, errors.New("smtp addr is invalid")
	}
	return host, port, nil
}

// 清洗 header 值，去掉首尾空格、拒绝空值、拒绝 \r \n
func sanitizeHeaderValue(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("smtp: %s is empty", field)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("smtp: invalid %s header value", field)
	}
	return value, nil
}

func sanitizeOptionalHeaderValue(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	return sanitizeHeaderValue(field, value)
}
