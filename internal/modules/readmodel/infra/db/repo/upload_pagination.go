package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	readmodeldb "cixing/internal/modules/readmodel/infra/db/gen"
	"cixing/internal/platform/postgres"
)

type PublicUploadPosition struct {
	ID          uuid.UUID
	PublishedAt time.Time
	RandKey     float64
	Phase       int
}
type PublicUploadPageRow struct {
	Card     UploadCard
	Position PublicUploadPosition
}
type PublicUploadPageParams struct {
	Sort     string
	CutoffAt time.Time
	Seed     float64
	After    *PublicUploadPosition
	Limit    int32
}

func (r *Repository) PublicUploadsCutoff(ctx context.Context) (time.Time, error) {
	value, err := r.q.GetPublicUploadsCutoff(ctx)
	return value.Time, err
}

type publicRangeReader func(context.Context, readmodeldb.Querier, PublicUploadPageParams, int) ([]PublicUploadPageRow, error)

func (r *Repository) readPublicPage(ctx context.Context, p PublicUploadPageParams, read publicRangeReader) ([]PublicUploadPageRow, error) {
	if p.Sort == "latest" {
		return read(ctx, r.q, p, 0)
	}
	if r.pool == nil {
		return nil, errors.New("public upload pagination transaction pool is not configured")
	}
	var result []PublicUploadPageRow
	err := postgres.WithTx(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		q := readmodeldb.New(tx)
		phase := 0
		if p.After != nil {
			phase = p.After.Phase
		}
		rows, err := read(ctx, q, p, phase)
		if err != nil {
			return err
		}
		result = rows
		if phase == 0 && int32(len(rows)) < p.Limit {
			// Crossing the seed resets the key: the low range starts from its beginning.
			p.Limit -= int32(len(rows))
			p.After = nil
			low, err := read(ctx, q, p, 1)
			if err != nil {
				return err
			}
			result = append(result, low...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
func mapPublicPageRow(row readmodeldb.ListPublicUploadsByDateLatestRow, phase int) PublicUploadPageRow {
	return PublicUploadPageRow{
		Card: mapPublicUploadCard(row.ID, row.BizDate, row.KeywordID, row.CoverImageID, row.DisplayText,
			row.CoverHasAudio, row.CoverAudioDurationMs, row.CoverObjectKey, row.ImageCount,
			row.ReactionInspiredCount, row.ReactionResonatedCount, row.CreatedAt),
		Position: PublicUploadPosition{ID: row.ID, PublishedAt: row.PublishedAt.Time, RandKey: row.RandKey, Phase: phase},
	}
}

func (r *Repository) ListPublicUploadsByDatePage(ctx context.Context, bizDate time.Time, params PublicUploadPageParams) ([]PublicUploadPageRow, error) {
	return r.readPublicPage(ctx, params, func(ctx context.Context, q readmodeldb.Querier, p PublicUploadPageParams, phase int) ([]PublicUploadPageRow, error) {
		out := make([]PublicUploadPageRow, 0, p.Limit)
		switch {
		case p.Sort == "latest" && p.After != nil:
			rows, err := q.ListPublicUploadsByDateLatestAfter(ctx, readmodeldb.ListPublicUploadsByDateLatestAfterParams{
				BizDate:  dateOnlyArg(bizDate),
				CutoffAt: pgtype.Timestamptz{Time: p.CutoffAt, Valid: true}, LimitCount: p.Limit,
				LastID:          p.After.ID,
				LastPublishedAt: pgtype.Timestamptz{Time: p.After.PublishedAt, Valid: true},
			})
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				out = append(out, mapPublicPageRow(readmodeldb.ListPublicUploadsByDateLatestRow(row), phase))
			}
		case p.Sort == "latest":
			rows, err := q.ListPublicUploadsByDateLatest(ctx, readmodeldb.ListPublicUploadsByDateLatestParams{
				BizDate:  dateOnlyArg(bizDate),
				CutoffAt: pgtype.Timestamptz{Time: p.CutoffAt, Valid: true}, LimitCount: p.Limit,
			})
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				out = append(out, mapPublicPageRow(readmodeldb.ListPublicUploadsByDateLatestRow(row), phase))
			}
		case phase == 0 && p.After != nil:
			rows, err := q.ListPublicUploadsByDateRandomHighAfter(ctx, readmodeldb.ListPublicUploadsByDateRandomHighAfterParams{
				BizDate:  dateOnlyArg(bizDate),
				CutoffAt: pgtype.Timestamptz{Time: p.CutoffAt, Valid: true}, LimitCount: p.Limit,
				Seed:        p.Seed,
				LastID:      p.After.ID,
				LastRandKey: p.After.RandKey,
			})
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				out = append(out, mapPublicPageRow(readmodeldb.ListPublicUploadsByDateLatestRow(row), phase))
			}
		case phase == 0:
			rows, err := q.ListPublicUploadsByDateRandomHigh(ctx, readmodeldb.ListPublicUploadsByDateRandomHighParams{
				BizDate:  dateOnlyArg(bizDate),
				CutoffAt: pgtype.Timestamptz{Time: p.CutoffAt, Valid: true}, LimitCount: p.Limit,
				Seed: p.Seed,
			})
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				out = append(out, mapPublicPageRow(readmodeldb.ListPublicUploadsByDateLatestRow(row), phase))
			}
		case phase == 1 && p.After != nil:
			rows, err := q.ListPublicUploadsByDateRandomLowAfter(ctx, readmodeldb.ListPublicUploadsByDateRandomLowAfterParams{
				BizDate:  dateOnlyArg(bizDate),
				CutoffAt: pgtype.Timestamptz{Time: p.CutoffAt, Valid: true}, LimitCount: p.Limit,
				Seed:        p.Seed,
				LastID:      p.After.ID,
				LastRandKey: p.After.RandKey,
			})
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				out = append(out, mapPublicPageRow(readmodeldb.ListPublicUploadsByDateLatestRow(row), phase))
			}
		case phase == 1:
			rows, err := q.ListPublicUploadsByDateRandomLow(ctx, readmodeldb.ListPublicUploadsByDateRandomLowParams{
				BizDate:  dateOnlyArg(bizDate),
				CutoffAt: pgtype.Timestamptz{Time: p.CutoffAt, Valid: true}, LimitCount: p.Limit,
				Seed: p.Seed,
			})
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				out = append(out, mapPublicPageRow(readmodeldb.ListPublicUploadsByDateLatestRow(row), phase))
			}
		}
		return out, nil
	})
}

func (r *Repository) ListPublicUploadsByKeywordPage(ctx context.Context, keywordID uuid.UUID, params PublicUploadPageParams) ([]PublicUploadPageRow, error) {
	return r.readPublicPage(ctx, params, func(ctx context.Context, q readmodeldb.Querier, p PublicUploadPageParams, phase int) ([]PublicUploadPageRow, error) {
		out := make([]PublicUploadPageRow, 0, p.Limit)
		switch {
		case p.Sort == "latest" && p.After != nil:
			rows, err := q.ListPublicUploadsByKeywordLatestAfter(ctx, readmodeldb.ListPublicUploadsByKeywordLatestAfterParams{
				KeywordID: keywordID,
				CutoffAt:  pgtype.Timestamptz{Time: p.CutoffAt, Valid: true}, LimitCount: p.Limit,
				LastID:          p.After.ID,
				LastPublishedAt: pgtype.Timestamptz{Time: p.After.PublishedAt, Valid: true},
			})
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				out = append(out, mapPublicPageRow(readmodeldb.ListPublicUploadsByDateLatestRow(row), phase))
			}
		case p.Sort == "latest":
			rows, err := q.ListPublicUploadsByKeywordLatest(ctx, readmodeldb.ListPublicUploadsByKeywordLatestParams{
				KeywordID: keywordID,
				CutoffAt:  pgtype.Timestamptz{Time: p.CutoffAt, Valid: true}, LimitCount: p.Limit,
			})
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				out = append(out, mapPublicPageRow(readmodeldb.ListPublicUploadsByDateLatestRow(row), phase))
			}
		case phase == 0 && p.After != nil:
			rows, err := q.ListPublicUploadsByKeywordRandomHighAfter(ctx, readmodeldb.ListPublicUploadsByKeywordRandomHighAfterParams{
				KeywordID: keywordID,
				CutoffAt:  pgtype.Timestamptz{Time: p.CutoffAt, Valid: true}, LimitCount: p.Limit,
				Seed:        p.Seed,
				LastID:      p.After.ID,
				LastRandKey: p.After.RandKey,
			})
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				out = append(out, mapPublicPageRow(readmodeldb.ListPublicUploadsByDateLatestRow(row), phase))
			}
		case phase == 0:
			rows, err := q.ListPublicUploadsByKeywordRandomHigh(ctx, readmodeldb.ListPublicUploadsByKeywordRandomHighParams{
				KeywordID: keywordID,
				CutoffAt:  pgtype.Timestamptz{Time: p.CutoffAt, Valid: true}, LimitCount: p.Limit,
				Seed: p.Seed,
			})
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				out = append(out, mapPublicPageRow(readmodeldb.ListPublicUploadsByDateLatestRow(row), phase))
			}
		case phase == 1 && p.After != nil:
			rows, err := q.ListPublicUploadsByKeywordRandomLowAfter(ctx, readmodeldb.ListPublicUploadsByKeywordRandomLowAfterParams{
				KeywordID: keywordID,
				CutoffAt:  pgtype.Timestamptz{Time: p.CutoffAt, Valid: true}, LimitCount: p.Limit,
				Seed:        p.Seed,
				LastID:      p.After.ID,
				LastRandKey: p.After.RandKey,
			})
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				out = append(out, mapPublicPageRow(readmodeldb.ListPublicUploadsByDateLatestRow(row), phase))
			}
		case phase == 1:
			rows, err := q.ListPublicUploadsByKeywordRandomLow(ctx, readmodeldb.ListPublicUploadsByKeywordRandomLowParams{
				KeywordID: keywordID,
				CutoffAt:  pgtype.Timestamptz{Time: p.CutoffAt, Valid: true}, LimitCount: p.Limit,
				Seed: p.Seed,
			})
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				out = append(out, mapPublicPageRow(readmodeldb.ListPublicUploadsByDateLatestRow(row), phase))
			}
		}
		return out, nil
	})
}
