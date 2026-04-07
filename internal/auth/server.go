package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/idityaGE/redyx/gen/redyx/auth/v1"
	perrors "github.com/idityaGE/redyx/internal/platform/errors"
)

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,20}$`)
)

// Server implements the AuthService gRPC server.
type Server struct {
	authv1.UnimplementedAuthServiceServer

	db         *pgxpool.Pool
	rdb        *goredis.Client
	jwt        *JWTManager
	otp        *OTPManager
	oauth      *OAuthManager
	emailer    EmailSender
	logger     *zap.Logger
	refreshTTL time.Duration
}

// NewServer creates a new auth service server.
func NewServer(
	db *pgxpool.Pool,
	rdb *goredis.Client,
	jwtMgr *JWTManager,
	otpMgr *OTPManager,
	oauthMgr *OAuthManager,
	emailer EmailSender,
	logger *zap.Logger,
	refreshTTL time.Duration,
) *Server {
	return &Server{
		db:         db,
		rdb:        rdb,
		jwt:        jwtMgr,
		otp:        otpMgr,
		oauth:      oauthMgr,
		emailer:    emailer,
		logger:     logger,
		refreshTTL: refreshTTL,
	}
}

// Register creates a new user account with email, username, and password.
func (s *Server) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	// Validate email
	email := strings.TrimSpace(strings.ToLower(req.GetEmail()))
	if !emailRegex.MatchString(email) {
		return nil, fmt.Errorf("invalid email format: %w", perrors.ErrInvalidInput)
	}

	// Validate username
	username := strings.TrimSpace(req.GetUsername())
	if !usernameRegex.MatchString(username) {
		return nil, fmt.Errorf("username must be 3-20 alphanumeric or underscore characters: %w", perrors.ErrInvalidInput)
	}

	// Validate password
	password := req.GetPassword()
	if len(password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters: %w", perrors.ErrInvalidInput)
	}

	// Hash password with argon2id
	passwordHash, err := HashPassword(password, DefaultParams)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	// Insert user into database
	var userID string
	err = s.db.QueryRow(ctx,
		`INSERT INTO users (email, username, password_hash, auth_method, is_verified)
		 VALUES ($1, $2, $3, 'email', false)
		 RETURNING id`,
		email, username, passwordHash,
	).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "idx_users_email") {
			return nil, fmt.Errorf("email already registered: %w", perrors.ErrAlreadyExists)
		}
		if strings.Contains(err.Error(), "idx_users_username") {
			return nil, fmt.Errorf("username already taken: %w", perrors.ErrAlreadyExists)
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	// Generate OTP for email verification
	code, err := s.otp.Generate(ctx, email)
	if err != nil {
		s.logger.Error("failed to generate OTP", zap.Error(err))
		// Don't fail registration — user exists, can request OTP later
	} else {
		if err := s.emailer.SendOTP(ctx, email, code); err != nil {
			s.logger.Error("failed to send OTP", zap.Error(err))
		}
	}

	s.logger.Info("user registered",
		zap.String("user_id", userID),
		zap.String("username", username),
	)

	return &authv1.RegisterResponse{
		UserId:               userID,
		RequiresVerification: true,
	}, nil
}

// Login authenticates a user with email and password.
func (s *Server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.GetEmail()))
	if email == "" {
		return nil, fmt.Errorf("email is required: %w", perrors.ErrInvalidInput)
	}

	password := req.GetPassword()
	if password == "" {
		return nil, fmt.Errorf("password is required: %w", perrors.ErrInvalidInput)
	}

	// Look up user by email
	var userID, username, passwordHash string
	var isVerified bool
	err := s.db.QueryRow(ctx,
		`SELECT id, username, password_hash, is_verified FROM users WHERE email = $1 AND auth_method = 'email'`,
		email,
	).Scan(&userID, &username, &passwordHash, &isVerified)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("invalid email or password: %w", perrors.ErrUnauthenticated)
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	// Verify password
	valid, err := VerifyPassword(password, passwordHash)
	if err != nil {
		return nil, fmt.Errorf("verify password: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("invalid email or password: %w", perrors.ErrUnauthenticated)
	}

	// Check verification status
	if !isVerified {
		return nil, fmt.Errorf("account not verified: %w", perrors.ErrUnauthenticated)
	}

	// Generate tokens
	accessToken, expiresAt, err := s.jwt.Generate(userID, username)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := s.createRefreshToken(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}

	s.logger.Info("user logged in",
		zap.String("user_id", userID),
		zap.String("username", username),
	)

	return &authv1.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    timestamppb.New(expiresAt),
		UserId:       userID,
	}, nil
}

// VerifyOTP verifies a 6-digit email OTP code.
func (s *Server) VerifyOTP(ctx context.Context, req *authv1.VerifyOTPRequest) (*authv1.VerifyOTPResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.GetEmail()))
	if email == "" {
		return nil, fmt.Errorf("email is required: %w", perrors.ErrInvalidInput)
	}

	code := strings.TrimSpace(req.GetCode())
	if code == "" {
		return nil, fmt.Errorf("code is required: %w", perrors.ErrInvalidInput)
	}

	valid, err := s.otp.Verify(ctx, email, code)
	if err != nil {
		return nil, fmt.Errorf("verify otp: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("invalid or expired OTP code: %w", perrors.ErrUnauthenticated)
	}

	// Mark user as verified
	var userID, username string
	err = s.db.QueryRow(ctx,
		`UPDATE users SET is_verified = true, updated_at = now()
		 WHERE email = $1
		 RETURNING id, username`,
		email,
	).Scan(&userID, &username)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("user not found: %w", perrors.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("update user verification: %w", err)
	}

	// Generate tokens (same as login)
	accessToken, expiresAt, err := s.jwt.Generate(userID, username)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := s.createRefreshToken(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}

	s.logger.Info("user verified via OTP",
		zap.String("user_id", userID),
	)

	return &authv1.VerifyOTPResponse{
		Verified:     true,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    timestamppb.New(expiresAt),
	}, nil
}

// RefreshToken issues a new access token using a valid refresh token.
func (s *Server) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	rawToken := req.GetRefreshToken()
	if rawToken == "" {
		return nil, fmt.Errorf("refresh token is required: %w", perrors.ErrInvalidInput)
	}

	tokenHash := hashRefreshToken(rawToken)

	// Look up refresh token
	var userID string
	var expiresAt time.Time
	err := s.db.QueryRow(ctx,
		`SELECT user_id, expires_at FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&userID, &expiresAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("invalid refresh token: %w", perrors.ErrUnauthenticated)
	}
	if err != nil {
		return nil, fmt.Errorf("query refresh token: %w", err)
	}

	// Check expiry
	if time.Now().After(expiresAt) {
		// Clean up expired token
		_, _ = s.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)
		return nil, fmt.Errorf("refresh token expired: %w", perrors.ErrUnauthenticated)
	}

	// Look up user
	var username string
	err = s.db.QueryRow(ctx,
		`SELECT username FROM users WHERE id = $1`,
		userID,
	).Scan(&username)
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	// Delete old refresh token (rotation)
	_, _ = s.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)

	// Generate new tokens
	accessToken, accessExpiresAt, err := s.jwt.Generate(userID, username)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	newRefreshToken, err := s.createRefreshToken(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}

	return &authv1.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    timestamppb.New(accessExpiresAt),
	}, nil
}

// Logout invalidates the user's refresh token.
func (s *Server) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	rawToken := req.GetRefreshToken()
	if rawToken == "" {
		// Idempotent: no token means nothing to invalidate
		return &authv1.LogoutResponse{}, nil
	}

	tokenHash := hashRefreshToken(rawToken)
	_, _ = s.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)

	return &authv1.LogoutResponse{}, nil
}

// ResetPassword initiates or completes a password reset flow.
func (s *Server) ResetPassword(ctx context.Context, req *authv1.ResetPasswordRequest) (*authv1.ResetPasswordResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.GetEmail()))
	if email == "" {
		return nil, fmt.Errorf("email is required: %w", perrors.ErrInvalidInput)
	}

	token := req.GetToken()
	newPassword := req.GetNewPassword()

	// Step 1: Initiate reset (email only, no token/password)
	if token == "" && newPassword == "" {
		return s.initiatePasswordReset(ctx, email)
	}

	// Step 2: Complete reset (token + new password)
	if token != "" && newPassword != "" {
		return s.completePasswordReset(ctx, email, token, newPassword)
	}

	return nil, fmt.Errorf("provide either email only (step 1) or email+token+new_password (step 2): %w", perrors.ErrInvalidInput)
}

func (s *Server) initiatePasswordReset(ctx context.Context, email string) (*authv1.ResetPasswordResponse, error) {
	// Verify user exists (don't reveal non-existence to prevent enumeration)
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND auth_method = 'email')`,
		email,
	).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check user: %w", err)
	}

	if !exists {
		// Return success even for non-existent emails (prevent enumeration)
		return &authv1.ResetPasswordResponse{Completed: false}, nil
	}

	// Generate reset token (32 bytes, base64url encoded)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate reset token: %w", err)
	}
	resetToken := base64.URLEncoding.EncodeToString(tokenBytes)

	// Store in Redis with 1hr TTL
	key := fmt.Sprintf("reset:%s", email)
	if err := s.rdb.Set(ctx, key, resetToken, time.Hour).Err(); err != nil {
		return nil, fmt.Errorf("store reset token: %w", err)
	}

	// Send reset email
	if err := s.emailer.SendPasswordReset(ctx, email, resetToken); err != nil {
		s.logger.Error("failed to send password reset email", zap.Error(err))
	}

	return &authv1.ResetPasswordResponse{Completed: false}, nil
}

func (s *Server) completePasswordReset(ctx context.Context, email, token, newPassword string) (*authv1.ResetPasswordResponse, error) {
	if len(newPassword) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters: %w", perrors.ErrInvalidInput)
	}

	// Verify reset token from Redis
	key := fmt.Sprintf("reset:%s", email)
	storedToken, err := s.rdb.Get(ctx, key).Result()
	if err == goredis.Nil {
		return nil, fmt.Errorf("invalid or expired reset token: %w", perrors.ErrUnauthenticated)
	}
	if err != nil {
		return nil, fmt.Errorf("get reset token: %w", err)
	}

	if storedToken != token {
		return nil, fmt.Errorf("invalid reset token: %w", perrors.ErrUnauthenticated)
	}

	// Hash new password
	passwordHash, err := HashPassword(newPassword, DefaultParams)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	// Update password in database
	result, err := s.db.Exec(ctx,
		`UPDATE users SET password_hash = $1, updated_at = now() WHERE email = $2 AND auth_method = 'email'`,
		passwordHash, email,
	)
	if err != nil {
		return nil, fmt.Errorf("update password: %w", err)
	}
	if result.RowsAffected() == 0 {
		return nil, fmt.Errorf("user not found: %w", perrors.ErrNotFound)
	}

	// Delete reset token
	s.rdb.Del(ctx, key)

	s.logger.Info("password reset completed", zap.String("email", email))

	return &authv1.ResetPasswordResponse{Completed: true}, nil
}

// GoogleOAuth authenticates or registers a user via Google OAuth.
func (s *Server) GoogleOAuth(ctx context.Context, req *authv1.GoogleOAuthRequest) (*authv1.GoogleOAuthResponse, error) {
	if s.oauth == nil {
		return nil, fmt.Errorf("Google OAuth is not configured: %w", perrors.ErrInvalidInput)
	}

	code := req.GetCode()
	if code == "" {
		return nil, fmt.Errorf("authorization code is required: %w", perrors.ErrInvalidInput)
	}

	// Exchange code for Google user info
	googleUser, err := s.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("google oauth exchange: %w", perrors.ErrUnauthenticated)
	}

	// Look up existing user by google_id
	var userID, username string
	err = s.db.QueryRow(ctx,
		`SELECT id, username FROM users WHERE google_id = $1`,
		googleUser.ID,
	).Scan(&userID, &username)

	if err == nil {
		// Existing user — generate tokens and return
		accessToken, expiresAt, err := s.jwt.Generate(userID, username)
		if err != nil {
			return nil, fmt.Errorf("generate access token: %w", err)
		}

		refreshToken, err := s.createRefreshToken(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("create refresh token: %w", err)
		}

		return &authv1.GoogleOAuthResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresAt:    timestamppb.New(expiresAt),
			UserId:       userID,
			IsNewUser:    false,
		}, nil
	}

	if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("query user: %w", err)
	}

	// New user — check if username is provided for registration
	reqUsername := strings.TrimSpace(req.GetUsername())
	if reqUsername == "" {
		// Client needs to ask user for a username
		return &authv1.GoogleOAuthResponse{
			IsNewUser: true,
		}, nil
	}

	// Validate username
	if !usernameRegex.MatchString(reqUsername) {
		return nil, fmt.Errorf("username must be 3-20 alphanumeric or underscore characters: %w", perrors.ErrInvalidInput)
	}

	// Create new user with Google OAuth
	err = s.db.QueryRow(ctx,
		`INSERT INTO users (email, username, auth_method, google_id, is_verified)
		 VALUES ($1, $2, 'google', $3, true)
		 RETURNING id`,
		googleUser.Email, reqUsername, googleUser.ID,
	).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "idx_users_email") {
			return nil, fmt.Errorf("email already registered: %w", perrors.ErrAlreadyExists)
		}
		if strings.Contains(err.Error(), "idx_users_username") {
			return nil, fmt.Errorf("username already taken: %w", perrors.ErrAlreadyExists)
		}
		return nil, fmt.Errorf("insert google user: %w", err)
	}

	// Generate tokens
	accessToken, expiresAt, err := s.jwt.Generate(userID, reqUsername)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := s.createRefreshToken(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}

	s.logger.Info("new user registered via Google OAuth",
		zap.String("user_id", userID),
		zap.String("username", reqUsername),
	)

	return &authv1.GoogleOAuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    timestamppb.New(expiresAt),
		UserId:       userID,
		IsNewUser:    false,
	}, nil
}

// createRefreshToken generates a UUID refresh token, hashes it with SHA-256,
// and stores the hash in the refresh_tokens table.
func (s *Server) createRefreshToken(ctx context.Context, userID string) (string, error) {
	rawToken := uuid.New().String()
	tokenHash := hashRefreshToken(rawToken)

	_, err := s.db.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)`,
		userID, tokenHash, time.Now().Add(s.refreshTTL),
	)
	if err != nil {
		return "", fmt.Errorf("insert refresh token: %w", err)
	}

	return rawToken, nil
}

// hashRefreshToken hashes a refresh token string with SHA-256 and returns hex.
func hashRefreshToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
