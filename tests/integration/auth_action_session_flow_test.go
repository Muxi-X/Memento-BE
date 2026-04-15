package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	authrepo "cixing/internal/modules/auth/infra/db/repo"
	"cixing/internal/shared/common"
)

func TestAuthSignupTokenIsSingleUseAndRollsBackOnConflict(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	regRepo := authrepo.NewRegistrationRepository(pool)

	tokenHash := "signup-token-hash"
	email := "signup-flow@example.com"

	if _, err := pool.Exec(ctx, `
		INSERT INTO email_action_sessions (purpose, email, token_hash, expires_at)
		VALUES (1, $1, $2, $3)
	`, email, tokenHash, time.Now().UTC().Add(10*time.Minute)); err != nil {
		t.Fatalf("insert signup email_action_session: %v", err)
	}

	userID, err := regRepo.RegisterUserWithSignupToken(ctx, tokenHash, "pw-hash-1")
	if err != nil {
		t.Fatalf("RegisterUserWithSignupToken() error = %v", err)
	}
	if userID == uuid.Nil {
		t.Fatalf("RegisterUserWithSignupToken() user id = nil")
	}

	var gotEmail string
	var gotVerified bool
	var gotPasswordHash string
	if err := pool.QueryRow(ctx, `
		SELECT email, email_verified, password_hash
		FROM user_email_identities
		WHERE user_id = $1
	`, userID).Scan(&gotEmail, &gotVerified, &gotPasswordHash); err != nil {
		t.Fatalf("query user_email_identities: %v", err)
	}
	if gotEmail != email {
		t.Fatalf("created email = %q, want %q", gotEmail, email)
	}
	if !gotVerified {
		t.Fatalf("created email_verified = false, want true")
	}
	if gotPasswordHash != "pw-hash-1" {
		t.Fatalf("created password_hash = %v, want pw-hash-1", gotPasswordHash)
	}

	var usedAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT used_at
		FROM email_action_sessions
		WHERE token_hash = $1
	`, tokenHash).Scan(&usedAt); err != nil {
		t.Fatalf("query used_at after signup complete: %v", err)
	}
	_, err = regRepo.RegisterUserWithSignupToken(ctx, tokenHash, "pw-hash-2")
	if !errors.Is(err, common.ErrNotFound) {
		t.Fatalf("RegisterUserWithSignupToken(reuse) error = %v, want not found", err)
	}

	conflictTokenHash := "signup-token-conflict-hash"
	conflictEmail := "signup-conflict@example.com"
	existingUserID := uuid.MustParse("91111111-1111-1111-1111-111111111111")
	seedUser(t, ctx, pool, existingUserID, conflictEmail, "Existing User")

	if _, err := pool.Exec(ctx, `
		INSERT INTO email_action_sessions (purpose, email, token_hash, expires_at)
		VALUES (1, $1, $2, $3)
	`, conflictEmail, conflictTokenHash, time.Now().UTC().Add(10*time.Minute)); err != nil {
		t.Fatalf("insert conflicting signup email_action_session: %v", err)
	}

	_, err = regRepo.RegisterUserWithSignupToken(ctx, conflictTokenHash, "pw-hash-3")
	if !errors.Is(err, common.ErrConflict) {
		t.Fatalf("RegisterUserWithSignupToken(conflict) error = %v, want conflict", err)
	}

	var conflictUsedAt pgtype.Timestamptz
	if err := pool.QueryRow(ctx, `
		SELECT used_at
		FROM email_action_sessions
		WHERE token_hash = $1
	`, conflictTokenHash).Scan(&conflictUsedAt); err != nil {
		t.Fatalf("query used_at after conflicting signup: %v", err)
	}
	if conflictUsedAt.Valid {
		t.Fatalf("conflicting signup token used_at = %v, want nil", conflictUsedAt)
	}
}

func TestAuthResetTokenIsSingleUseAndRollsBackWhenIdentityMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	repo := authrepo.NewRepository(pool)

	userID := uuid.MustParse("92222222-2222-2222-2222-222222222222")
	email := "reset-flow@example.com"
	tokenHash := "reset-token-hash"
	seedUser(t, ctx, pool, userID, email, "Reset User")

	if _, err := pool.Exec(ctx, `
		UPDATE user_email_identities
		SET password_hash = 'old-hash'
		WHERE user_id = $1
	`, userID); err != nil {
		t.Fatalf("seed old password hash: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO email_action_sessions (purpose, email, token_hash, expires_at)
		VALUES (2, $1, $2, $3)
	`, email, tokenHash, time.Now().UTC().Add(10*time.Minute)); err != nil {
		t.Fatalf("insert reset email_action_session: %v", err)
	}

	gotUserID, err := repo.ResetPasswordWithResetToken(ctx, tokenHash, "new-hash")
	if err != nil {
		t.Fatalf("ResetPasswordWithResetToken() error = %v", err)
	}
	if gotUserID != userID {
		t.Fatalf("ResetPasswordWithResetToken() user id = %s, want %s", gotUserID, userID)
	}

	var gotPasswordHash string
	if err := pool.QueryRow(ctx, `
		SELECT password_hash
		FROM user_email_identities
		WHERE user_id = $1
	`, userID).Scan(&gotPasswordHash); err != nil {
		t.Fatalf("query updated password hash: %v", err)
	}
	if gotPasswordHash != "new-hash" {
		t.Fatalf("updated password_hash = %v, want new-hash", gotPasswordHash)
	}

	var usedAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT used_at
		FROM email_action_sessions
		WHERE token_hash = $1
	`, tokenHash).Scan(&usedAt); err != nil {
		t.Fatalf("query used_at after reset complete: %v", err)
	}
	_, err = repo.ResetPasswordWithResetToken(ctx, tokenHash, "another-hash")
	if !errors.Is(err, common.ErrNotFound) {
		t.Fatalf("ResetPasswordWithResetToken(reuse) error = %v, want not found", err)
	}

	missingIdentityTokenHash := "reset-token-missing-identity"
	missingIdentityEmail := "missing-reset@example.com"
	if _, err := pool.Exec(ctx, `
		INSERT INTO email_action_sessions (purpose, email, token_hash, expires_at)
		VALUES (2, $1, $2, $3)
	`, missingIdentityEmail, missingIdentityTokenHash, time.Now().UTC().Add(10*time.Minute)); err != nil {
		t.Fatalf("insert reset email_action_session missing identity: %v", err)
	}

	_, err = repo.ResetPasswordWithResetToken(ctx, missingIdentityTokenHash, "new-hash")
	if !errors.Is(err, common.ErrNotFound) {
		t.Fatalf("ResetPasswordWithResetToken(missing identity) error = %v, want not found", err)
	}

	var missingIdentityUsedAt pgtype.Timestamptz
	if err := pool.QueryRow(ctx, `
		SELECT used_at
		FROM email_action_sessions
		WHERE token_hash = $1
	`, missingIdentityTokenHash).Scan(&missingIdentityUsedAt); err != nil {
		t.Fatalf("query used_at after missing identity reset: %v", err)
	}
	if missingIdentityUsedAt.Valid {
		t.Fatalf("missing identity reset token used_at = %v, want nil", missingIdentityUsedAt)
	}
}
