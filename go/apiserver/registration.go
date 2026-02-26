package apiserver

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// RegistrationAPI handles self-service user registration and email verification.
type RegistrationAPI struct {
	userRegistry         registry.UserRegistry
	verificationRegistry registry.EmailVerificationRegistry
	emailService         services.EmailService
	auditService         services.AuditLogger
	rateLimiter          services.AuthRateLimiter
	registrationMode     models.RegistrationMode
}

// RegistrationParams holds all dependencies needed by the registration API.
type RegistrationParams struct {
	UserRegistry         registry.UserRegistry
	VerificationRegistry registry.EmailVerificationRegistry
	EmailService         services.EmailService
	AuditService         services.AuditLogger
	RateLimiter          services.AuthRateLimiter
	// RegistrationMode controls how new registrations are processed.
	// Defaults to RegistrationModeOpen when zero value.
	RegistrationMode models.RegistrationMode
}

// RegisterRequest is the body for POST /register.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// ResendVerificationRequest is the body for POST /resend-verification.
type ResendVerificationRequest struct {
	Email string `json:"email"`
}

// Registration sets up the registration API routes.
func Registration(params RegistrationParams) func(r chi.Router) {
	mode := params.RegistrationMode
	if mode == "" {
		mode = models.RegistrationModeOpen
	}
	api := &RegistrationAPI{
		userRegistry:         params.UserRegistry,
		verificationRegistry: params.VerificationRegistry,
		emailService:         params.EmailService,
		auditService:         params.AuditService,
		rateLimiter:          params.RateLimiter,
		registrationMode:     mode,
	}
	return func(r chi.Router) {
		r.With(RegistrationRateLimitMiddleware(params.RateLimiter)).Post("/register", api.handleRegister)
		r.Get("/verify-email", api.handleVerifyEmail)
		r.With(RegistrationRateLimitMiddleware(params.RateLimiter)).Post("/resend-verification", api.handleResendVerification)
	}
}

// handleRegister creates a new inactive user account.
// Behaviour depends on the configured RegistrationMode:
//   - closed    → 403 Forbidden; registration is disabled.
//   - approval  → account created (inactive); admin must activate; no verification email sent.
//   - open      → account created (inactive); verification email sent; activates on token click.
//
// @Summary Register a new user
// @Description Create a new user account. Behaviour depends on the server's registration mode: open (email verification sent), approval (pending admin activation), or closed (403 returned).
// @Tags registration
// @Accept json
// @Produce json
// @Param data body RegisterRequest true "Registration data"
// @Success 200 {object} map[string]string "OK - registration accepted"
// @Failure 400 {string} string "Bad Request"
// @Failure 403 {string} string "Forbidden - registrations are closed"
// @Failure 500 {string} string "Internal Server Error"
// @Router /register [post]
func (api *RegistrationAPI) handleRegister(w http.ResponseWriter, r *http.Request) {
	// Enforce registration mode before processing anything.
	switch api.registrationMode {
	case models.RegistrationModeClosed:
		http.Error(w, "Registrations are currently closed", http.StatusForbidden)
		return
	case models.RegistrationModeOpen, models.RegistrationModeApproval:
		// proceed
	default:
		// treat unknown modes as open to avoid accidental lock-out
		slog.Warn("Unknown registration mode, treating as open", "mode", api.registrationMode)
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)

	if req.Email == "" || req.Password == "" || req.Name == "" {
		http.Error(w, "Email, password, and name are required", http.StatusBadRequest)
		return
	}
	if err := models.ValidatePassword(req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Always respond with success to prevent user enumeration.
	var successMsg string
	if api.registrationMode == models.RegistrationModeApproval {
		successMsg = "Registration successful. Your account is pending administrator approval."
	} else {
		successMsg = "Registration successful. Please check your email to verify your account."
	}

	// Silently ignore duplicate registrations.
	if existing, err := api.userRegistry.GetByEmail(r.Context(), DefaultTenantID, req.Email); err == nil && existing != nil {
		api.logAuth(r, "register_duplicate", nil, false, "email already registered")
		writeJSON(w, http.StatusOK, map[string]string{"message": successMsg})
		return
	}

	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: uuid.New().String()},
			TenantID: DefaultTenantID,
		},
		Email:    req.Email,
		Name:     req.Name,
		Role:     models.UserRoleUser,
		IsActive: false,
	}
	if err := user.SetPassword(req.Password); err != nil {
		http.Error(w, "Failed to process registration", http.StatusInternalServerError)
		return
	}
	// Self-reference: UserID = the user's own ID.
	user.UserID = user.ID

	created, err := api.userRegistry.Create(r.Context(), user)
	if err != nil {
		slog.Error("Failed to create user during registration", "email", req.Email, "error", err)
		http.Error(w, "Failed to create account", http.StatusInternalServerError)
		return
	}

	if api.registrationMode == models.RegistrationModeOpen {
		// Send email verification only in open mode.
		api.sendVerification(r, created)
	} else {
		// Approval mode: log that the account is pending admin approval.
		slog.Info("User registered, pending admin approval", "user_id", created.ID, "email", created.Email)
	}

	api.logAuth(r, "register", &created.ID, true, "")
	slog.Info("User registered", "user_id", created.ID, "email", created.Email, "mode", api.registrationMode)
	writeJSON(w, http.StatusOK, map[string]string{"message": successMsg})
}

// handleVerifyEmail activates a user account when a valid token is presented.
// @Summary Verify email address
// @Description Activate a user account using the verification token sent by email.
// @Tags registration
// @Produce json
// @Param token query string true "Email verification token"
// @Success 200 {object} map[string]string "OK"
// @Failure 400 {string} string "Bad Request - missing, invalid, or expired token"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Router /verify-email [get]
func (api *RegistrationAPI) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Verification token required", http.StatusBadRequest)
		return
	}

	ev, err := api.verificationRegistry.GetByToken(r.Context(), token)
	if err != nil {
		http.Error(w, "Invalid or expired verification token", http.StatusBadRequest)
		return
	}
	if ev.IsVerified() {
		writeJSON(w, http.StatusOK, map[string]string{"message": "Email already verified. You can log in now."})
		return
	}
	if ev.IsExpired() {
		http.Error(w, "Verification token expired. Please request a new one.", http.StatusBadRequest)
		return
	}

	user, err := api.userRegistry.Get(r.Context(), ev.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	user.IsActive = true
	if _, err := api.userRegistry.Update(r.Context(), *user); err != nil {
		slog.Error("Failed to activate user", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to verify email", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	ev.VerifiedAt = &now
	if _, err := api.verificationRegistry.Update(r.Context(), *ev); err != nil {
		slog.Error("Failed to mark verification as used", "id", ev.ID, "error", err)
	}

	api.logAuth(r, "email_verified", &user.ID, true, "")
	slog.Info("Email verified", "user_id", user.ID, "email", user.Email)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Email verified successfully. You can now log in."})
}

// handleResendVerification issues a fresh verification token for an unverified account.
// @Summary Resend verification email
// @Description Issue a new email verification link for an unverified account. Always responds with success to prevent email enumeration.
// @Tags registration
// @Accept json
// @Produce json
// @Param data body ResendVerificationRequest true "Email to resend verification to"
// @Success 200 {object} map[string]string "OK"
// @Failure 400 {string} string "Bad Request"
// @Router /resend-verification [post]
func (api *RegistrationAPI) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	var req ResendVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	successMsg := "If the email exists and is unverified, a new verification link has been sent."
	user, err := api.userRegistry.GetByEmail(r.Context(), DefaultTenantID, req.Email)
	if err != nil || user == nil {
		writeJSON(w, http.StatusOK, map[string]string{"message": successMsg})
		return
	}
	if user.IsActive {
		writeJSON(w, http.StatusOK, map[string]string{"message": "Email already verified."})
		return
	}

	api.sendVerification(r, user)
	writeJSON(w, http.StatusOK, map[string]string{"message": successMsg})
}

// sendVerification generates a token, stores it, and delegates sending to the email service.
func (api *RegistrationAPI) sendVerification(r *http.Request, user *models.User) {
	token, err := models.GenerateVerificationToken()
	if err != nil {
		slog.Error("Failed to generate verification token", "user_id", user.ID, "error", err)
		return
	}
	ev := models.EmailVerification{
		UserID:    user.ID,
		TenantID:  DefaultTenantID,
		Email:     user.Email,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if _, err := api.verificationRegistry.Create(r.Context(), ev); err != nil {
		slog.Error("Failed to store verification record", "user_id", user.ID, "error", err)
		return
	}
	verificationURL := fmt.Sprintf("/verify-email?token=%s", token)
	go func() {
		if err := api.emailService.SendVerificationEmail(user.Email, user.Name, verificationURL); err != nil {
			slog.Error("Failed to send verification email", "user_id", user.ID, "error", err)
		}
	}()
}

// logAuth is a nil-safe wrapper around the audit service.
func (api *RegistrationAPI) logAuth(r *http.Request, action string, userID *string, success bool, errMsg string) {
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

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
