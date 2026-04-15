package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"

	authapp "cixing/internal/modules/auth/application"
	"cixing/internal/transport/http/server/response"
	v1gen "cixing/internal/transport/http/v1/gen"
)

// (POST /v1/auth/login/email/send_code)
func (h *Handler) SendLoginEmailCode(c *gin.Context, _ v1gen.SendLoginEmailCodeParams) {
	var req v1gen.EmailRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Login.SendCode(c.Request.Context(), string(req.Email)); err != nil {
		writeAuthError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// (POST /v1/auth/login/email/verify_code)
func (h *Handler) LoginByEmailCode(c *gin.Context, _ v1gen.LoginByEmailCodeParams) {
	var req v1gen.EmailCodeRequest
	if !bindJSON(c, &req) {
		return
	}
	token, err := h.Login.LoginByCode(c.Request.Context(), string(req.Email), req.Code)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, authTokenFrom(token))
}

// (POST /v1/auth/login/password)
func (h *Handler) LoginByPassword(c *gin.Context, _ v1gen.LoginByPasswordParams) {
	var req v1gen.LoginPasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	token, err := h.Login.LoginByPassword(c.Request.Context(), string(req.Email), req.Password)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, authTokenFrom(token))
}

// (POST /v1/auth/password_reset/complete)
func (h *Handler) ResetComplete(c *gin.Context, _ v1gen.ResetCompleteParams) {
	var req v1gen.ResetCompleteRequest
	if !bindJSON(c, &req) {
		return
	}
	token, err := h.Reset.Complete(c.Request.Context(), req.ResetToken, req.NewPassword)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, authTokenFrom(token))
}

// (POST /v1/auth/password_reset/email/send_code)
func (h *Handler) SendResetEmailCode(c *gin.Context, _ v1gen.SendResetEmailCodeParams) {
	var req v1gen.EmailRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Reset.SendCode(c.Request.Context(), string(req.Email)); err != nil {
		writeAuthError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// (POST /v1/auth/password_reset/email/verify_code)
func (h *Handler) VerifyResetEmailCode(c *gin.Context, _ v1gen.VerifyResetEmailCodeParams) {
	var req v1gen.EmailCodeRequest
	if !bindJSON(c, &req) {
		return
	}
	token, err := h.Reset.VerifyCode(c.Request.Context(), string(req.Email), req.Code)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, v1gen.ResetVerifyResponse{ResetToken: token})
}

// (POST /v1/auth/signup/complete)
func (h *Handler) SignupComplete(c *gin.Context, _ v1gen.SignupCompleteParams) {
	var req v1gen.SignupCompleteRequest
	if !bindJSON(c, &req) {
		return
	}
	token, err := h.Signup.Complete(c.Request.Context(), req.SignupToken, req.Password)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, authTokenFrom(token))
}

// (POST /v1/auth/signup/email/send_code)
func (h *Handler) SendSignupEmailCode(c *gin.Context, _ v1gen.SendSignupEmailCodeParams) {
	var req v1gen.EmailRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Signup.SendCode(c.Request.Context(), string(req.Email)); err != nil {
		writeAuthError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// (POST /v1/auth/signup/email/verify_code)
func (h *Handler) VerifySignupEmailCode(c *gin.Context, _ v1gen.VerifySignupEmailCodeParams) {
	var req v1gen.EmailCodeRequest
	if !bindJSON(c, &req) {
		return
	}
	token, err := h.Signup.VerifyCode(c.Request.Context(), string(req.Email), req.Code)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, v1gen.SignupVerifyResponse{SignupToken: token})
}

func authTokenFrom(token *authapp.AuthToken) v1gen.AuthToken {
	expires := token.ExpiresIn
	return v1gen.AuthToken{
		AccessToken: token.AccessToken,
		ExpiresIn:   &expires,
		TokenType:   v1gen.Bearer,
	}
}
