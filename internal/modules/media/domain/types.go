package media

type Kind string

const (
	KindImage Kind = "image"
	KindAudio Kind = "audio"
)

type AssetStatus string

const (
	AssetStatusPendingUpload AssetStatus = "pending_upload"
	AssetStatusUploaded      AssetStatus = "uploaded"
	AssetStatusProcessing    AssetStatus = "processing"
	AssetStatusReady         AssetStatus = "ready"
	AssetStatusFailed        AssetStatus = "failed"
	AssetStatusDeleted       AssetStatus = "deleted"
)

type VariantStatus string

const (
	VariantStatusPending    VariantStatus = "pending"
	VariantStatusProcessing VariantStatus = "processing"
	VariantStatusReady      VariantStatus = "ready"
	VariantStatusFailed     VariantStatus = "failed"
)

type VariantName string

const (
	VariantNameOriginal    VariantName = "original"
	VariantNameCard4x3     VariantName = "card_4x3"
	VariantNameSquareSmall VariantName = "square_small"
	VariantNameSquareMed   VariantName = "square_medium"
	VariantNameAvatarSmall VariantName = "avatar_small"
	VariantNameAvatarMed   VariantName = "avatar_medium"
)
