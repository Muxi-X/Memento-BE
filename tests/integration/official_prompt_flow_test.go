package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	officialapp "cixing/internal/modules/official/application"
	officialdb "cixing/internal/modules/official/infra/db/gen"
	officialrepo "cixing/internal/modules/official/infra/db/repo"
	"cixing/internal/shared/common"
)

func TestOfficialPromptDrawRejectsInactiveKeyword(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)

	keywordID := uuid.MustParse("52222222-2222-2222-2222-222222222222")
	promptID := uuid.MustParse("53333333-3333-3333-3333-333333333333")

	if _, err := pool.Exec(ctx, `
		INSERT INTO official_keywords (id, text, category, is_active, display_order)
		VALUES ($1, $2, 'emotion', FALSE, 1)
	`, keywordID, "停用关键词"); err != nil {
		t.Fatalf("insert official_keywords: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO official_keyword_prompts (id, keyword_id, kind, content, display_order, is_active)
		VALUES ($1, $2, 'intuition', $3, 1, TRUE)
	`, promptID, keywordID, "still active prompt"); err != nil {
		t.Fatalf("insert official_keyword_prompts: %v", err)
	}

	promptSvc := officialapp.NewPromptService(officialrepo.NewRepository(officialdb.New(pool)))

	_, err := promptSvc.Draw(ctx, keywordID, "intuition")
	if !errors.Is(err, common.ErrNotFound) {
		t.Fatalf("Draw() error = %v, want not found", err)
	}
}
