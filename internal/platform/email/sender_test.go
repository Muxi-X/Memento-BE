package email

import (
	"strings"
	"testing"

	"cixing/internal/config"
)

func TestBuildMessageIncludesReplyTo(t *testing.T) {
	msg, err := buildMessage(
		"no-reply@mail.hello-lemon.cloud",
		"tasteofzzzlemon@outlook.com",
		"user@example.com",
		"Cixing verification code",
		"Your verification code is: 123456",
	)
	if err != nil {
		t.Fatalf("buildMessage() error = %v", err)
	}

	if !strings.Contains(string(msg), "Reply-To: tasteofzzzlemon@outlook.com\r\n") {
		t.Fatalf("message missing Reply-To header:\n%s", string(msg))
	}
}

func TestBuildMessageOmitsEmptyReplyTo(t *testing.T) {
	msg, err := buildMessage(
		"no-reply@mail.hello-lemon.cloud",
		"",
		"user@example.com",
		"Cixing verification code",
		"Your verification code is: 123456",
	)
	if err != nil {
		t.Fatalf("buildMessage() error = %v", err)
	}

	if strings.Contains(string(msg), "Reply-To:") {
		t.Fatalf("message unexpectedly contains Reply-To header:\n%s", string(msg))
	}
}

func TestBuildMessageRejectsHeaderInjection(t *testing.T) {
	_, err := buildMessage(
		"no-reply@mail.hello-lemon.cloud",
		"",
		"user@example.com\r\nBcc: attacker@example.com",
		"Cixing verification code",
		"Your verification code is: 123456",
	)
	if err == nil {
		t.Fatal("buildMessage() expected error for injected header value")
	}
}

func TestNewSenderBuildsSingleSMTPSender(t *testing.T) {
	sender, err := NewSender(configFixture())
	if err != nil {
		t.Fatalf("NewSender() error = %v", err)
	}

	smtpSender, ok := sender.(*SMTPSender)
	if !ok {
		t.Fatalf("sender type = %T, want *SMTPSender", sender)
	}
	if smtpSender.Host != "smtpdm.aliyun.com" {
		t.Fatalf("Host = %q, want smtpdm.aliyun.com", smtpSender.Host)
	}
	if smtpSender.Port != 465 {
		t.Fatalf("Port = %d, want 465", smtpSender.Port)
	}
	if smtpSender.From != "no-reply@mail.hello-lemon.cloud" {
		t.Fatalf("From = %q, want no-reply@mail.hello-lemon.cloud", smtpSender.From)
	}
	if smtpSender.ReplyTo != "tasteofzzzlemon@outlook.com" {
		t.Fatalf("ReplyTo = %q, want tasteofzzzlemon@outlook.com", smtpSender.ReplyTo)
	}
}

func configFixture() config.EmailConfig {
	return config.EmailConfig{
		Addr:     "smtpdm.aliyun.com:465",
		Username: "no-reply@mail.hello-lemon.cloud",
		Password: "secret",
		From:     "no-reply@mail.hello-lemon.cloud",
		ReplyTo:  "tasteofzzzlemon@outlook.com",
		UseTLS:   true,
	}
}
