package application

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"

	"cixing/internal/modules/readmodel/infra/db/repo"
)

var (
	ErrInvalidCursor  = errors.New("invalid public uploads cursor")
	ErrCursorMismatch = errors.New("public uploads cursor does not match request")
	// Restrict the wire timestamp before parsing: time.Parse truncates fractional
	// seconds beyond nanoseconds, which must not silently change a cursor boundary.
	cursorTimestampPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.[0-9]{1,6})?Z$`)
)

const (
	dateScope            = "official_date"
	keywordScope         = "official_keyword"
	maxUploadCursorBytes = 2048
)

type PublicUploadListOptions struct {
	Sort                  *string
	Limit                 *int
	Seed                  *float64
	Cursor                *string
	IncludeReactionCounts bool
}

type UploadListValidationError struct {
	Field  string
	Rule   string
	Reason string
}

func (e *UploadListValidationError) Error() string { return e.Field + ": " + e.Reason }

type uploadCursor struct {
	Version         int     `json:"v"`
	Scope           string  `json:"scope"`
	ScopeID         string  `json:"scope_id"`
	Sort            string  `json:"sort"`
	CutoffAt        string  `json:"cutoff_at"`
	LastID          string  `json:"last_id"`
	LastPublishedAt *string `json:"last_published_at,omitempty"`
	Seed            *string `json:"seed,omitempty"`
	Phase           *int    `json:"phase,omitempty"`
	LastRandKey     *string `json:"last_rand_key,omitempty"`
}

type publicUploadPage struct {
	scope   string
	scopeID string
	limit   int
	query   repo.PublicUploadPageParams
}

func validRandomKey(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v < 1 }

func parseCursorTime(value string) (time.Time, error) {
	if !cursorTimestampPattern.MatchString(value) {
		return time.Time{}, ErrInvalidCursor
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || t.IsZero() || t.Nanosecond()%1000 != 0 {
		return time.Time{}, ErrInvalidCursor
	}
	return t.UTC(), nil
}

func decodeUploadCursor(encoded string) (uploadCursor, repo.PublicUploadPageParams, error) {
	var c uploadCursor
	var q repo.PublicUploadPageParams
	invalid := func() (uploadCursor, repo.PublicUploadPageParams, error) { return c, q, ErrInvalidCursor }
	if len(encoded) == 0 || len(encoded) > maxUploadCursorBytes {
		return invalid()
	}
	for _, b := range []byte(encoded) {
		if !(b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-' || b == '_') {
			return invalid()
		}
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return invalid()
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return invalid()
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return invalid()
	}
	// Null optional fields must not disguise a field from the other sort mode.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return invalid()
	}
	for _, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return invalid()
		}
	}
	if c.Version != 1 {
		return invalid()
	}
	switch c.Scope {
	case dateScope:
		day, err := time.Parse("2006-01-02", c.ScopeID)
		if err != nil || day.Format("2006-01-02") != c.ScopeID {
			return invalid()
		}
	case keywordScope:
		id, err := uuid.Parse(c.ScopeID)
		if err != nil {
			return invalid()
		}
		c.ScopeID = id.String()
	default:
		return invalid()
	}
	q.CutoffAt, err = parseCursorTime(c.CutoffAt)
	if err != nil {
		return invalid()
	}
	id, err := uuid.Parse(c.LastID)
	if err != nil {
		return invalid()
	}
	q.Sort = c.Sort
	q.After = &repo.PublicUploadPosition{ID: id}
	switch c.Sort {
	case sortLatest:
		if c.LastPublishedAt == nil || c.Seed != nil || c.Phase != nil || c.LastRandKey != nil {
			return invalid()
		}
		q.After.PublishedAt, err = parseCursorTime(*c.LastPublishedAt)
		if err != nil || q.After.PublishedAt.After(q.CutoffAt) {
			return invalid()
		}
	case sortRandom:
		if c.Seed == nil || c.Phase == nil || c.LastRandKey == nil || c.LastPublishedAt != nil {
			return invalid()
		}
		q.Seed, err = strconv.ParseFloat(*c.Seed, 64)
		if err != nil || !validRandomKey(q.Seed) {
			return invalid()
		}
		q.After.RandKey, err = strconv.ParseFloat(*c.LastRandKey, 64)
		if err != nil || !validRandomKey(q.After.RandKey) {
			return invalid()
		}
		q.After.Phase = *c.Phase
		if q.After.Phase != 0 && q.After.Phase != 1 {
			return invalid()
		}
		if (q.After.Phase == 0) != (q.After.RandKey >= q.Seed) {
			return invalid()
		}
	default:
		return invalid()
	}
	return c, q, nil
}

func resolvePublicUploadPage(scope, scopeID string, options PublicUploadListOptions, now func() time.Time) (publicUploadPage, error) {
	p := publicUploadPage{scope: scope, scopeID: scopeID, limit: 20}
	if options.Limit != nil {
		if *options.Limit < 1 || *options.Limit > 50 {
			return p, &UploadListValidationError{"limit", "range", "must be between 1 and 50"}
		}
		p.limit = *options.Limit
	}
	if options.Sort != nil && *options.Sort != sortLatest && *options.Sort != sortRandom {
		return p, &UploadListValidationError{"sort", "oneof", "must be latest or random"}
	}
	if options.Cursor != nil {
		c, q, err := decodeUploadCursor(*options.Cursor)
		if err != nil {
			return p, err
		}
		if c.Scope != scope || c.ScopeID != scopeID || options.Sort != nil && *options.Sort != c.Sort {
			return p, ErrCursorMismatch
		}
		p.query = q
	} else {
		p.query.Sort = sortLatest
		if options.Sort != nil {
			p.query.Sort = *options.Sort
		}
	}
	if p.query.Sort == sortRandom {
		if options.Seed != nil && !validRandomKey(*options.Seed) {
			return p, &UploadListValidationError{"seed", "range", "must be finite and between 0 (inclusive) and 1 (exclusive)"}
		}
		if options.Cursor != nil {
			if options.Seed != nil && *options.Seed != p.query.Seed {
				return p, ErrCursorMismatch
			}
		} else {
			p.query.Seed = normalizeSeed(now, options.Seed)
		}
	}
	p.query.Limit = int32(p.limit + 1)
	return p, nil
}

func (p publicUploadPage) encodeCursor(last repo.PublicUploadPosition) (string, error) {
	c := uploadCursor{Version: 1, Scope: p.scope, ScopeID: p.scopeID, Sort: p.query.Sort,
		CutoffAt: p.query.CutoffAt.UTC().Format(time.RFC3339Nano), LastID: last.ID.String()}
	if p.query.Sort == sortLatest {
		v := last.PublishedAt.UTC().Format(time.RFC3339Nano)
		c.LastPublishedAt = &v
	} else {
		seed := strconv.FormatFloat(p.query.Seed, 'g', -1, 64)
		key := strconv.FormatFloat(last.RandKey, 'g', -1, 64)
		c.Seed, c.LastRandKey, c.Phase = &seed, &key, &last.Phase
	}
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func (p publicUploadPage) finish(rows []repo.PublicUploadPageRow) ([]repo.UploadCard, *string, bool, error) {
	hasMore := len(rows) > p.limit
	if hasMore {
		rows = rows[:p.limit]
	}
	cards := make([]repo.UploadCard, 0, len(rows))
	for _, row := range rows {
		cards = append(cards, row.Card)
	}
	var next *string
	if hasMore {
		cursor, err := p.encodeCursor(rows[len(rows)-1].Position)
		if err != nil {
			return nil, nil, false, err
		}
		next = &cursor
	}
	return cards, next, hasMore, nil
}
