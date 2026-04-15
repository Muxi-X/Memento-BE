package application

import (
	"context"
	"errors"

	"github.com/google/uuid"

	dofficial "cixing/internal/modules/official/domain"
	"cixing/internal/shared/common"
)

var ErrInvalidPromptKind = errors.New("invalid prompt kind")

type PromptService struct {
	repo dofficial.Repository
}

func NewPromptService(repo dofficial.Repository) *PromptService {
	return &PromptService{repo: repo}
}

type PromptOutput struct {
	ID      uuid.UUID
	Kind    dofficial.PromptKind
	Content string
}

func (s *PromptService) Draw(ctx context.Context, keywordID uuid.UUID, kind string) (*PromptOutput, error) {
	promptKind, err := parsePromptKind(kind)
	if err != nil {
		return nil, err
	}

	prompt, err := s.repo.DrawRandomPrompt(ctx, keywordID, promptKind)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, common.ErrNotFound
		}
		return nil, err
	}

	return &PromptOutput{
		ID:      prompt.ID,
		Kind:    prompt.Kind,
		Content: prompt.Content,
	}, nil
}

func parsePromptKind(kind string) (dofficial.PromptKind, error) {
	switch dofficial.PromptKind(kind) {
	case dofficial.PromptKindIntuition, dofficial.PromptKindStructure, dofficial.PromptKindConcept:
		return dofficial.PromptKind(kind), nil
	default:
		return "", ErrInvalidPromptKind
	}
}
