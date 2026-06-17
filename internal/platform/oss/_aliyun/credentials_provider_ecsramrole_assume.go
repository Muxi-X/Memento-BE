//go:build !windows

package oss

import (
	"context"
	"fmt"
	"strings"

	osscredentials "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/aliyun/credentials-go/credentials/providers"

	appcfg "cixing/internal/config"
)

func newECSRAMRoleAssumeRoleCredentialsProvider(cfg appcfg.OSSConfig) (osscredentials.CredentialsProvider, error) {
	roleName := strings.TrimSpace(cfg.ECSRoleName)
	if roleName == "" {
		return nil, fmt.Errorf("oss: ecs role name is required for ecs_ram_role_assume_role")
	}
	roleARN := strings.TrimSpace(cfg.AssumeRoleARN)
	if roleARN == "" {
		return nil, fmt.Errorf("oss: assume role arn is required for ecs_ram_role_assume_role")
	}
	durationSeconds, err := assumeRoleSessionDurationSeconds(cfg.AssumeRoleSessionDuration)
	if err != nil {
		return nil, err
	}

	sourceProvider, err := providers.NewECSRAMRoleCredentialsProviderBuilder().
		WithRoleName(roleName).
		WithDisableIMDSv1(cfg.DisableIMDSv1).
		Build()
	if err != nil {
		return nil, fmt.Errorf("oss: init ecs ram role source credential: %w", err)
	}

	targetBuilder := providers.NewRAMRoleARNCredentialsProviderBuilder().
		WithCredentialsProvider(sourceProvider).
		WithRoleArn(roleARN).
		WithRoleSessionName(strings.TrimSpace(cfg.AssumeRoleSessionName)).
		WithDurationSeconds(durationSeconds)
	if externalID := strings.TrimSpace(cfg.AssumeRoleExternalID); externalID != "" {
		targetBuilder.WithExternalId(externalID)
	}
	if stsEndpoint := strings.TrimSpace(cfg.AssumeRoleSTSEndpoint); stsEndpoint != "" {
		targetBuilder.WithStsEndpoint(stsEndpoint)
	}

	targetProvider, err := targetBuilder.Build()
	if err != nil {
		return nil, fmt.Errorf("oss: init assume role credential: %w", err)
	}

	return osscredentials.CredentialsProviderFunc(func(context.Context) (osscredentials.Credentials, error) {
		cred, err := targetProvider.GetCredentials()
		if err != nil {
			return osscredentials.Credentials{}, err
		}
		return osscredentials.Credentials{
			AccessKeyID:     cred.AccessKeyId,
			AccessKeySecret: cred.AccessKeySecret,
			SecurityToken:   cred.SecurityToken,
		}, nil
	}), nil
}
