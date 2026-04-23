package oss

import (
	"fmt"
	"net/url"
	"strings"

	alioss "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"

	appcfg "cixing/internal/config"
)

type Clients struct {
	Internal *alioss.Client // 给外部/前端使用
	Public   *alioss.Client // 给后端服务自己用
}

type clientOptions struct {
	endpoint            string
	endpointIsCName     bool
	useInternalEndpoint bool
}

// 创建 Clients，同时具有 Internal 和 Public
func NewClients(cfg appcfg.OSSConfig) (*Clients, error) {
	// 基础校验
	if strings.TrimSpace(cfg.Region) == "" {
		return nil, fmt.Errorf("oss: region is required")
	}
	if err := validateCredentialsConfig(cfg); err != nil {
		return nil, err
	}

	// 创建公共 client，给外部/前端使用
	publicOpts := clientOptions{
		endpoint:        cfg.PublicEndpoint,
		endpointIsCName: cfg.PublicEndpointIsCName,
	}
	publicClient, _, err := newClient(cfg, publicOpts)
	if err != nil {
		return nil, err
	}

	// 判断是否需要创建专门的 internal client
	internalOpts := clientOptions{
		endpoint:            cfg.InternalEndpoint,
		endpointIsCName:     cfg.InternalEndpointIsCName,
		useInternalEndpoint: cfg.UseInternalEndpoint && strings.TrimSpace(cfg.InternalEndpoint) == "",
	}
	if !needsDedicatedInternalClient(publicOpts, internalOpts) {
		return &Clients{Internal: publicClient, Public: publicClient}, nil
	}

	internalClient, _, err := newClient(cfg, internalOpts)
	if err != nil {
		return nil, err
	}
	return &Clients{Internal: internalClient, Public: publicClient}, nil
}

func validateCredentialsConfig(cfg appcfg.OSSConfig) error {
	switch credentialMode(cfg) {
	case credentialModeStatic:
		if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.AccessKeySecret) == "" {
			return fmt.Errorf("oss: access key id/secret are required")
		}
	case credentialModeECSRAMRole:
		if strings.TrimSpace(cfg.ECSRoleName) == "" {
			return fmt.Errorf("oss: ecs role name is required for ecs_ram_role")
		}
	case credentialModeECSRAMRoleAssumeRole:
		if strings.TrimSpace(cfg.ECSRoleName) == "" {
			return fmt.Errorf("oss: ecs role name is required for ecs_ram_role_assume_role")
		}
		if strings.TrimSpace(cfg.AssumeRoleARN) == "" {
			return fmt.Errorf("oss: assume role arn is required for ecs_ram_role_assume_role")
		}
		if _, err := assumeRoleSessionDurationSeconds(cfg.AssumeRoleSessionDuration); err != nil {
			return err
		}
	default:
		return fmt.Errorf("oss: unsupported credential mode %q", cfg.CredentialMode)
	}
	return nil
}

// 判断是否需要不同的 client
func needsDedicatedInternalClient(publicOpts, internalOpts clientOptions) bool {
	if internalOpts.useInternalEndpoint {
		return true
	}

	publicEndpoint := strings.TrimSpace(publicOpts.endpoint)
	internalEndpoint := strings.TrimSpace(internalOpts.endpoint)
	if internalEndpoint == "" {
		return false
	}

	return publicEndpoint != internalEndpoint || publicOpts.endpointIsCName != internalOpts.endpointIsCName
}

func newClient(cfg appcfg.OSSConfig, opts clientOptions) (*alioss.Client, string, error) {
	// 规范化 endpoint
	normalizedEndpoint, err := normalizeEndpoint(opts.endpoint)
	if err != nil {
		return nil, "", err
	}

	// 创建 CredentialsProvider
	provider, err := newCredentialsProvider(cfg)
	if err != nil {
		return nil, "", err
	}

	// 创建 OSS Client
	ossCfg := alioss.LoadDefaultConfig().
		WithRegion(strings.TrimSpace(cfg.Region)).
		WithCredentialsProvider(provider)

	if normalizedEndpoint != "" {
		ossCfg.WithEndpoint(normalizedEndpoint)
	}
	if opts.endpointIsCName {
		ossCfg.WithUseCName(true)
	}
	if opts.useInternalEndpoint {
		ossCfg.WithUseInternalEndpoint(true)
	}

	return alioss.NewClient(ossCfg), normalizedEndpoint, nil
}

// 获取 credentialMode，默认为 static
func credentialMode(cfg appcfg.OSSConfig) string {
	mode := strings.ToLower(strings.TrimSpace(cfg.CredentialMode))
	if mode == "" {
		return credentialModeStatic
	}
	return mode
}

// 规范化 endpoint
func normalizeEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", nil
	}
	if !strings.Contains(endpoint, "://") {
		if strings.Contains(endpoint, "/") {
			return "", fmt.Errorf("oss: invalid endpoint %q", endpoint)
		}
		return strings.TrimRight(endpoint, "/"), nil
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("oss: invalid endpoint %q: %w", endpoint, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("oss: invalid endpoint %q", endpoint)
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("oss: endpoint must not contain a path: %q", endpoint)
	}

	return strings.TrimRight(u.Scheme+"://"+u.Host, "/"), nil
}
