package domain

type ReactionType string

const (
	ReactionTypeInspired  ReactionType = "inspired"
	ReactionTypeResonated ReactionType = "resonated"
)

func ParseReactionType(v string) (ReactionType, bool) {
	t := ReactionType(v)
	return t, t.IsValid()
}

func (t ReactionType) IsValid() bool {
	switch t {
	case ReactionTypeInspired, ReactionTypeResonated:
		return true
	default:
		return false
	}
}

func (t ReactionType) String() string {
	return string(t)
}

type NotificationType string

const (
	NotificationTypeReactionReceived NotificationType = "reaction_received"
)

func ParseNotificationType(v string) (NotificationType, bool) {
	t := NotificationType(v)
	return t, t.IsValid()
}

func (t NotificationType) IsValid() bool {
	switch t {
	case NotificationTypeReactionReceived:
		return true
	default:
		return false
	}
}

func (t NotificationType) String() string {
	return string(t)
}
