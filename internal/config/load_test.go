package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadWith_UsesDefaultConfigFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(%s) error = %v", tempDir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	if err := os.WriteFile("config.yaml", []byte(`
app:
  env: from-yaml
http:
  addr: ":9090"
`), 0o644); err != nil {
		t.Fatalf("WriteFile(config.yaml) error = %v", err)
	}

	cfg, err := LoadWith(Options{SkipValidate: true})
	if err != nil {
		t.Fatalf("LoadWith() error = %v", err)
	}
	if cfg.App.Env != "from-yaml" {
		t.Fatalf("App.Env = %q, want from-yaml", cfg.App.Env)
	}
	if cfg.HTTP.Addr != ":9090" {
		t.Fatalf("HTTP.Addr = %q, want :9090", cfg.HTTP.Addr)
	}
}

func TestLoadWith_OnlyOverridesMinimalEnvSet(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
app:
  env: from-yaml
http:
  addr: ":8088"
logging:
  log_level: "warn"
postgres:
  dsn: "postgres://yaml"
redis:
  addr: "yaml-redis:6379"
  password: "yaml-pass"
  db: 1
email:
  password: "yaml-smtp"
jwt:
  private_key_pem: "yaml-private"
  public_key_pem: "yaml-public"
`), 0o644); err != nil {
		t.Fatalf("WriteFile(config.yaml) error = %v", err)
	}

	t.Setenv("PG_DSN", "postgres://env")
	t.Setenv("REDIS_ADDR", "env-redis:6379")
	t.Setenv("REDIS_PASSWORD", "env-pass")
	t.Setenv("REDIS_DB", "7")
	t.Setenv("SMTP_PASSWORD", "env-smtp")
	t.Setenv("JWT_PRIVATE_KEY_PEM", "env-private")
	t.Setenv("JWT_PUBLIC_KEY_PEM", "env-public")
	t.Setenv("OSS_CREDENTIAL_MODE", "ecs_ram_role_assume_role")
	t.Setenv("OSS_ECS_ROLE_NAME", "env-source-role")
	t.Setenv("OSS_ASSUME_ROLE_ARN", "acs:ram::1234567890123456:role/env-target-role")
	t.Setenv("OSS_ASSUME_ROLE_SESSION_NAME", "env-session")
	t.Setenv("OSS_ASSUME_ROLE_EXTERNAL_ID", "env-external")
	t.Setenv("OSS_ASSUME_ROLE_STS_ENDPOINT", "sts.cn-hongkong.aliyuncs.com")
	t.Setenv("OSS_ASSUME_ROLE_SESSION_DURATION", "45m")

	// These used to override config, but should no longer be applied.
	t.Setenv("HTTP_ADDR", ":9999")
	t.Setenv("APP_ENV", "from-env")

	cfg, err := LoadWith(Options{ConfigFile: configPath, SkipValidate: true})
	if err != nil {
		t.Fatalf("LoadWith() error = %v", err)
	}

	if cfg.Postgres.DSN != "postgres://env" {
		t.Fatalf("Postgres.DSN = %q, want postgres://env", cfg.Postgres.DSN)
	}
	if cfg.Redis.Addr != "env-redis:6379" {
		t.Fatalf("Redis.Addr = %q, want env-redis:6379", cfg.Redis.Addr)
	}
	if cfg.Redis.Password != "env-pass" {
		t.Fatalf("Redis.Password = %q, want env-pass", cfg.Redis.Password)
	}
	if cfg.Redis.DB != 7 {
		t.Fatalf("Redis.DB = %d, want 7", cfg.Redis.DB)
	}
	if cfg.Email.Password != "env-smtp" {
		t.Fatalf("Email.Password = %q, want env-smtp", cfg.Email.Password)
	}
	if cfg.JWT.PrivateKeyPEM != "env-private" {
		t.Fatalf("JWT.PrivateKeyPEM = %q, want env-private", cfg.JWT.PrivateKeyPEM)
	}
	if cfg.JWT.PublicKeyPEM != "env-public" {
		t.Fatalf("JWT.PublicKeyPEM = %q, want env-public", cfg.JWT.PublicKeyPEM)
	}
	if cfg.OSS.CredentialMode != "ecs_ram_role_assume_role" {
		t.Fatalf("OSS.CredentialMode = %q, want ecs_ram_role_assume_role", cfg.OSS.CredentialMode)
	}
	if cfg.OSS.ECSRoleName != "env-source-role" {
		t.Fatalf("OSS.ECSRoleName = %q, want env-source-role", cfg.OSS.ECSRoleName)
	}
	if cfg.OSS.AssumeRoleARN != "acs:ram::1234567890123456:role/env-target-role" {
		t.Fatalf("OSS.AssumeRoleARN = %q, want env target role arn", cfg.OSS.AssumeRoleARN)
	}
	if cfg.OSS.AssumeRoleSessionName != "env-session" {
		t.Fatalf("OSS.AssumeRoleSessionName = %q, want env-session", cfg.OSS.AssumeRoleSessionName)
	}
	if cfg.OSS.AssumeRoleExternalID != "env-external" {
		t.Fatalf("OSS.AssumeRoleExternalID = %q, want env-external", cfg.OSS.AssumeRoleExternalID)
	}
	if cfg.OSS.AssumeRoleSTSEndpoint != "sts.cn-hongkong.aliyuncs.com" {
		t.Fatalf("OSS.AssumeRoleSTSEndpoint = %q, want sts.cn-hongkong.aliyuncs.com", cfg.OSS.AssumeRoleSTSEndpoint)
	}
	if cfg.OSS.AssumeRoleSessionDuration != 45*time.Minute {
		t.Fatalf("OSS.AssumeRoleSessionDuration = %s, want 45m", cfg.OSS.AssumeRoleSessionDuration)
	}
	if cfg.HTTP.Addr != ":8088" {
		t.Fatalf("HTTP.Addr = %q, want :8088", cfg.HTTP.Addr)
	}
	if cfg.App.Env != "from-yaml" {
		t.Fatalf("App.Env = %q, want from-yaml", cfg.App.Env)
	}
	if cfg.Logging.LogLevel != "warn" {
		t.Fatalf("Logging.LogLevel = %q, want warn", cfg.Logging.LogLevel)
	}
}

func TestLoadWith_LoadsOSSAssumeRoleFields(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
oss:
  credential_mode: " ECS_RAM_ROLE_ASSUME_ROLE "
  ecs_role_name: " source-role "
  assume_role_arn: " acs:ram::1234567890123456:role/target-role "
  assume_role_session_name: " cixing-test "
  assume_role_external_id: " external-id "
  assume_role_sts_endpoint: " sts.cn-hongkong.aliyuncs.com "
  assume_role_session_duration: 30m
`), 0o644); err != nil {
		t.Fatalf("WriteFile(config.yaml) error = %v", err)
	}

	cfg, err := LoadWith(Options{ConfigFile: configPath, SkipValidate: true})
	if err != nil {
		t.Fatalf("LoadWith() error = %v", err)
	}

	if cfg.OSS.CredentialMode != "ecs_ram_role_assume_role" {
		t.Fatalf("OSS.CredentialMode = %q, want ecs_ram_role_assume_role", cfg.OSS.CredentialMode)
	}
	if cfg.OSS.ECSRoleName != "source-role" {
		t.Fatalf("OSS.ECSRoleName = %q, want source-role", cfg.OSS.ECSRoleName)
	}
	if cfg.OSS.AssumeRoleARN != "acs:ram::1234567890123456:role/target-role" {
		t.Fatalf("OSS.AssumeRoleARN = %q, want trimmed target role arn", cfg.OSS.AssumeRoleARN)
	}
	if cfg.OSS.AssumeRoleSessionName != "cixing-test" {
		t.Fatalf("OSS.AssumeRoleSessionName = %q, want cixing-test", cfg.OSS.AssumeRoleSessionName)
	}
	if cfg.OSS.AssumeRoleExternalID != "external-id" {
		t.Fatalf("OSS.AssumeRoleExternalID = %q, want external-id", cfg.OSS.AssumeRoleExternalID)
	}
	if cfg.OSS.AssumeRoleSTSEndpoint != "sts.cn-hongkong.aliyuncs.com" {
		t.Fatalf("OSS.AssumeRoleSTSEndpoint = %q, want sts.cn-hongkong.aliyuncs.com", cfg.OSS.AssumeRoleSTSEndpoint)
	}
	if cfg.OSS.AssumeRoleSessionDuration != 30*time.Minute {
		t.Fatalf("OSS.AssumeRoleSessionDuration = %s, want 30m", cfg.OSS.AssumeRoleSessionDuration)
	}
}

func TestLoadWith_InvalidOSSAssumeRoleSessionDurationEnv(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`app: {}`), 0o644); err != nil {
		t.Fatalf("WriteFile(config.yaml) error = %v", err)
	}
	t.Setenv("OSS_ASSUME_ROLE_SESSION_DURATION", "not-a-duration")

	_, err := LoadWith(Options{ConfigFile: configPath, SkipValidate: true})
	if err == nil {
		t.Fatalf("LoadWith() error = nil, want invalid duration error")
	}
	if got := err.Error(); !strings.Contains(got, "OSS_ASSUME_ROLE_SESSION_DURATION") {
		t.Fatalf("LoadWith() error = %q, want OSS_ASSUME_ROLE_SESSION_DURATION", got)
	}
}

func TestConfigValidate_OnlyChecksHardRequirements(t *testing.T) {
	cfg := Default()
	cfg.OSS.Bucket = "bucket"
	cfg.OSS.Region = "cn-hongkong"
	cfg.JWT.PrivateKeyPEM = "private"
	cfg.JWT.PublicKeyPEM = "public"
	cfg.Auth.HashPepper = "pepper"
	cfg.Email.Addr = "localhost:1025"
	cfg.Email.From = "noreply@example.com"

	cfg.App.Env = ""
	cfg.HTTP.Addr = ""
	cfg.OSS.CredentialMode = "unsupported"
	cfg.OSS.PutPresignExpire = 0
	cfg.OSS.GetPresignExpire = 0

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidate_RequiresHardDependencies(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "postgres dsn",
			mutate: func(cfg *Config) {
				cfg.Postgres.DSN = ""
			},
			wantErr: "postgres.dsn",
		},
		{
			name: "redis addr",
			mutate: func(cfg *Config) {
				cfg.Redis.Addr = ""
			},
			wantErr: "redis.addr",
		},
		{
			name: "oss bucket",
			mutate: func(cfg *Config) {
				cfg.OSS.Bucket = ""
			},
			wantErr: "oss.bucket",
		},
		{
			name: "oss region",
			mutate: func(cfg *Config) {
				cfg.OSS.Region = ""
			},
			wantErr: "oss.region",
		},
		{
			name: "jwt keys",
			mutate: func(cfg *Config) {
				cfg.JWT.PrivateKeyPEM = ""
			},
			wantErr: "jwt private/public key pem",
		},
		{
			name: "auth hash pepper",
			mutate: func(cfg *Config) {
				cfg.Auth.HashPepper = ""
			},
			wantErr: "auth.hash_pepper",
		},
		{
			name: "email addr",
			mutate: func(cfg *Config) {
				cfg.Email.Addr = ""
			},
			wantErr: "email.addr",
		},
		{
			name: "email addr format",
			mutate: func(cfg *Config) {
				cfg.Email.Addr = "localhost"
			},
			wantErr: "email.addr must be host:port",
		},
		{
			name: "email from",
			mutate: func(cfg *Config) {
				cfg.Email.From = ""
			},
			wantErr: "email.from",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Default()
			cfg.OSS.Bucket = "bucket"
			cfg.OSS.Region = "cn-hongkong"
			cfg.JWT.PrivateKeyPEM = "private"
			cfg.JWT.PublicKeyPEM = "public"
			cfg.Auth.HashPepper = "pepper"
			cfg.Email.Addr = "localhost:1025"
			cfg.Email.From = "noreply@example.com"

			tc.mutate(&cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", tc.wantErr)
			}
			if got := err.Error(); got == "" || !strings.Contains(got, tc.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestConfigNormalize_TrimsEmailReplyTo(t *testing.T) {
	cfg := Default()
	cfg.Email.ReplyTo = "  reply@example.com  "

	cfg.Normalize()

	if cfg.Email.ReplyTo != "reply@example.com" {
		t.Fatalf("Email.ReplyTo = %q, want reply@example.com", cfg.Email.ReplyTo)
	}
}
