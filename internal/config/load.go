package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// 默认的配置 yaml 文件
const defaultConfigFile = "config.yaml"

// Load() 不设置 opts，也即不跳过 Validate
func Load() (*Config, error) {
	// 在 env 设置了 config 文件路径，则覆盖默认的 config yaml
	return LoadWith(Options{ConfigFile: strings.TrimSpace(os.Getenv(EnvConfigFile))})
}

type Options struct {
	ConfigFile   string
	SkipValidate bool // 跳过 Validate 的设置
}

// LoadWith() 设置 opts，可跳过 Validate
// 覆盖优先级：default < yaml < env
func LoadWith(opts Options) (*Config, error) {
	// 加载默认值
	cfg := Default()
	configFile := strings.TrimSpace(opts.ConfigFile)
	if configFile == "" {
		configFile = defaultConfigFile
	}

	// 加载 config.yaml 中的配置
	if err := loadYAML(&cfg, configFile); err != nil {
		return nil, err
	}
	// 加载 env 配置
	if err := applyEnv(&cfg); err != nil {
		return nil, err
	}

	// 标准化
	cfg.Normalize()
	// SkipValidate 为 true 则跳过 Validate，如用于 migrate 命令等不需要完整配置的场景
	if !opts.SkipValidate {
		if err := cfg.Validate(); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

// 加载 yaml 文件配置
func loadYAML(cfg *Config, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config: read yaml: %w", err)
	}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return fmt.Errorf("config: parse yaml: %w", err)
	}
	return nil
}

// 应用环境变量覆盖配置
func applyEnv(cfg *Config) error {
	setString(&cfg.Postgres.DSN, EnvPGDSN)
	setString(&cfg.Redis.Addr, EnvRedisAddr)
	setString(&cfg.Redis.Password, EnvRedisPassword)
	if err := setInt(&cfg.Redis.DB, EnvRedisDB); err != nil {
		return err
	}

	setString(&cfg.OSS.AccessKeyID, EnvOSSAccessKeyID)
	setString(&cfg.OSS.AccessKeySecret, EnvOSSAccessKeySecret)
	setPEM(&cfg.JWT.PrivateKeyPEM, EnvJWTPrivatePEM)
	setPEM(&cfg.JWT.PublicKeyPEM, EnvJWTPublicPEM)

	setString(&cfg.Email.Password, EnvSMTPPass)

	return nil
}

func readEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// 下面是一些 env 覆盖配置的 helper 函数

// String 类型直接赋值
func setString(dst *string, key string) {
	if v := readEnv(key); v != "" {
		*dst = v
	}
}

// 处理 \n 到真实换行
func setPEM(dst *string, key string) {
	if v := readEnv(key); v != "" {
		*dst = strings.ReplaceAll(v, `\n`, "\n")
	}
}

// 整数要做 Atoi 转换
func setInt(dst *int, key string) error {
	if v := readEnv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("config: invalid int for %s: %w", key, err)
		}
		*dst = n
	}
	return nil
}
