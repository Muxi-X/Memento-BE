package domain

type PublishContextType string

const (
	PublishContextOfficialToday PublishContextType = "official_today"
	PublishContextCustomKeyword PublishContextType = "custom_keyword"
)

type SessionStatus string

const (
	SessionStatusCreated   SessionStatus = "created"
	SessionStatusCommitted SessionStatus = "committed"
	SessionStatusCanceled  SessionStatus = "canceled"
	SessionStatusExpired   SessionStatus = "expired"
)

type ItemStatus string

const (
	ItemStatusPendingUpload ItemStatus = "pending_upload"
	ItemStatusUploaded      ItemStatus = "uploaded"
)

type WorkUploadVisibilityStatus string

const (
	WorkUploadVisibilityProcessing WorkUploadVisibilityStatus = "processing"
	WorkUploadVisibilityVisible    WorkUploadVisibilityStatus = "visible"
	WorkUploadVisibilityHidden     WorkUploadVisibilityStatus = "hidden"
	WorkUploadVisibilityDeleted    WorkUploadVisibilityStatus = "deleted"
)

type MediaKind string

const (
	MediaKindImage MediaKind = "image"
	MediaKindAudio MediaKind = "audio"
)

type MediaAssetStatus string

const (
	MediaAssetStatusPendingUpload MediaAssetStatus = "pending_upload"
	MediaAssetStatusUploaded      MediaAssetStatus = "uploaded"
	MediaAssetStatusProcessing    MediaAssetStatus = "processing"
	MediaAssetStatusReady         MediaAssetStatus = "ready"
	MediaAssetStatusFailed        MediaAssetStatus = "failed"
	MediaAssetStatusDeleted       MediaAssetStatus = "deleted"
)
