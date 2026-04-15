package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"cixing/internal/config"
	authapp "cixing/internal/modules/auth/application"
	authrepo "cixing/internal/modules/auth/infra/db/repo"
	customapp "cixing/internal/modules/customkeywords/application"
	customdb "cixing/internal/modules/customkeywords/infra/db/gen"
	customrepo "cixing/internal/modules/customkeywords/infra/db/repo"
	officialapp "cixing/internal/modules/official/application"
	officialdb "cixing/internal/modules/official/infra/db/gen"
	officialrepo "cixing/internal/modules/official/infra/db/repo"
	profileapp "cixing/internal/modules/profile/application"
	profiledb "cixing/internal/modules/profile/infra/db/gen"
	profilerepo "cixing/internal/modules/profile/infra/db/repo"
	publishingapp "cixing/internal/modules/publishing/application"
	readmodelapp "cixing/internal/modules/readmodel/application"
	readmodeldb "cixing/internal/modules/readmodel/infra/db/gen"
	readmodelrepo "cixing/internal/modules/readmodel/infra/db/repo"
	socialapp "cixing/internal/modules/social/application"
	platformemail "cixing/internal/platform/email"
	platformlogging "cixing/internal/platform/logging"
	platformoss "cixing/internal/platform/oss"
	platformpostgres "cixing/internal/platform/postgres"
	platformredis "cixing/internal/platform/redis"
	"cixing/internal/transport/http/server"
	v1 "cixing/internal/transport/http/v1"
)

const (
	defaultHTTPReadTimeout  = 10 * time.Second
	defaultHTTPWriteTimeout = 10 * time.Second
	defaultHTTPIdleTimeout  = 60 * time.Second
	defaultRateLimitRPS     = 10
	defaultRateLimitBurst   = 20

	defaultJWTIssuer    = "cixing"
	defaultJWTKID       = "main"
	defaultJWTAccessTTL = 72 * time.Hour

	defaultAuthCodeTTL           = 10 * time.Minute
	defaultAuthCodeCooldown      = 60 * time.Second
	defaultAuthCodeMaxAttempts   = 5
	defaultAuthCodeLockTTL       = 10 * time.Minute
	defaultAuthSignupTokenTTL    = 10 * time.Minute
	defaultAuthResetTokenTTL     = 10 * time.Minute
	defaultAuthPasswordMinLength = 8
)

// api 入口，config.Load() 加载配置，交给 RunWithConfig 启动服务
func Run(ctx context.Context) error {
	// 加载逻辑：Default() -> YAML 文件 -> 环境变量覆盖 -> Normalize() -> Validate()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return RunWithConfig(ctx, cfg)
}

// 使用设定好的配置 启动 API 服务，以别的 config 启动时可以直接调用 RunWithConfig
// 初始化 Logger / Metrics / Postgres / Redis / 各种 Service
// 组装 Gin router
// 启动 http.Server
// 监听信号，关闭服务
func RunWithConfig(ctx context.Context, cfg *config.Config) error {
	// 初始化 logger
	logger := platformlogging.NewLogger(platformlogging.LoggerConfig{
		Service:   "cixing-api",
		Env:       cfg.App.Env,
		Level:     cfg.Logging.LogLevel,
		AddSource: cfg.Logging.LogAddSource,
	})
	// 初始化 Postgres 连接池
	pool, err := platformpostgres.NewPool(ctx, platformpostgres.PoolConfig{
		DSN: cfg.Postgres.DSN,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	// 初始化 Redis 客户端
	redisClient, err := platformredis.NewClient(ctx, cfg.Redis)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	// 初始化 EmailSender
	sender, err := platformemail.NewSender(cfg.Email)
	if err != nil {
		return err
	}

	authModuleRepo := authrepo.NewRepository(pool)
	authRegistrationRepo := authrepo.NewRegistrationRepository(pool)
	authM, err := authapp.NewModule(authapp.Deps{
		CodeStore:        platformredis.NewAuthCodeStore(redisClient),
		EmailSender:      sender,
		AuthRepo:         authModuleRepo,
		RegistrationRepo: authRegistrationRepo,
	}, authapp.Config{
		HashPepper:        cfg.Auth.HashPepper,
		CodeTTL:           defaultAuthCodeTTL,
		CooldownTTL:       defaultAuthCodeCooldown,
		LockTTL:           defaultAuthCodeLockTTL,
		MaxAttempts:       defaultAuthCodeMaxAttempts,
		ResetTokenTTL:     defaultAuthResetTokenTTL,
		PasswordMinLength: defaultAuthPasswordMinLength,
		SignupTokenTTL:    defaultAuthSignupTokenTTL,
		JWT: authapp.JWTConfig{
			Issuer:        defaultJWTIssuer,
			KID:           defaultJWTKID,
			PrivateKeyPEM: cfg.JWT.PrivateKeyPEM,
			PublicKeyPEM:  cfg.JWT.PublicKeyPEM,
			AccessTTL:     defaultJWTAccessTTL,
		},
	})
	if err != nil {
		return err
	}

	objectStorage, err := platformoss.NewStorage(ctx, cfg.OSS)
	if err != nil {
		return err
	}
	urlResolver := platformoss.NewURLResolver(platformoss.URLResolverConfig{
		PublicBaseURL: strings.TrimSpace(cfg.OSS.PublicEndpoint),
		Styles: platformoss.URLStyleConfig{
			Card4x3:      cfg.OSS.Styles.Card4x3,
			SquareSmall:  cfg.OSS.Styles.SquareSmall,
			SquareMedium: cfg.OSS.Styles.SquareMed,
			DetailLarge:  cfg.OSS.Styles.DetailLarge,
		},
	})

	officialModuleRepo := officialrepo.NewRepository(officialdb.New(pool))
	readmodelModuleRepo := readmodelrepo.NewRepository(readmodeldb.New(pool))
	officialCatalog := officialapp.NewCatalogService(officialModuleRepo, nil)
	officialPromptSvc := officialapp.NewPromptService(officialModuleRepo)
	customKeywordRepo := customrepo.NewRepository(customdb.New(pool))
	customKeywordSvc := customapp.NewService(customKeywordRepo, urlResolver)
	profileSvc := profileapp.NewService(profilerepo.NewRepository(profiledb.New(pool)), urlResolver)
	publishingSvc := publishingapp.NewService(pool, officialCatalog, nil).WithCustomKeywords(customKeywordRepo)
	publishSessionSvc, err := publishingapp.NewUploadSessionService(pool, publishingSvc, objectStorage, publishingapp.UploadSessionServiceConfig{
		Bucket:           cfg.OSS.Bucket,
		UploadPrefix:     cfg.OSS.UploadPrefix,
		PutPresignExpire: cfg.OSS.PutPresignExpire,
	})
	if err != nil {
		return err
	}
	readmodelSvc := readmodelapp.NewService(readmodelModuleRepo, urlResolver, officialCatalog, nil).WithCustomKeywords(customKeywordSvc)
	reactionSvc := socialapp.NewReactionService(pool)
	notificationSvc := socialapp.NewNotificationService(pool, urlResolver, nil)

	// 组装 Gin router
	ginMode := ginModeFromEnv(cfg.App.Env)
	router := server.NewRouter(server.Options{
		Logger: logger,
		V1: &v1.Handler{
			Signup:             authM.Signup,
			Login:              authM.Login,
			Reset:              authM.Reset,
			OfficialPrompts:    officialPromptSvc,
			PublishingSessions: publishSessionSvc,
			CustomKeywords:     customKeywordSvc,
			Profile:            profileSvc,
			ReadModel:          readmodelSvc,
			SocialReactions:    reactionSvc,
			Notifications:      notificationSvc,
		},
		GinMode:             ginMode,
		RateLimitRPS:        defaultRateLimitRPS,
		RateLimitBurst:      defaultRateLimitBurst,
		AccessTokenVerifier: authM.Token,
	})

	// 启动 http.Server
	srv := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      router,
		ReadTimeout:  defaultHTTPReadTimeout,
		WriteTimeout: defaultHTTPWriteTimeout,
		IdleTimeout:  defaultHTTPIdleTimeout,
	}
	// 启动服务 goroutine，用 errCh 接收服务启动错误，并带回主 goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	// 在主 goroutine 监听中断信号，关闭服务

	// 基于传入的 ctx 派生一个新的 ctx
	// 当收到中断信号 Ctrl+C（os.Interrupt）或 SIGTERM 时，新的 ctx.Done() 会被触发
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 两种关闭
	select {
	// 服务启动错误
	case err := <-errCh:
		// http.ErrServerClosed 表示“正常关闭”，返回nil
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	// 收到中断信号
	case <-ctx.Done():
		// 此时 ctx 已被取消，所以用一个新的 Background 再套 timeout
		// timeout 避免无法关闭时卡住
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_ = srv.Shutdown(shutdownCtx)

		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// 根据配置选择 gin 运行模式
func ginModeFromEnv(env string) string {
	e := strings.ToLower(strings.TrimSpace(env))
	switch e {
	case "prod", "production":
		return gin.ReleaseMode
	case "test", "testing":
		return gin.TestMode
	default:
		return gin.DebugMode
	}
}
