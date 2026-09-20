package application

import (
	"errors"
	"time"

	"github.com/google/uuid"

	dofficial "cixing/internal/modules/official/domain"
	"cixing/internal/shared/common"
)

var (
	ErrRotationNotInitialized    = errors.New("official keyword rotation is not initialized")
	ErrRotationStateInconsistent = errors.New("official keyword rotation state is inconsistent")
	ErrRotationDateNotAllowed    = errors.New("official keyword rotation date is not allowed")
	ErrNoEligibleRotationKeyword = errors.New("no eligible official keyword")
	ErrRepeatKeywordNotAvailable = errors.New("repeat official keyword is not available")
	ErrRotationHistoryGap        = errors.New("official keyword rotation history has a gap")
	ErrRotationCatalogChanged    = errors.New("official keyword catalog changed during a scheduling gap")
)

type rotationDecisionMode uint8

const (
	rotationModeNormal rotationDecisionMode = iota + 1
	rotationModeRepeat
)

type rotationDecision struct {
	KeywordID      uuid.UUID
	QueueKeywordID uuid.UUID
	Mode           rotationDecisionMode
}

func planNextDay(
	day time.Time,
	effectiveDate time.Time,
	queue []dofficial.RotationQueueKeyword,
	history map[time.Time]uuid.UUID,
) (rotationDecision, error) {
	normalizedDay := common.NormalizeBizDate(day)
	normalizedEffective := common.NormalizeBizDate(effectiveDate)
	if normalizedEffective.IsZero() || normalizedDay.Before(normalizedEffective) {
		return rotationDecision{}, ErrRotationHistoryGap
	}

	recent6 := make(map[uuid.UUID]struct{}, 6)
	for current := normalizedDay.AddDate(0, 0, -6); current.Before(normalizedDay); current = current.AddDate(0, 0, 1) {
		keywordID, ok := history[current]
		if !ok {
			if !current.Before(normalizedEffective) {
				return rotationDecision{}, ErrRotationHistoryGap
			}
			continue
		}
		recent6[keywordID] = struct{}{}
	}

	var candidate *dofficial.RotationQueueKeyword
	for i := range queue {
		if !queue[i].IsActive {
			continue
		}
		if _, cooling := recent6[queue[i].KeywordID]; cooling {
			continue
		}
		candidate = &queue[i]
		break
	}
	if candidate == nil {
		return rotationDecision{}, ErrNoEligibleRotationKeyword
	}

	recent29Start := normalizedDay.AddDate(0, 0, -29)
	if !recent29Start.Before(normalizedEffective) {
		distinct := make(map[uuid.UUID]struct{}, 30)
		for current := recent29Start; current.Before(normalizedDay); current = current.AddDate(0, 0, 1) {
			keywordID, ok := history[current]
			if !ok {
				return rotationDecision{}, ErrRotationHistoryGap
			}
			distinct[keywordID] = struct{}{}
		}
		distinct[candidate.KeywordID] = struct{}{}

		if len(distinct) == 30 {
			for offset := 7; offset <= 29; offset++ {
				repeatDate := normalizedDay.AddDate(0, 0, -offset)
				repeatKeywordID, ok := history[repeatDate]
				if !ok {
					return rotationDecision{}, ErrRotationHistoryGap
				}
				if _, cooling := recent6[repeatKeywordID]; cooling {
					continue
				}
				for _, item := range queue {
					if item.KeywordID == repeatKeywordID && item.IsActive {
						return rotationDecision{
							KeywordID:      repeatKeywordID,
							QueueKeywordID: candidate.KeywordID,
							Mode:           rotationModeRepeat,
						}, nil
					}
				}
			}
			return rotationDecision{}, ErrRepeatKeywordNotAvailable
		}
	}

	return rotationDecision{
		KeywordID:      candidate.KeywordID,
		QueueKeywordID: candidate.KeywordID,
		Mode:           rotationModeNormal,
	}, nil
}
