package application

import (
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	dofficial "cixing/internal/modules/official/domain"
)

func TestRotationPlannerMaintainsSevenAndThirtyDayWindows(t *testing.T) {
	t.Parallel()

	for _, size := range []int{7, 8, 25, 29, 30, 31, 100, 1000} {
		size := size
		t.Run(fmt.Sprintf("%d_keywords", size), func(t *testing.T) {
			t.Parallel()
			sequence := simulateRotation(t, size, 3650, nil)
			assertRotationWindows(t, sequence)
		})
	}
}

func TestRotationPlannerExamplesAndQueueBehavior(t *testing.T) {
	t.Parallel()

	effective := dateOnly(2026, time.January, 1)
	queue := rotationTestQueue(51, 1)
	history := map[time.Time]uuid.UUID{}
	sequence := make([]uuid.UUID, 0, 53)
	modes := make([]rotationDecisionMode, 0, 53)

	for i := 0; i < 53; i++ {
		day := effective.AddDate(0, 0, i)
		t.Logf("repeat test queue: %+v history29: %s", queue, history[day.AddDate(0, 0, -29)])
		t.Logf("queue=%v history29=%s", queue, history[day.AddDate(0, 0, -29)])
		decision, err := planNextDay(day, effective, queue, history)
		if err != nil {
			t.Fatalf("planNextDay(%s) error = %v", day.Format(time.DateOnly), err)
		}
		sequence = append(sequence, decision.KeywordID)
		modes = append(modes, decision.Mode)
		history[day] = decision.KeywordID
		if decision.Mode == rotationModeNormal {
			queue = moveRotationTestKeyword(queue, decision.QueueKeywordID)
		}
	}

	if sequence[29] != sequence[22] {
		t.Fatalf("day 30 keyword = %s, want day 23 keyword %s", sequence[29], sequence[22])
	}
	if modes[29] != rotationModeRepeat {
		t.Fatalf("day 30 mode = %d, want repeat", modes[29])
	}
	if sequence[30] != rotationTestID(30) {
		t.Fatalf("day 31 keyword = %s, want A30 %s", sequence[30], rotationTestID(30))
	}
	if sequence[52] != sequence[45] {
		t.Fatalf("day 53 keyword = %s, want day 46 keyword %s", sequence[52], sequence[45])
	}
	if modes[52] != rotationModeRepeat {
		t.Fatalf("day 53 mode = %d, want repeat", modes[52])
	}
	assertRotationWindows(t, sequence)
}

func TestRotationPlannerTwentyFiveKeywordsRotateNaturally(t *testing.T) {
	t.Parallel()

	effective := dateOnly(2026, time.March, 1)
	queue := rotationTestQueue(25, 1)
	history := map[time.Time]uuid.UUID{}

	for i := 0; i < 100; i++ {
		day := effective.AddDate(0, 0, i)
		t.Logf("repeat test queue: %+v history29: %s", queue, history[day.AddDate(0, 0, -29)])
		t.Logf("queue=%v history29=%s", queue, history[day.AddDate(0, 0, -29)])
		decision, err := planNextDay(day, effective, queue, history)
		if err != nil {
			t.Fatalf("planNextDay(%s) error = %v", day.Format(time.DateOnly), err)
		}
		if decision.Mode != rotationModeNormal {
			t.Fatalf("day %d mode = %d, want natural normal rotation", i+1, decision.Mode)
		}
		want := rotationTestID(i%25 + 1)
		if decision.KeywordID != want {
			t.Fatalf("day %d keyword = %s, want %s", i+1, decision.KeywordID, want)
		}
		history[day] = decision.KeywordID
		queue = moveRotationTestKeyword(queue, decision.QueueKeywordID)
	}
}

func TestRotationPlannerKeepsCoolingCandidateInPlace(t *testing.T) {
	t.Parallel()

	effective := dateOnly(2026, time.February, 1)
	day := effective.AddDate(0, 0, 3)
	queue := rotationTestQueue(3, 1)
	history := map[time.Time]uuid.UUID{
		effective:                  rotationTestID(1),
		effective.AddDate(0, 0, 1): rotationTestID(1),
		effective.AddDate(0, 0, 2): rotationTestID(2),
	}

	before := append([]dofficial.RotationQueueKeyword(nil), queue...)
	decision, err := planNextDay(day, effective, queue, history)
	if err != nil {
		t.Fatalf("planNextDay() error = %v", err)
	}
	if decision.KeywordID != rotationTestID(3) || decision.Mode != rotationModeNormal {
		t.Fatalf("decision = %+v, want normal keyword A3", decision)
	}
	if len(queue) != len(before) {
		t.Fatalf("queue length changed during planning: got %d want %d", len(queue), len(before))
	}
	for i := range before {
		if queue[i] != before[i] {
			t.Fatalf("queue changed during planning at index %d: got %+v want %+v", i, queue[i], before[i])
		}
	}
}

func TestRotationPlannerHandlesRepeatLifecycleAndHistoryGaps(t *testing.T) {
	t.Parallel()

	effective := dateOnly(2026, time.April, 1)
	day := effective.AddDate(0, 0, 29)
	history := map[time.Time]uuid.UUID{}
	for i := 0; i < 29; i++ {
		history[effective.AddDate(0, 0, i)] = rotationTestID(i + 1)
	}

	queue := rotationTestQueue(31, 1)
	queue[0] = dofficial.RotationQueueKeyword{KeywordID: rotationTestID(30), QueuePosition: 1, IsActive: true}
	queue[1] = dofficial.RotationQueueKeyword{KeywordID: rotationTestID(1), QueuePosition: 2, IsActive: true}
	decision, err := planNextDay(day, effective, queue, history)
	if err != nil {
		t.Fatalf("planNextDay(natural repeat) error = %v", err)
	}
	if decision.Mode != rotationModeRepeat || decision.KeywordID != rotationTestID(23) {
		t.Fatalf("natural repeat decision = %+v, want repeat A23", decision)
	}

	queue[22].IsActive = false
	substitute, err := planNextDay(day, effective, queue, history)
	if err != nil {
		t.Fatalf("planNextDay(inactive repeat source) error = %v", err)
	}
	if substitute.Mode != rotationModeRepeat || substitute.KeywordID != rotationTestID(22) {
		t.Fatalf("substitute repeat decision = %+v, want repeat A22", substitute)
	}

	delete(history, effective.AddDate(0, 0, 27))
	if _, err := planNextDay(day, effective, queue, history); !errors.Is(err, ErrRotationHistoryGap) {
		t.Fatalf("planNextDay(history gap) error = %v, want ErrRotationHistoryGap", err)
	}
}

func TestRotationPlannerAdditionsStayBehindExistingQueue(t *testing.T) {
	t.Parallel()

	effective := dateOnly(2026, time.May, 1)
	queue := rotationTestQueue(10, 1)
	history := map[time.Time]uuid.UUID{}
	nextNewID := 11
	firstSeen := make(map[uuid.UUID]int)

	for i := 0; i < 240; i++ {
		day := effective.AddDate(0, 0, i)
		if i == 10 || i == 40 || i == 100 {
			for j := 0; j < 3; j++ {
				queue = append(queue, dofficial.RotationQueueKeyword{
					KeywordID:     rotationTestID(nextNewID),
					QueuePosition: int64(len(queue) + 1),
					IsActive:      true,
				})
				nextNewID++
			}
		}
		t.Logf("repeat test queue: %+v history29: %s", queue, history[day.AddDate(0, 0, -29)])
		t.Logf("queue=%v history29=%s", queue, history[day.AddDate(0, 0, -29)])
		decision, err := planNextDay(day, effective, queue, history)
		if err != nil {
			t.Fatalf("planNextDay(%s) error = %v", day.Format(time.DateOnly), err)
		}
		if _, ok := firstSeen[decision.KeywordID]; !ok {
			firstSeen[decision.KeywordID] = i
		}
		history[day] = decision.KeywordID
		if decision.Mode == rotationModeNormal {
			queue = moveRotationTestKeyword(queue, decision.QueueKeywordID)
		}
	}

	for keywordID, seenAt := range firstSeen {
		var numeric int
		for i := 0; i < 8; i++ {
			numeric = numeric<<8 | int(keywordID[i+8])
		}
		if numeric >= 14 && seenAt < 10 {
			t.Fatalf("new keyword %s appeared at day %d before it was added", keywordID, seenAt)
		}
		if numeric >= 14 && numeric <= 16 {
			for oldID := 1; oldID <= 10; oldID++ {
				if _, ok := firstSeen[rotationTestID(oldID)]; !ok || firstSeen[rotationTestID(oldID)] > seenAt {
					t.Fatalf("new keyword %s appeared before existing keyword A%d", keywordID, oldID)
				}
			}
		}
	}
}

func TestRotationPlannerKeepsOneHundredKeywordIntervalsBounded(t *testing.T) {
	t.Parallel()

	sequence := simulateRotation(t, 100, 1050, nil)
	lastSeen := make(map[uuid.UUID]int, 100)
	maxInterval := 0
	for day, keywordID := range sequence {
		if previous, ok := lastSeen[keywordID]; ok {
			if interval := day - previous; interval > maxInterval {
				maxInterval = interval
			}
		}
		lastSeen[keywordID] = day
	}
	if maxInterval > 105 {
		t.Fatalf("maximum appearance interval = %d days, want <= 105", maxInterval)
	}
}

func simulateRotation(t *testing.T, size int, days int, additions map[int]int) []uuid.UUID {
	t.Helper()

	effective := dateOnly(2026, time.January, 1)
	queue := rotationTestQueue(size, 1)
	history := make(map[time.Time]uuid.UUID, days)
	sequence := make([]uuid.UUID, 0, days)
	nextID := size + 1

	for i := 0; i < days; i++ {
		day := effective.AddDate(0, 0, i)
		if additions != nil {
			for add := 0; add < additions[i]; add++ {
				queue = append(queue, dofficial.RotationQueueKeyword{
					KeywordID:     rotationTestID(nextID),
					QueuePosition: int64(len(queue) + 1),
					IsActive:      true,
				})
				nextID++
			}
		}
		t.Logf("repeat test queue: %+v history29: %s", queue, history[day.AddDate(0, 0, -29)])
		t.Logf("queue=%v history29=%s", queue, history[day.AddDate(0, 0, -29)])
		decision, err := planNextDay(day, effective, queue, history)
		if err != nil {
			t.Fatalf("planNextDay(%s) error = %v", day.Format(time.DateOnly), err)
		}
		sequence = append(sequence, decision.KeywordID)
		history[day] = decision.KeywordID
		if decision.Mode == rotationModeNormal {
			queue = moveRotationTestKeyword(queue, decision.QueueKeywordID)
		}
	}
	return sequence
}

func assertRotationWindows(t *testing.T, sequence []uuid.UUID) {
	t.Helper()
	for start := 0; start+7 <= len(sequence); start++ {
		seen := make(map[uuid.UUID]struct{}, 7)
		for _, keywordID := range sequence[start : start+7] {
			seen[keywordID] = struct{}{}
		}
		if len(seen) != 7 {
			t.Fatalf("7-day window starting at index %d has %d distinct keywords", start, len(seen))
		}
	}
	for start := 0; start+30 <= len(sequence); start++ {
		seen := make(map[uuid.UUID]struct{}, 30)
		for _, keywordID := range sequence[start : start+30] {
			seen[keywordID] = struct{}{}
		}
		if len(seen) >= 30 {
			t.Fatalf("30-day window starting at index %d has no repeat", start)
		}
	}
}

func rotationTestQueue(size int, startID int) []dofficial.RotationQueueKeyword {
	queue := make([]dofficial.RotationQueueKeyword, 0, size)
	for i := 0; i < size; i++ {
		queue = append(queue, dofficial.RotationQueueKeyword{
			KeywordID:     rotationTestID(startID + i),
			QueuePosition: int64(i + 1),
			IsActive:      true,
		})
	}
	return queue
}

func moveRotationTestKeyword(queue []dofficial.RotationQueueKeyword, keywordID uuid.UUID) []dofficial.RotationQueueKeyword {
	for i, item := range queue {
		if item.KeywordID != keywordID {
			continue
		}
		copy(queue[i:], queue[i+1:])
		queue[len(queue)-1] = item
		return queue
	}
	return queue
}

func rotationTestID(value int) uuid.UUID {
	var id uuid.UUID
	binary.BigEndian.PutUint64(id[8:], uint64(value))
	return id
}

func dateOnly(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
func TestRotationPlannerSevenDayCooldownAcrossCalendarBoundaries(t *testing.T) {
	t.Parallel()

	for _, start := range []time.Time{
		dateOnly(2026, time.January, 28),
		dateOnly(2026, time.December, 28),
		dateOnly(2028, time.February, 22),
	} {
		start := start
		t.Run(start.Format(time.DateOnly), func(t *testing.T) {
			t.Parallel()
			queue := rotationTestQueue(7, 1)
			history := map[time.Time]uuid.UUID{}
			var first uuid.UUID

			for offset := 0; offset <= 7; offset++ {
				day := start.AddDate(0, 0, offset)
				decision, err := planNextDay(day, start, queue, history)
				if err != nil {
					t.Fatalf("planNextDay(%s) error = %v", day.Format(time.DateOnly), err)
				}
				if offset == 0 {
					first = decision.KeywordID
				}
				if offset > 0 && offset < 7 && decision.KeywordID == first {
					t.Fatalf("keyword %s repeated after %d days", first, offset)
				}
				if offset == 7 && decision.KeywordID != first {
					t.Fatalf("keyword at D+7 = %s, want %s", decision.KeywordID, first)
				}
				history[day] = decision.KeywordID
				if decision.Mode == rotationModeNormal {
					queue = moveRotationTestKeyword(queue, decision.QueueKeywordID)
				}
			}
		})
	}
}
func TestInitialRotationQueueStartsAfterLastKeywordAndWraps(t *testing.T) {
	t.Parallel()

	keywords := []dofficial.OfficialKeyword{
		{ID: rotationTestID(1), DisplayOrder: 1, IsActive: true},
		{ID: rotationTestID(2), DisplayOrder: 2, IsActive: false},
		{ID: rotationTestID(3), DisplayOrder: 3, IsActive: true},
		{ID: rotationTestID(4), DisplayOrder: 4, IsActive: true},
	}
	got := initialRotationQueue(keywords, dofficial.DailyKeywordAssignment{KeywordID: rotationTestID(4)}, true)
	want := []uuid.UUID{rotationTestID(1), rotationTestID(3), rotationTestID(4)}
	if len(got) != len(want) {
		t.Fatalf("initial queue = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("initial queue[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestRotationPlannerSearchesRepeatSourceBackToDayTwentyNine(t *testing.T) {
	t.Parallel()

	effective := dateOnly(2026, time.June, 1)
	day := effective.AddDate(0, 0, 29)
	history := map[time.Time]uuid.UUID{}
	for i := 0; i < 29; i++ {
		history[effective.AddDate(0, 0, i)] = rotationTestID(i + 1)
	}
	queue := rotationTestQueue(31, 1)
	queue[0] = dofficial.RotationQueueKeyword{KeywordID: rotationTestID(30), QueuePosition: 1, IsActive: true}
	queue[1] = dofficial.RotationQueueKeyword{KeywordID: rotationTestID(1), QueuePosition: 2, IsActive: true}
	for offset := 7; offset <= 28; offset++ {
		keywordID := rotationTestID(30 - offset)
		for i := range queue {
			if queue[i].KeywordID == keywordID {
				queue[i].IsActive = false
			}
		}
	}
	decision, err := planNextDay(day, effective, queue, history)
	if err != nil {
		t.Fatalf("planNextDay(D-29 substitute) error = %v", err)
	}
	if decision.Mode != rotationModeRepeat || decision.KeywordID != rotationTestID(1) {
		t.Fatalf("D-29 substitute = %+v, want repeat A1", decision)
	}

	for i := range queue {
		if queue[i].KeywordID == rotationTestID(1) {
			queue[i].IsActive = false
		}
	}
	if _, err := planNextDay(day, effective, queue, history); !errors.Is(err, ErrRepeatKeywordNotAvailable) {
		t.Fatalf("planNextDay(no repeat substitute) error = %v, want ErrRepeatKeywordNotAvailable", err)
	}
}
func TestRotationPlannerUsesPreEffectiveHistoryForCooldown(t *testing.T) {
	t.Parallel()

	effective := dateOnly(2026, time.July, 1)
	queue := []dofficial.RotationQueueKeyword{
		{KeywordID: rotationTestID(1), QueuePosition: 1, IsActive: true},
		{KeywordID: rotationTestID(2), QueuePosition: 2, IsActive: true},
	}
	history := map[time.Time]uuid.UUID{effective.AddDate(0, 0, -1): rotationTestID(1)}
	decision, err := planNextDay(effective, effective, queue, history)
	if err != nil {
		t.Fatalf("planNextDay() error = %v", err)
	}
	if decision.KeywordID != rotationTestID(2) {
		t.Fatalf("effective-day keyword = %s, want A2 after pre-effective A1 cooldown", decision.KeywordID)
	}
}
