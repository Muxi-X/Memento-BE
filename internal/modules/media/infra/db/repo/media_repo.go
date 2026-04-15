package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dmedia "cixing/internal/modules/media/domain"
	mediadb "cixing/internal/modules/media/infra/db/gen"
	"cixing/internal/shared/common"
)

type Repository struct {
	q mediadb.Querier
}

var _ dmedia.Repository = (*Repository)(nil)

func NewRepository(q mediadb.Querier) *Repository {
	return &Repository{q: q}
}

func (r *Repository) CreateAsset(ctx context.Context, params dmedia.CreateAssetParams) (dmedia.Asset, error) {
	row, err := r.q.CreateMediaAsset(ctx, mediadb.CreateMediaAssetParams{
		OwnerUserID:       params.OwnerUserID,
		MediaKind:         string(params.Kind),
		MimeType:          params.MimeType,
		OriginalObjectKey: params.OriginalObjectKey,
		ByteSize:          params.ByteSize,
		Width:             int4OrNull(params.Width),
		Height:            int4OrNull(params.Height),
		DurationMs:        int4OrNull(params.DurationMS),
		Status:            string(params.Status),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return dmedia.Asset{}, common.ErrConflict
		}
		return dmedia.Asset{}, err
	}
	return mapAsset(row), nil
}

func (r *Repository) GetAssetByID(ctx context.Context, id uuid.UUID) (dmedia.Asset, error) {
	row, err := r.q.GetMediaAssetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dmedia.Asset{}, common.ErrNotFound
		}
		return dmedia.Asset{}, err
	}
	return mapAsset(row), nil
}

func (r *Repository) GetAssetByIDForUpdate(ctx context.Context, id uuid.UUID) (dmedia.Asset, error) {
	row, err := r.q.GetMediaAssetByIDForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dmedia.Asset{}, common.ErrNotFound
		}
		return dmedia.Asset{}, err
	}
	return mapAsset(row), nil
}

func (r *Repository) UpdateAsset(ctx context.Context, params dmedia.UpdateAssetParams) (dmedia.Asset, error) {
	row, err := r.q.UpdateMediaAsset(ctx, mediadb.UpdateMediaAssetParams{
		Width:      int4OrNull(params.Width),
		Height:     int4OrNull(params.Height),
		DurationMs: int4OrNull(params.DurationMS),
		Status:     string(params.Status),
		DeletedAt:  timestamptzOrNull(params.DeletedAt),
		ID:         params.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dmedia.Asset{}, common.ErrNotFound
		}
		return dmedia.Asset{}, err
	}
	return mapAsset(row), nil
}

func (r *Repository) TransitionAssetStatus(ctx context.Context, params dmedia.TransitionAssetStatusParams) (dmedia.Asset, error) {
	row, err := r.q.TransitionMediaAssetStatus(ctx, mediadb.TransitionMediaAssetStatusParams{
		Width:         int4OrNull(params.Width),
		Height:        int4OrNull(params.Height),
		DurationMs:    int4OrNull(params.DurationMS),
		NextStatus:    string(params.NextStatus),
		ID:            params.ID,
		CurrentStatus: string(params.CurrentStatus),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, loadErr := r.GetAssetByID(ctx, params.ID); loadErr != nil {
				return dmedia.Asset{}, loadErr
			}
			return dmedia.Asset{}, dmedia.ErrInvalidAssetStatusTransition
		}
		return dmedia.Asset{}, err
	}
	return mapAsset(row), nil
}

func (r *Repository) SoftDeleteAsset(ctx context.Context, id uuid.UUID, deletedAt time.Time) (dmedia.Asset, error) {
	row, err := r.q.SoftDeleteMediaAsset(ctx, mediadb.SoftDeleteMediaAssetParams{
		DeletedAt: timestamptzOrNull(&deletedAt),
		ID:        id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dmedia.Asset{}, common.ErrNotFound
		}
		return dmedia.Asset{}, err
	}
	return mapAsset(row), nil
}

func (r *Repository) UpsertVariant(ctx context.Context, params dmedia.UpsertVariantParams) (dmedia.Variant, error) {
	row, err := r.q.UpsertMediaVariant(ctx, mediadb.UpsertMediaVariantParams{
		AssetID:     params.AssetID,
		VariantName: string(params.Name),
		ObjectKey:   params.ObjectKey,
		Width:       int4OrNull(params.Width),
		Height:      int4OrNull(params.Height),
		Status:      string(params.Status),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return dmedia.Variant{}, common.ErrConflict
		}
		return dmedia.Variant{}, err
	}
	return mapVariant(row), nil
}

func (r *Repository) GetVariant(ctx context.Context, assetID uuid.UUID, name dmedia.VariantName) (dmedia.Variant, error) {
	row, err := r.q.GetMediaVariantByAssetIDAndName(ctx, mediadb.GetMediaVariantByAssetIDAndNameParams{
		AssetID:     assetID,
		VariantName: string(name),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dmedia.Variant{}, common.ErrNotFound
		}
		return dmedia.Variant{}, err
	}
	return mapVariant(row), nil
}

func (r *Repository) GetVariantForUpdate(ctx context.Context, assetID uuid.UUID, name dmedia.VariantName) (dmedia.Variant, error) {
	row, err := r.q.GetMediaVariantByAssetIDAndNameForUpdate(ctx, mediadb.GetMediaVariantByAssetIDAndNameForUpdateParams{
		AssetID:     assetID,
		VariantName: string(name),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dmedia.Variant{}, common.ErrNotFound
		}
		return dmedia.Variant{}, err
	}
	return mapVariant(row), nil
}

func (r *Repository) ListVariantsByAssetID(ctx context.Context, assetID uuid.UUID) ([]dmedia.Variant, error) {
	rows, err := r.q.ListMediaVariantsByAssetID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]dmedia.Variant, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapVariant(row))
	}
	return out, nil
}

func (r *Repository) UpdateVariant(ctx context.Context, params dmedia.UpdateVariantParams) (dmedia.Variant, error) {
	row, err := r.q.UpdateMediaVariant(ctx, mediadb.UpdateMediaVariantParams{
		ObjectKey:   textOrNull(params.ObjectKey),
		Width:       int4OrNull(params.Width),
		Height:      int4OrNull(params.Height),
		Status:      string(params.Status),
		AssetID:     params.AssetID,
		VariantName: string(params.Name),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dmedia.Variant{}, common.ErrNotFound
		}
		return dmedia.Variant{}, err
	}
	return mapVariant(row), nil
}

func (r *Repository) TransitionVariantStatus(ctx context.Context, params dmedia.TransitionVariantStatusParams) (dmedia.Variant, error) {
	row, err := r.q.TransitionMediaVariantStatus(ctx, mediadb.TransitionMediaVariantStatusParams{
		Width:         int4OrNull(params.Width),
		Height:        int4OrNull(params.Height),
		NextStatus:    string(params.NextStatus),
		AssetID:       params.AssetID,
		VariantName:   string(params.Name),
		CurrentStatus: string(params.CurrentStatus),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if _, loadErr := r.GetVariant(ctx, params.AssetID, params.Name); loadErr != nil {
				return dmedia.Variant{}, loadErr
			}
			return dmedia.Variant{}, dmedia.ErrInvalidVariantStatusTransition
		}
		return dmedia.Variant{}, err
	}
	return mapVariant(row), nil
}

func (r *Repository) CountAssetLiveReferences(ctx context.Context, assetID uuid.UUID) (int64, error) {
	return r.q.CountMediaAssetLiveReferences(ctx, assetID)
}

func mapAsset(row mediadb.MediaAsset) dmedia.Asset {
	return dmedia.Asset{
		ID:                row.ID,
		OwnerUserID:       row.OwnerUserID,
		Kind:              dmedia.Kind(enumString(row.MediaKind)),
		MimeType:          row.MimeType,
		OriginalObjectKey: row.OriginalObjectKey,
		ByteSize:          row.ByteSize,
		Width:             int4Ptr(row.Width),
		Height:            int4Ptr(row.Height),
		DurationMS:        int4Ptr(row.DurationMs),
		Status:            dmedia.AssetStatus(enumString(row.Status)),
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
		DeletedAt:         timePtr(row.DeletedAt),
	}
}

func mapVariant(row mediadb.MediaVariant) dmedia.Variant {
	return dmedia.Variant{
		ID:        row.ID,
		AssetID:   row.AssetID,
		Name:      dmedia.VariantName(row.VariantName),
		ObjectKey: row.ObjectKey,
		Width:     int4Ptr(row.Width),
		Height:    int4Ptr(row.Height),
		Status:    dmedia.VariantStatus(enumString(row.Status)),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

func enumString(v interface{}) string {
	switch e := v.(type) {
	case string:
		return e
	case []byte:
		return string(e)
	case fmt.Stringer:
		return e.String()
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
}

func int4OrNull(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

func int4Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	n := v.Int32
	return &n
}

func timestamptzOrNull(v *time.Time) pgtype.Timestamptz {
	if v == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *v, Valid: true}
}

func timePtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func textOrNull(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}
