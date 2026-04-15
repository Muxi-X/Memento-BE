package domain

type CoverSource string

const (
	CoverSourceManual     CoverSource = "manual"
	CoverSourceAutoLatest CoverSource = "auto_latest"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusDeleted  Status = "deleted"
)
