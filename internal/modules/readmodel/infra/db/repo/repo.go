package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dofficial "cixing/internal/modules/official/domain"
	readmodeldb "cixing/internal/modules/readmodel/infra/db/gen"
	"cixing/internal/shared/common"
)

type Repository struct {
	q readmodeldb.Querier
}

func NewRepository(q readmodeldb.Querier) *Repository {
	return &Repository{q: q}
}

type OfficialHomeDay struct {
	BizDate              time.Time
	Keyword              dofficial.OfficialKeyword
	ParticipantUserCount int32
}

func (r *Repository) GetOfficialHomeDay(ctx context.Context, bizDate time.Time) (OfficialHomeDay, error) {
	row, err := r.q.GetOfficialHomeDay(ctx, dateOnlyArg(bizDate))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OfficialHomeDay{}, common.ErrNotFound
		}
		return OfficialHomeDay{}, err
	}
	return OfficialHomeDay{
		BizDate: row.BizDate.Time,
		Keyword: dofficial.OfficialKeyword{
			ID:           row.KeywordID,
			Text:         row.Text,
			Category:     dofficial.KeywordCategory(enumString(row.Category)),
			IsActive:     row.IsActive,
			DisplayOrder: row.DisplayOrder,
		},
		ParticipantUserCount: row.ParticipantUserCount,
	}, nil
}

type MeHomeSummary struct {
	Nickname                string
	AvatarObjectKey         *string
	OfficialImageCount      int32
	CustomImageCount        int32
	UnreadNotificationCount int32
}

type MeHomeCoverImage struct {
	ID        uuid.UUID
	ObjectKey string
}

type MeHomeCustomKeyword struct {
	ID               uuid.UUID
	Text             string
	TargetImageCount *int32
	TotalImageCount  int32
	MyImageCount     int32
	CoverImage       *MeHomeCoverImage
}

func (r *Repository) GetMeHomeSummary(ctx context.Context, userID uuid.UUID) (MeHomeSummary, error) {
	row, err := r.q.GetMeHomeSummary(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MeHomeSummary{}, common.ErrNotFound
		}
		return MeHomeSummary{}, err
	}
	return MeHomeSummary{
		Nickname:                row.Nickname,
		AvatarObjectKey:         textPtr(row.AvatarObjectKey),
		OfficialImageCount:      row.OfficialImageCount,
		CustomImageCount:        row.CustomImageCount,
		UnreadNotificationCount: row.UnreadNotificationCount,
	}, nil
}

func (r *Repository) ListMeHomeCustomKeywords(ctx context.Context, userID uuid.UUID) ([]MeHomeCustomKeyword, error) {
	rows, err := r.q.ListMeHomeCustomKeywords(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]MeHomeCustomKeyword, 0, len(rows))
	for _, row := range rows {
		var cover *MeHomeCoverImage
		if row.CoverImageID.Valid && row.CoverObjectKey.Valid {
			cover = &MeHomeCoverImage{
				ID:        uuid.UUID(row.CoverImageID.Bytes),
				ObjectKey: row.CoverObjectKey.String,
			}
		}
		out = append(out, MeHomeCustomKeyword{
			ID:               row.ID,
			Text:             row.Text,
			TargetImageCount: zeroInt32Ptr(row.TargetImageCount),
			TotalImageCount:  row.TotalImageCount,
			MyImageCount:     row.MyImageCount,
			CoverImage:       cover,
		})
	}
	return out, nil
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

func textPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func anyTextPtr(v interface{}) *string {
	switch vv := v.(type) {
	case nil:
		return nil
	case string:
		if vv == "" {
			return nil
		}
		s := vv
		return &s
	case []byte:
		if len(vv) == 0 {
			return nil
		}
		s := string(vv)
		return &s
	default:
		s := fmt.Sprint(v)
		if s == "" || s == "<nil>" {
			return nil
		}
		return &s
	}
}

func int4Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	i := v.Int32
	return &i
}

func zeroInt32Ptr(v int32) *int32 {
	if v == 0 {
		return nil
	}
	i := v
	return &i
}

func zeroIfInvalidUUID(v pgtype.UUID) uuid.UUID {
	if !v.Valid {
		return uuid.Nil
	}
	return uuid.UUID(v.Bytes)
}

func zeroIfInvalidDate(v pgtype.Date) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time
}

func dateOnlyArg(t time.Time) pgtype.Date {
	return pgtype.Date{Time: common.NormalizeBizDate(t), Valid: true}
}
