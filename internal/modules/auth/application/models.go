package application

type AuthToken struct {
	TokenType   string
	AccessToken string
	ExpiresIn   int64
}
