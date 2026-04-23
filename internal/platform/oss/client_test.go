package oss

import (
	"strings"
	"testing"
	"time"

	appcfg "cixing/internal/config"
)

func TestValidateCredentialsConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     appcfg.OSSConfig
		wantErr string
	}{
		{
			name: "static missing access key",
			cfg: appcfg.OSSConfig{
				CredentialMode: credentialModeStatic,
			},
			wantErr: "access key id/secret",
		},
		{
			name: "ecs ram role missing role name",
			cfg: appcfg.OSSConfig{
				CredentialMode: credentialModeECSRAMRole,
			},
			wantErr: "ecs role name",
		},
		{
			name: "assume role missing source role name",
			cfg: appcfg.OSSConfig{
				CredentialMode: credentialModeECSRAMRoleAssumeRole,
				AssumeRoleARN:  "acs:ram::1234567890123456:role/target-role",
			},
			wantErr: "ecs role name",
		},
		{
			name: "assume role missing target role arn",
			cfg: appcfg.OSSConfig{
				CredentialMode: credentialModeECSRAMRoleAssumeRole,
				ECSRoleName:    "source-role",
			},
			wantErr: "assume role arn",
		},
		{
			name: "assume role duration too short",
			cfg: appcfg.OSSConfig{
				CredentialMode:            credentialModeECSRAMRoleAssumeRole,
				ECSRoleName:               "source-role",
				AssumeRoleARN:             "acs:ram::1234567890123456:role/target-role",
				AssumeRoleSessionDuration: 14 * time.Minute,
			},
			wantErr: "assume role session duration",
		},
		{
			name: "assume role valid with default duration",
			cfg: appcfg.OSSConfig{
				CredentialMode: credentialModeECSRAMRoleAssumeRole,
				ECSRoleName:    "source-role",
				AssumeRoleARN:  "acs:ram::1234567890123456:role/target-role",
			},
		},
		{
			name: "assume role valid with explicit duration",
			cfg: appcfg.OSSConfig{
				CredentialMode:            credentialModeECSRAMRoleAssumeRole,
				ECSRoleName:               "source-role",
				AssumeRoleARN:             "acs:ram::1234567890123456:role/target-role",
				AssumeRoleSessionDuration: 15 * time.Minute,
			},
		},
		{
			name: "unsupported",
			cfg: appcfg.OSSConfig{
				CredentialMode: "unsupported",
			},
			wantErr: "unsupported credential mode",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCredentialsConfig(tc.cfg)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateCredentialsConfig() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateCredentialsConfig() error = nil, want %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("validateCredentialsConfig() error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestCredentialModeDefaultsToStatic(t *testing.T) {
	if got := credentialMode(appcfg.OSSConfig{}); got != credentialModeStatic {
		t.Fatalf("credentialMode() = %q, want %q", got, credentialModeStatic)
	}
}
