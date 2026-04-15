package application

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dsocial "cixing/internal/modules/social/domain"
	socialdb "cixing/internal/modules/social/infra/db/gen"
	"cixing/internal/modules/social/infra/db/repo"
	platformpostgres "cixing/internal/platform/postgres"
)

var ErrInvalidReactionType = errors.New("invalid reaction type")

type ReactionService struct {
	db *pgxpool.Pool
}

func NewReactionService(db *pgxpool.Pool) *ReactionService {
	return &ReactionService{db: db}
}

func (s *ReactionService) React(ctx context.Context, actorUserID uuid.UUID, uploadID uuid.UUID, reactionType string) error {
	typedReaction, ok := dsocial.ParseReactionType(reactionType)
	if !ok {
		return ErrInvalidReactionType
	}
	return platformpostgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		r := repo.NewRepository(socialdb.New(tx))
		target, err := r.GetVisibleReactionTarget(ctx, uploadID)
		if err != nil {
			return err
		}
		changed, err := r.InsertReaction(ctx, uploadID, actorUserID, typedReaction)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		if err := r.RecomputeReactionCounts(ctx, uploadID); err != nil {
			return err
		}
		if actorUserID == target.AuthorUserID || !target.ReactionNotificationEnabled {
			return nil
		}
		actor, err := r.GetActorSnapshot(ctx, actorUserID)
		if err != nil {
			return err
		}
		return r.CreateNotification(ctx, repo.CreateNotificationInput{
			RecipientUserID:              target.AuthorUserID,
			ActorUserID:                  actorUserID,
			ActorNicknameSnapshot:        actor.Nickname,
			ActorAvatarObjectKeySnapshot: actor.AvatarObjectKey,
			TargetUploadID:               uploadID,
			TargetUploadCoverObjectKey:   target.TargetCoverObjectKey,
			Type:                         dsocial.NotificationTypeReactionReceived,
			ReactionType:                 &typedReaction,
		})
	})
}

func (s *ReactionService) Unreact(ctx context.Context, actorUserID uuid.UUID, uploadID uuid.UUID, reactionType string) error {
	typedReaction, ok := dsocial.ParseReactionType(reactionType)
	if !ok {
		return ErrInvalidReactionType
	}
	return platformpostgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		r := repo.NewRepository(socialdb.New(tx))
		changed, err := r.DeleteReaction(ctx, uploadID, actorUserID, typedReaction)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		return r.RecomputeReactionCounts(ctx, uploadID)
	})
}
