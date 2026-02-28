package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// passwordResetExpiration is how long a password-reset token remains valid.
const passwordResetExpiration = 1 * time.Hour

// PasswordResetAPI handles the forgot-password / reset-password flow.
type PasswordResetAPI struct {
	userRegistry          registry.UserRegistry
	passwordResetRegistry registry.PasswordResetRegistry
	refreshTokenRegistry  registry.RefreshTokenRegistry
	blacklistService      services.TokenBlacklister
	emailService          services.EmailService
	auditService          services.AuditLogger
	rateLimiter           services.AuthRateLimiter
	publicBaseURL         string
}

// PasswordResetParams holds all dependencies needed by the password-reset API.
type PasswordResetParams struct {
	UserRegistry          registry.UserRegistry
	PasswordResetRegistry registry.PasswordResetRegistry
	RefreshTokenRegistry  registry.RefreshTokenRegistry
	BlacklistService      services.TokenBlacklister
	EmailService          services.EmailService
	AuditService          services.AuditLogger
	RateLimiter           services.AuthRateLimiter
	PublicBaseURL         string
}

// ForgotPasswordRequest is the body for POST /forgot-password.
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest is the body for POST /reset-password.
type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// PasswordReset sets up the password-reset API routes.
func PasswordReset(params PasswordResetParams) func(r chi.Router) {
	api := &PasswordResetAPI{
		userRegistry:          params.UserRegistry,
		passwordResetRegistry: params.PasswordResetRegistry,
		refreshTokenRegistry:  params.RefreshTokenRegistry,
		blacklistService:      params.BlacklistService,
		emailService:          params.EmailService,
		auditService:          params.AuditService,
		rateLimiter:           params.RateLimiter,
		publicBaseURL:         strings.TrimSpace(params.PublicBaseURL),
	}
	return func(r chi.Router) {
		r.With(PasswordResetRateLimitMiddleware(params.RateLimiter)).Post("/forgot-password", api.handleForgotPassword)
		r.Post("/reset-password", api.handleResetPassword)
	}
}

// handleForgotPassword accepts an email and sends a reset link.
// Always responds with success to prevent user-enumeration.
//
// @Summary Request a password reset
// @Description Send a password-reset link to the given email address. Always returns 200 to prevent email enumeration.
// @Tags auth
// @Accept json
// @Produce json
// @Param data body ForgotPasswordRequest true "Email address"
// @Success 200 {object} map[string]string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 429 {string} string "Too Many Requests"
// @Router /forgot-password [post]
func (api *PasswordResetAPI) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	successMsg := "If that email address is registered you will receive a password reset link shortly."

	user, err := api.userRegistry.GetByEmail(r.Context(), DefaultTenantID, req.Email)
	if err != nil {
		if !errors.Is(err, registry.ErrNotFound) {
			// Real DB error — log it, but still return a generic response to prevent enumeration.
			slog.ErrorContext(r.Context(), "failed to look up user by email for password reset",
				"email", req.Email, "error", err)
			api.logAuth(r, "forgot_password_error", nil, false, "user lookup error")
		} else {
			api.logAuth(r, "forgot_password_unknown", nil, false, "email not found")
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": successMsg})
		return
	}
	if user == nil {
		// Defensive: GetByEmail should return ErrNotFound, not nil.
		api.logAuth(r, "forgot_password_unknown", nil, false, "email not found")
		writeJSON(w, http.StatusOK, map[string]string{"message": successMsg})
		return
	}

	api.sendPasswordReset(r, user)
	api.logAuth(r, "forgot_password", &user.ID, true, "")
	writeJSON(w, http.StatusOK, map[string]string{"message": successMsg})
}

// handleResetPassword validates a reset token and updates the user's password.
//
// @Summary Reset password using a token
// @Description Validate a password-reset token and update the user's password. Invalidates all existing sessions.
// @Tags auth
// @Accept json
// @Produce json
// @Param data body ResetPasswordRequest true "Reset token and new password"
// @Success 200 {object} map[string]string "OK"
// @Failure 400 {string} string "Bad Request - missing, invalid, or expired token"
// @Failure 500 {string} string "Internal Server Error"
// @Router /reset-password [post]
func (api *PasswordResetAPI) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Token == "" {
		http.Error(w, "Reset token is required", http.StatusBadRequest)
		return
	}
	if req.NewPassword == "" {
		http.Error(w, "New password is required", http.StatusBadRequest)
		return
	}
	if err := models.ValidatePassword(req.NewPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pr, err := api.passwordResetRegistry.GetByToken(r.Context(), req.Token)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) || errors.Is(err, registry.ErrFieldRequired) {
			http.Error(w, "Invalid or expired reset token", http.StatusBadRequest)
			return
		}
		slog.Error("Failed to look up password-reset token", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if pr.IsExpired() {
		http.Error(w, "Reset token has expired. Please request a new one.", http.StatusBadRequest)
		return
	}

	user, err := api.userRegistry.Get(r.Context(), pr.UserID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			http.Error(w, "User not found", http.StatusBadRequest)
			return
		}
		slog.Error("Failed to look up user for password reset", "user_id", pr.UserID, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := user.SetPassword(req.NewPassword); err != nil {
		slog.Error("Failed to hash new password", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}
	if _, err := api.userRegistry.Update(r.Context(), *user); err != nil {
		slog.Error("Failed to update user password", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	// Consume the token by deleting all reset tokens for this user.
	// The token is considered used-on-delete: once gone, GetByToken returns ErrNotFound.
	// The audit log below records the successful use.
	if err := api.passwordResetRegistry.DeleteByUserID(r.Context(), pr.UserID); err != nil {
		slog.Warn("Failed to clean up password-reset tokens", "user_id", pr.UserID, "error", err)
	}

	// Invalidate all active refresh tokens so existing sessions are terminated.
	api.invalidateUserSessions(r.Context(), user.ID)
	api.sendPasswordChangedNotification(user)

	api.logAuth(r, "password_reset", &user.ID, true, "")
	slog.Info("Password reset successful", "user_id", user.ID, "email", user.Email)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Password reset successful. You can now log in with your new password."})
}

// sendPasswordReset generates a token, stores it, and sends the reset email.
func (api *PasswordResetAPI) sendPasswordReset(r *http.Request, user *models.User) {
	if api.emailService == nil {
		return
	}
	// Invalidate all existing pending reset tokens before issuing a new one.
	if err := api.passwordResetRegistry.DeleteByUserID(r.Context(), user.ID); err != nil {
		slog.Warn("Failed to delete old password-reset tokens", "user_id", user.ID, "error", err)
	}

	token, err := models.GeneratePasswordResetToken()
	if err != nil {
		slog.Error("Failed to generate password-reset token", "user_id", user.ID, "error", err)
		return
	}
	pr := models.PasswordReset{
		UserID:    user.ID,
		TenantID:  DefaultTenantID,
		Email:     user.Email,
		Token:     token,
		ExpiresAt: time.Now().Add(passwordResetExpiration),
	}
	if _, err := api.passwordResetRegistry.Create(r.Context(), pr); err != nil {
		slog.Error("Failed to store password-reset record", "user_id", user.ID, "error", err)
		return
	}

	resetURL := buildPublicURL(api.publicBaseURL, r, "/reset-password", url.Values{"token": {token}})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := api.emailService.SendPasswordResetEmail(ctx, user.Email, user.Name, resetURL); err != nil {
			slog.Error("Failed to send password-reset email", "user_id", user.ID, "error", err)
		}
	}()
}

func (api *PasswordResetAPI) sendPasswordChangedNotification(user *models.User) {
	if api.emailService == nil || user == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := api.emailService.SendPasswordChangedEmail(ctx, user.Email, user.Name, time.Now()); err != nil {
			slog.Error("Failed to send password-changed notification email", "user_id", user.ID, "error", err)
		}
	}()
}

// invalidateUserSessions revokes all refresh tokens for the user and blacklists active access tokens.
func (api *PasswordResetAPI) invalidateUserSessions(ctx context.Context, userID string) {
	if api.refreshTokenRegistry != nil {
		if err := api.refreshTokenRegistry.RevokeByUserID(ctx, userID); err != nil {
			slog.Warn("Failed to revoke refresh tokens for session invalidation", "user_id", userID, "error", err)
		}
	}

	// Blacklist all active access tokens for the user to ensure immediate session invalidation.
	if api.blacklistService != nil {
		if err := api.blacklistService.BlacklistUserTokens(ctx, userID, 2*accessTokenExpiration); err != nil {
			slog.Warn("Failed to blacklist user tokens for session invalidation", "user_id", userID, "error", err)
		}
	}
}

// logAuth is a nil-safe wrapper around the audit service.
func (api *PasswordResetAPI) logAuth(r *http.Request, action string, userID *string, success bool, errMsg string) {
	if api.auditService == nil {
		return
	}
	var ep *string
	if errMsg != "" {
		ep = &errMsg
	}
	tenantID := DefaultTenantID
	api.auditService.LogAuth(r.Context(), action, userID, &tenantID, success, r, ep)
}
