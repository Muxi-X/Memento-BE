package application

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"cixing/internal/modules/readmodel/infra/db/repo"
)

func cursorPtr[T any](v T) *T { return &v }

func cursorFixture(sort string) (publicUploadPage, repo.PublicUploadPosition) {
	tm := time.Date(2026, 9, 21, 3, 4, 5, 123456000, time.UTC)
	p := publicUploadPage{scope: dateScope, scopeID: "2026-09-21", limit: 20,
		query: repo.PublicUploadPageParams{Sort: sort, CutoffAt: tm, Seed: math.Nextafter(0.5, 1), Limit: 21}}
	return p, repo.PublicUploadPosition{ID: uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef"), PublishedAt: tm.Add(-time.Microsecond), RandKey: math.Nextafter(1, 0)}
}

func TestUploadCursorRoundTrip(t *testing.T) {
	for _, scope := range []string{dateScope, keywordScope} {
		for _, sort := range []string{sortLatest, sortRandom} {
			p, last := cursorFixture(sort)
			p.scope = scope
			if scope == keywordScope {
				p.scopeID = uuid.NewString()
			}
			encoded, err := p.encodeCursor(last)
			if err != nil {
				t.Fatal(err)
			}
			c, q, err := decodeUploadCursor(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if c.Scope != scope || c.ScopeID != p.scopeID || q.Sort != sort || !q.CutoffAt.Equal(p.query.CutoffAt) || q.After.ID != last.ID {
				t.Fatalf("round trip lost metadata: %+v %+v", c, q)
			}
			if sort == sortLatest && !q.After.PublishedAt.Equal(last.PublishedAt) {
				t.Fatal("lost timestamp precision")
			}
			if sort == sortRandom && (math.Float64bits(q.Seed) != math.Float64bits(p.query.Seed) || math.Float64bits(q.After.RandKey) != math.Float64bits(last.RandKey)) {
				t.Fatal("lost float precision")
			}
		}
	}
}

func TestUploadCursorInvalid(t *testing.T) {
	p, last := cursorFixture(sortRandom)
	valid, _ := p.encodeCursor(last)
	raw, _ := base64.RawURLEncoding.DecodeString(valid)
	var original map[string]any
	if err := json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{"empty": "", "overlong": strings.Repeat("a", 2049), "illegal": "a+b", "padding": valid + "=", "newline": valid + "\n", "not json": "YQ", "null": "bnVsbA", "array": "W10"}
	mutations := map[string]func(map[string]any){
		"version":                   func(m map[string]any) { m["v"] = 2 },
		"unknown":                   func(m map[string]any) { m["unknown"] = true },
		"scope":                     func(m map[string]any) { m["scope"] = "private" },
		"scope date":                func(m map[string]any) { m["scope_id"] = "2026-02-30" },
		"scope uuid":                func(m map[string]any) { m["scope"] = keywordScope; m["scope_id"] = "bad" },
		"last uuid":                 func(m map[string]any) { m["last_id"] = "bad" },
		"time":                      func(m map[string]any) { m["cutoff_at"] = "bad" },
		"nanoseconds":               func(m map[string]any) { m["cutoff_at"] = "2026-09-21T03:04:05.123456789Z" },
		"truncated fractional time": func(m map[string]any) { m["cutoff_at"] = "2026-09-21T03:04:05.1234560001Z" },
		"missing phase":             func(m map[string]any) { delete(m, "phase") },
		"null phase":                func(m map[string]any) { m["phase"] = nil },
		"phase type":                func(m map[string]any) { m["phase"] = "0" },
		"phase fractional":          func(m map[string]any) { m["phase"] = 0.5 },
		"phase value":               func(m map[string]any) { m["phase"] = 2 },
		"phase conflict":            func(m map[string]any) { m["phase"] = 1 },
		"missing seed":              func(m map[string]any) { delete(m, "seed") },
		"seed number":               func(m map[string]any) { m["seed"] = 0.5 },
		"seed nan":                  func(m map[string]any) { m["seed"] = "NaN" },
		"seed inf":                  func(m map[string]any) { m["seed"] = "+Inf" },
		"seed one":                  func(m map[string]any) { m["seed"] = "1" },
		"seed negative":             func(m map[string]any) { m["seed"] = "-0.1" },
		"key missing":               func(m map[string]any) { delete(m, "last_rand_key") },
		"key nan":                   func(m map[string]any) { m["last_rand_key"] = "NaN" },
		"key one":                   func(m map[string]any) { m["last_rand_key"] = "1" },
		"sort":                      func(m map[string]any) { m["sort"] = "invalid" },
		"random with time":          func(m map[string]any) { m["last_published_at"] = m["cutoff_at"] },
		"random with null time":     func(m map[string]any) { m["last_published_at"] = nil },
	}
	for name, mutate := range mutations {
		copy := make(map[string]any, len(original))
		for k, v := range original {
			copy[k] = v
		}
		mutate(copy)
		data, _ := json.Marshal(copy)
		cases[name] = base64.RawURLEncoding.EncodeToString(data)
	}
	cases["trailing json"] = base64.RawURLEncoding.EncodeToString(append(raw, []byte(" {}")...))
	latest, position := cursorFixture(sortLatest)
	position.PublishedAt = latest.query.CutoffAt.Add(time.Microsecond)
	cases["time after cutoff"], _ = latest.encodeCursor(position)
	for name, encoded := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := decodeUploadCursor(encoded); !errors.Is(err, ErrInvalidCursor) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	// Exactly 2048 ASCII bytes is accepted when it still encodes one valid object.
	data := append(append([]byte{}, raw...), []byte(strings.Repeat(" ", 1536-len(raw)))...)
	if _, _, err := decodeUploadCursor(base64.RawURLEncoding.EncodeToString(data)); err != nil {
		t.Fatalf("2048-byte cursor: %v", err)
	}
}

func TestResolvePublicUploadPage(t *testing.T) {
	p, last := cursorFixture(sortRandom)
	cursor, _ := p.encodeCursor(last)
	for _, tc := range []struct {
		name    string
		options PublicUploadListOptions
		want    error
		field   string
	}{
		{"restore", PublicUploadListOptions{Cursor: &cursor}, nil, ""},
		{"change limit", PublicUploadListOptions{Cursor: &cursor, Limit: cursorPtr(1)}, nil, ""},
		{"matching", PublicUploadListOptions{Cursor: &cursor, Sort: cursorPtr(sortRandom), Seed: &p.query.Seed}, nil, ""},
		{"sort mismatch", PublicUploadListOptions{Cursor: &cursor, Sort: cursorPtr(sortLatest)}, ErrCursorMismatch, ""},
		{"seed mismatch", PublicUploadListOptions{Cursor: &cursor, Seed: cursorPtr(0.2)}, ErrCursorMismatch, ""},
		{"zero limit", PublicUploadListOptions{Limit: cursorPtr(0)}, nil, "limit"},
		{"large limit", PublicUploadListOptions{Limit: cursorPtr(51)}, nil, "limit"},
		{"bad sort", PublicUploadListOptions{Sort: cursorPtr("bad")}, nil, "sort"},
		{"bad seed", PublicUploadListOptions{Sort: cursorPtr(sortRandom), Seed: cursorPtr(math.NaN())}, nil, "seed"},
		{"one seed", PublicUploadListOptions{Sort: cursorPtr(sortRandom), Seed: cursorPtr(1.0)}, nil, "seed"},
		{"ignored seed", PublicUploadListOptions{Seed: cursorPtr(3.0)}, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolvePublicUploadPage(p.scope, p.scopeID, tc.options, nil)
			if tc.field != "" {
				var validation *UploadListValidationError
				if !errors.As(err, &validation) || validation.Field != tc.field {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v want %v", err, tc.want)
			}
			if err == nil && tc.options.Cursor != nil && (got.query.Sort != sortRandom || got.query.Seed != p.query.Seed || !got.query.CutoffAt.Equal(p.query.CutoffAt)) {
				t.Fatal("cursor state changed")
			}
		})
	}
	for _, scope := range []struct{ kind, id string }{{dateScope, "2026-09-20"}, {keywordScope, uuid.NewString()}} {
		if _, err := resolvePublicUploadPage(scope.kind, scope.id, PublicUploadListOptions{Cursor: &cursor}, nil); !errors.Is(err, ErrCursorMismatch) {
			t.Fatal(err)
		}
	}
	got, err := resolvePublicUploadPage(dateScope, p.scopeID, PublicUploadListOptions{}, nil)
	if err != nil || got.limit != 20 || got.query.Limit != 21 || got.query.Sort != sortLatest {
		t.Fatalf("defaults: %+v %v", got, err)
	}
}

func TestPublicUploadPageFinish(t *testing.T) {
	p, last := cursorFixture(sortRandom)
	for _, count := range []int{0, 20, 21} {
		rows := make([]repo.PublicUploadPageRow, count)
		for i := range rows {
			pos := last
			pos.ID = uuid.New()
			rows[i] = repo.PublicUploadPageRow{Card: repo.UploadCard{ID: pos.ID}, Position: pos}
		}
		if count == 21 {
			rows[20].Position.Phase = 1
			rows[20].Position.RandKey = 0.1
		}
		cards, next, more, err := p.finish(rows)
		if err != nil || more != (count > 20) || len(cards) != min(count, 20) || (next != nil) != more {
			t.Fatalf("finish(%d): %v", count, err)
		}
		if more {
			_, q, err := decodeUploadCursor(*next)
			if err != nil || q.After.ID != rows[19].Card.ID || q.After.Phase != 0 {
				t.Fatal("cursor used probe record")
			}
			for _, id := range uploadIDs(cards) {
				if id == rows[20].Card.ID {
					t.Fatal("probe included in interaction query IDs")
				}
			}
		}
	}
}
