//go:build windows

package oss

import (
	"fmt"

	osscredentials "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"

	appcfg "cixing/internal/config"
)

// 只在 Windows 平台编译
// 直接返回错误，提示 Windows 平台不支持 ECS RAM Role 方式获取临时凭证，要走 static
func newECSRAMRoleCredentialsProvider(_ appcfg.OSSConfig) (osscredentials.CredentialsProvider, error) {
	return nil, fmt.Errorf("oss: ecs_ram_role credential mode is not supported in Windows builds; use static mode locally and ecs_ram_role on ECS/Linux")
}
