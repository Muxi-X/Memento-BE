//go:build !windows

package oss

import (
	"context"
	"fmt"
	"strings"

	osscredentials "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	openapicred "github.com/aliyun/credentials-go/credentials"

	appcfg "cixing/internal/config"
)

// 只在非 Windows 平台编译
// newECSRAMRoleCredentialsProvider 创建一个基于 ECS RAM Role 的 CredentialsProvider
func newECSRAMRoleCredentialsProvider(cfg appcfg.OSSConfig) (osscredentials.CredentialsProvider, error) {
	// 构造阿里云 credentials-go 的配置
	credCfg := new(openapicred.Config).
		SetType("ecs_ram_role").
		SetRoleName(strings.TrimSpace(cfg.ECSRoleName))
	// 可选禁用旧版的 ECS 元数据服务访问方式
	if cfg.DisableIMDSv1 {
		credCfg.SetDisableIMDSv1(true)
	}

	// 创建凭证获取客户端
	credClient, err := openapicred.NewCredential(credCfg)
	if err != nil {
		return nil, fmt.Errorf("oss: init ecs ram role credential: %w", err)
	}

	// 包装成 OSS SDK 需要的 CredentialsProvider
	return osscredentials.CredentialsProviderFunc(func(ctx context.Context) (osscredentials.Credentials, error) {
		// 获取凭证
		cred, err := credClient.GetCredential()
		if err != nil {
			return osscredentials.Credentials{}, err
		}

		// 转换成 OSS SDK 需要的 Credentials 格式
		out := osscredentials.Credentials{}
		if cred.AccessKeyId != nil {
			out.AccessKeyID = *cred.AccessKeyId
		}
		if cred.AccessKeySecret != nil {
			out.AccessKeySecret = *cred.AccessKeySecret
		}
		if cred.SecurityToken != nil {
			out.SecurityToken = *cred.SecurityToken
		}
		return out, nil
	}), nil
}
