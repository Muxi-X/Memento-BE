package config

const (
	EnvConfigFile = "CONFIG_FILE"

	EnvPGDSN = "PG_DSN"

	EnvRedisAddr     = "REDIS_ADDR"
	EnvRedisPassword = "REDIS_PASSWORD"
	EnvRedisDB       = "REDIS_DB"

	EnvOSSCredentialMode            = "OSS_CREDENTIAL_MODE"
	EnvOSSECSRoleName               = "OSS_ECS_ROLE_NAME"
	EnvOSSAccessKeyID               = "OSS_ACCESS_KEY_ID"
	EnvOSSAccessKeySecret           = "OSS_ACCESS_KEY_SECRET"
	EnvOSSAssumeRoleARN             = "OSS_ASSUME_ROLE_ARN"
	EnvOSSAssumeRoleSessionName     = "OSS_ASSUME_ROLE_SESSION_NAME"
	EnvOSSAssumeRoleExternalID      = "OSS_ASSUME_ROLE_EXTERNAL_ID"
	EnvOSSAssumeRoleSTSEndpoint     = "OSS_ASSUME_ROLE_STS_ENDPOINT"
	EnvOSSAssumeRoleSessionDuration = "OSS_ASSUME_ROLE_SESSION_DURATION"

	EnvJWTPrivatePEM = "JWT_PRIVATE_KEY_PEM"
	EnvJWTPublicPEM  = "JWT_PUBLIC_KEY_PEM"

	EnvSMTPPass = "SMTP_PASSWORD"
)
