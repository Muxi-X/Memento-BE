package application

import (
	"context"
	"crypto/rsa"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type JWTConfig struct {
	Issuer        string
	KID           string
	PrivateKeyPEM string
	PublicKeyPEM  string
	AccessTTL     time.Duration
}

type TokenService struct {
	Issuer     string
	KID        string
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	AccessTTL  time.Duration
}

func NewTokenService(cfg JWTConfig) (*TokenService, error) {
	priv, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(cfg.PrivateKeyPEM))
	if err != nil {
		return nil, err
	}
	pub, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cfg.PublicKeyPEM))
	if err != nil {
		return nil, err
	}

	return &TokenService{
		Issuer:     cfg.Issuer,
		KID:        cfg.KID,
		PrivateKey: priv,
		PublicKey:  pub,
		AccessTTL:  cfg.AccessTTL,
	}, nil
}

func (s *TokenService) IssueToken(userID uuid.UUID) (*AuthToken, error) {
	if s == nil {
		return nil, errors.New("token: service is nil")
	}

	now := time.Now()
	accessExp := now.Add(s.AccessTTL)
	claims := jwt.RegisteredClaims{
		Issuer:    s.Issuer,
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(accessExp),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	if s.KID != "" {
		token.Header["kid"] = s.KID
	}

	accessToken, err := token.SignedString(s.PrivateKey)
	if err != nil {
		return nil, err
	}

	return &AuthToken{
		TokenType:   "Bearer",
		AccessToken: accessToken,
		ExpiresIn:   int64(s.AccessTTL.Seconds()),
	}, nil
}

func (s *TokenService) VerifyAccessToken(ctx context.Context, token string) (string, error) {
	_ = ctx
	if s == nil {
		return "", ErrInvalidToken
	}

	parsed, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, ErrInvalidToken
		}
		return s.PublicKey, nil
	})
	if err != nil {
		return "", ErrInvalidToken
	}

	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || !parsed.Valid {
		return "", ErrInvalidToken
	}
	if claims.Issuer != s.Issuer || claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}
