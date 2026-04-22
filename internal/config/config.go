package config

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	Logging  LoggingConfig  `yaml:"logging"`
	HTTP     HTTPConfig     `yaml:"http"`
	Postgres PostgresConfig `yaml:"postgres"`
	Redis    RedisConfig    `yaml:"redis"`
	OSS      OSSConfig      `yaml:"oss"`
	JWT      JWTConfig      `yaml:"jwt"`
	Auth     AuthConfig     `yaml:"auth"`
	Email    EmailConfig    `yaml:"email"`
}

type AppConfig struct {
	Env string `yaml:"env"`
}

type LoggingConfig struct {
	LogLevel     string `yaml:"log_level"`
	LogAddSource bool   `yaml:"log_add_source"`
}

type HTTPConfig struct {
	Addr string `yaml:"addr"`
}

type PostgresConfig struct {
	DSN string `yaml:"dsn"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type OSSConfig struct {
	CredentialMode            string         `yaml:"credential_mode"`
	ECSRoleName               string         `yaml:"ecs_role_name"`
	DisableIMDSv1             bool           `yaml:"disable_imdsv1"`
	AssumeRoleARN             string         `yaml:"assume_role_arn"`
	AssumeRoleSessionName     string         `yaml:"assume_role_session_name"`
	AssumeRoleExternalID      string         `yaml:"assume_role_external_id"`
	AssumeRoleSTSEndpoint     string         `yaml:"assume_role_sts_endpoint"`
	AssumeRoleSessionDuration time.Duration  `yaml:"assume_role_session_duration"`
	Bucket                    string         `yaml:"bucket"`
	Region                    string         `yaml:"region"`
	AccessKeyID               string         `yaml:"access_key_id"`
	AccessKeySecret           string         `yaml:"access_key_secret"`
	PublicEndpoint            string         `yaml:"public_endpoint"`
	PublicEndpointIsCName     bool           `yaml:"public_endpoint_is_cname"`
	InternalEndpoint          string         `yaml:"internal_endpoint"`
	InternalEndpointIsCName   bool           `yaml:"internal_endpoint_is_cname"`
	UseInternalEndpoint       bool           `yaml:"use_internal_endpoint"`
	PutPresignExpire          time.Duration  `yaml:"put_presign_expire"`
	GetPresignExpire          time.Duration  `yaml:"get_presign_expire"`
	UploadPrefix              string         `yaml:"upload_prefix"`
	Styles                    OSSStyleConfig `yaml:"styles"`
}

type OSSStyleConfig struct {
	Card4x3     string `yaml:"card_4x3"`
	SquareSmall string `yaml:"square_small"`
	SquareMed   string `yaml:"square_medium"`
	DetailLarge string `yaml:"detail_large"`
}

type JWTConfig struct {
	PrivateKeyPEM string `yaml:"private_key_pem"`
	PublicKeyPEM  string `yaml:"public_key_pem"`
}

type AuthConfig struct {
	HashPepper string `yaml:"hash_pepper"`
}

type EmailConfig struct {
	Addr     string `yaml:"addr"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	ReplyTo  string `yaml:"reply_to"`
	UseTLS   bool   `yaml:"use_tls"`
}

// 加载默认值
func Default() Config {
	return Config{
		App: AppConfig{
			Env: "local",
		},
		Logging: LoggingConfig{
			LogLevel:     "info",
			LogAddSource: false,
		},
		HTTP: HTTPConfig{
			Addr: ":8080",
		},
		Postgres: PostgresConfig{
			DSN: "postgres://postgres:postgres@127.0.0.1:5432/cixing?sslmode=disable",
		},
		Redis: RedisConfig{
			Addr: "localhost:6379",
			DB:   0,
		},
		OSS: OSSConfig{
			CredentialMode:            "static",
			ECSRoleName:               "CixingEcsOssUploadRole",
			AssumeRoleSessionDuration: time.Hour,
			PutPresignExpire:          15 * time.Minute,
			GetPresignExpire:          10 * time.Minute,
			UploadPrefix:              "uploads/",
			Styles: OSSStyleConfig{
				Card4x3:     "card_4x3",
				SquareSmall: "square_small",
				SquareMed:   "square_medium",
				DetailLarge: "detail_large",
			},
		},
		JWT:  JWTConfig{},
		Auth: AuthConfig{},
	}
}

func (c *Config) Normalize() {
	c.App.Env = strings.TrimSpace(c.App.Env)
	c.Logging.LogLevel = strings.TrimSpace(c.Logging.LogLevel)

	if c.HTTP.Addr == "" {
		c.HTTP.Addr = ":8080"
	}
	c.Postgres.DSN = strings.TrimSpace(c.Postgres.DSN)
	c.Redis.Addr = strings.TrimSpace(c.Redis.Addr)
	c.Redis.Password = strings.TrimSpace(c.Redis.Password)

	c.OSS.Bucket = strings.TrimSpace(c.OSS.Bucket)
	c.OSS.Region = strings.TrimSpace(c.OSS.Region)
	c.OSS.CredentialMode = strings.ToLower(strings.TrimSpace(c.OSS.CredentialMode))
	c.OSS.ECSRoleName = strings.TrimSpace(c.OSS.ECSRoleName)
	c.OSS.AssumeRoleARN = strings.TrimSpace(c.OSS.AssumeRoleARN)
	c.OSS.AssumeRoleSessionName = strings.TrimSpace(c.OSS.AssumeRoleSessionName)
	c.OSS.AssumeRoleExternalID = strings.TrimSpace(c.OSS.AssumeRoleExternalID)
	c.OSS.AssumeRoleSTSEndpoint = strings.TrimSpace(c.OSS.AssumeRoleSTSEndpoint)
	c.OSS.AccessKeyID = strings.TrimSpace(c.OSS.AccessKeyID)
	c.OSS.AccessKeySecret = strings.TrimSpace(c.OSS.AccessKeySecret)
	c.OSS.PublicEndpoint = strings.TrimSpace(c.OSS.PublicEndpoint)
	c.OSS.InternalEndpoint = strings.TrimSpace(c.OSS.InternalEndpoint)
	c.OSS.UploadPrefix = strings.TrimSpace(c.OSS.UploadPrefix)
	c.OSS.Styles.Card4x3 = strings.TrimSpace(c.OSS.Styles.Card4x3)
	c.OSS.Styles.SquareSmall = strings.TrimSpace(c.OSS.Styles.SquareSmall)
	c.OSS.Styles.SquareMed = strings.TrimSpace(c.OSS.Styles.SquareMed)
	c.OSS.Styles.DetailLarge = strings.TrimSpace(c.OSS.Styles.DetailLarge)
	c.Auth.HashPepper = strings.TrimSpace(c.Auth.HashPepper)
	c.Email.Addr = strings.TrimSpace(c.Email.Addr)
	c.Email.Username = strings.TrimSpace(c.Email.Username)
	c.Email.Password = strings.TrimSpace(c.Email.Password)
	c.Email.From = strings.TrimSpace(c.Email.From)
	c.Email.ReplyTo = strings.TrimSpace(c.Email.ReplyTo)
}

// 校验必须的配置
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Postgres.DSN) == "" {
		return fmt.Errorf("config: postgres.dsn is required")
	}
	if strings.TrimSpace(c.Redis.Addr) == "" {
		return fmt.Errorf("config: redis.addr is required")
	}

	if strings.TrimSpace(c.OSS.Bucket) == "" {
		return fmt.Errorf("config: oss.bucket is required")
	}
	if strings.TrimSpace(c.OSS.Region) == "" {
		return fmt.Errorf("config: oss.region is required")
	}
	if strings.TrimSpace(c.JWT.PrivateKeyPEM) == "" || strings.TrimSpace(c.JWT.PublicKeyPEM) == "" {
		return fmt.Errorf("config: jwt private/public key pem is required")
	}
	if strings.TrimSpace(c.Auth.HashPepper) == "" {
		return fmt.Errorf("config: auth.hash_pepper is required")
	}

	if strings.TrimSpace(c.Email.Addr) == "" {
		return fmt.Errorf("config: email.addr is required")
	}
	if err := validateSMTPAddr(c.Email.Addr, "email.addr"); err != nil {
		return err
	}
	if strings.TrimSpace(c.Email.From) == "" {
		return fmt.Errorf("config: email.from is required")
	}

	return nil
}

// 校验 SMTP 地址是不是 host:port 格式
func validateSMTPAddr(addr string, field string) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return fmt.Errorf("config: %s must be host:port", field)
	}
	return nil
}
