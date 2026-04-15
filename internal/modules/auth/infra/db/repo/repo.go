package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	dauth "cixing/internal/modules/auth/domain"
	authdb "cixing/internal/modules/auth/infra/db/gen"
	platformpostgres "cixing/internal/platform/postgres"
	"cixing/internal/shared/common"
)

type Repository struct {
	db *pgxpool.Pool
	q  authdb.Querier
}

var _ dauth.Repository = (*Repository)(nil)

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
		q:  authdb.New(db),
	}
}

func (r *Repository) GetEmailIdentityByEmail(ctx context.Context, email string) (dauth.EmailIdentity, error) {
	row, err := r.q.GetEmailIdentityByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dauth.EmailIdentity{}, common.ErrNotFound
		}
		return dauth.EmailIdentity{}, err
	}
	return dauth.EmailIdentity{
		UserID:        row.UserID,
		Email:         row.Email,
		EmailVerified: row.EmailVerified,
		PasswordHash:  textPtr(row.PasswordHash),
	}, nil
}

func (r *Repository) GetUserEmailIdentityByUserID(ctx context.Context, userID uuid.UUID) (dauth.EmailIdentity, error) {
	row, err := r.q.GetUserEmailIdentityByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dauth.EmailIdentity{}, common.ErrNotFound
		}
		return dauth.EmailIdentity{}, err
	}
	return dauth.EmailIdentity{
		UserID:        row.UserID,
		Email:         row.Email,
		EmailVerified: row.EmailVerified,
		PasswordHash:  textPtr(row.PasswordHash),
	}, nil
}

func (r *Repository) CreateUserEmailIdentity(ctx context.Context, userID uuid.UUID, email string, verified bool, passwordHash *string) error {
	_, err := r.q.CreateUserEmailIdentity(ctx, authdb.CreateUserEmailIdentityParams{
		UserID:        userID,
		Email:         email,
		EmailVerified: verified,
		PasswordHash:  toText(passwordHash),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return common.ErrConflict
		}
		return err
	}
	return nil
}

func (r *Repository) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
	_, err := r.q.MarkEmailVerified(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return common.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) SetPasswordHashByUserID(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	_, err := r.q.SetPasswordHashByUserID(ctx, authdb.SetPasswordHashByUserIDParams{
		UserID:       userID,
		PasswordHash: toText(&passwordHash),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return common.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ResetPasswordWithResetToken(ctx context.Context, tokenHash string, passwordHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := platformpostgres.WithTx(ctx, r.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := authdb.New(tx)

		session, err := q.GetEmailActionSessionByHashForUpdate(ctx, tokenHash)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return common.ErrNotFound
			}
			return err
		}
		if dauth.EmailActionPurpose(session.Purpose) != dauth.PurposeResetPassword {
			return common.ErrNotFound
		}

		identity, err := q.GetEmailIdentityByEmail(ctx, session.Email)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return common.ErrNotFound
			}
			return err
		}

		if _, err := q.SetPasswordHashByUserID(ctx, authdb.SetPasswordHashByUserIDParams{
			UserID:       identity.UserID,
			PasswordHash: toText(&passwordHash),
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return common.ErrNotFound
			}
			return err
		}

		if _, err := q.MarkEmailActionSessionUsed(ctx, session.ID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return common.ErrNotFound
			}
			return err
		}

		userID = identity.UserID
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

func (r *Repository) InvalidateEmailActionSessionsByEmailPurpose(ctx context.Context, email string, purpose dauth.EmailActionPurpose) error {
	return r.q.InvalidateEmailActionSessionsByEmailPurpose(ctx, authdb.InvalidateEmailActionSessionsByEmailPurposeParams{
		Email:   email,
		Purpose: int16(purpose),
	})
}

func (r *Repository) CreateEmailActionSession(ctx context.Context, purpose dauth.EmailActionPurpose, email string, tokenHash string, expiresAt time.Time) error {
	_, err := r.q.CreateEmailActionSession(ctx, authdb.CreateEmailActionSessionParams{
		Purpose:   int16(purpose),
		Email:     email,
		TokenHash: tokenHash,
		ExpiresAt: timestamptz(expiresAt),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return common.ErrConflict
		}
		return err
	}
	return nil
}

func (r *Repository) GetEmailActionSessionByHash(ctx context.Context, tokenHash string) (dauth.ActionSession, error) {
	row, err := r.q.GetEmailActionSessionByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dauth.ActionSession{}, common.ErrNotFound
		}
		return dauth.ActionSession{}, err
	}
	return dauth.ActionSession{
		ID:        row.ID,
		Purpose:   dauth.EmailActionPurpose(row.Purpose),
		Email:     row.Email,
		ExpiresAt: row.ExpiresAt.Time,
		UsedAt:    timePtr(row.UsedAt),
	}, nil
}

func (r *Repository) MarkEmailActionSessionUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.q.MarkEmailActionSessionUsed(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return common.ErrNotFound
		}
		return err
	}
	return nil
}

func textPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func toText(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func timePtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
