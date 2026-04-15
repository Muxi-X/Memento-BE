package domain

type KeywordCategory string

const (
	KeywordCategoryEmotion  KeywordCategory = "emotion"
	KeywordCategoryColor    KeywordCategory = "color"
	KeywordCategoryShape    KeywordCategory = "shape"
	KeywordCategoryTime     KeywordCategory = "time"
	KeywordCategoryAbstract KeywordCategory = "abstract"
)

type PromptKind string

const (
	PromptKindIntuition PromptKind = "intuition"
	PromptKindStructure PromptKind = "structure"
	PromptKindConcept   PromptKind = "concept"
)
