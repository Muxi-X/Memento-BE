package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dauth "cixing/internal/modules/auth/domain"
	authdb "cixing/internal/modules/auth/infra/db/gen"
	platformpostgres "cixing/internal/platform/postgres"
	"cixing/internal/shared/common"
)

type RegistrationRepository struct {
	db *pgxpool.Pool
}

var _ dauth.RegistrationRepository = (*RegistrationRepository)(nil)

func NewRegistrationRepository(db *pgxpool.Pool) *RegistrationRepository {
	return &RegistrationRepository{db: db}
}

func (r *RegistrationRepository) RegisterUser(ctx context.Context, email string, passwordHash string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := platformpostgres.WithTx(ctx, r.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := authdb.New(tx)

		createdUserID, err := q.CreateUser(ctx)
		if err != nil {
			return err
		}
		if err := q.InitUserProfile(ctx, createdUserID); err != nil {
			return err
		}
		if err := q.InitUserSettings(ctx, createdUserID); err != nil {
			return err
		}
		if _, err := q.CreateUserEmailIdentity(ctx, authdb.CreateUserEmailIdentityParams{
			UserID:        createdUserID,
			Email:         email,
			EmailVerified: true,
			PasswordHash:  toText(&passwordHash),
		}); err != nil {
			if isUniqueViolation(err) {
				return common.ErrConflict
			}
			return err
		}

		userID = createdUserID
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

func (r *RegistrationRepository) RegisterUserWithSignupToken(ctx context.Context, tokenHash string, passwordHash string) (uuid.UUID, error) {
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
		if dauth.EmailActionPurpose(session.Purpose) != dauth.PurposeSignup {
			return common.ErrNotFound
		}

		createdUserID, err := q.CreateUser(ctx)
		if err != nil {
			return err
		}
		if err := q.InitUserProfile(ctx, createdUserID); err != nil {
			return err
		}
		if err := q.InitUserSettings(ctx, createdUserID); err != nil {
			return err
		}
		if _, err := q.CreateUserEmailIdentity(ctx, authdb.CreateUserEmailIdentityParams{
			UserID:        createdUserID,
			Email:         session.Email,
			EmailVerified: true,
			PasswordHash:  toText(&passwordHash),
		}); err != nil {
			if isUniqueViolation(err) {
				return common.ErrConflict
			}
			return err
		}
		if _, err := q.MarkEmailActionSessionUsed(ctx, session.ID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return common.ErrNotFound
			}
			return err
		}

		userID = createdUserID
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}
