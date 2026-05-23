package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	authv1 "github.com/idityaGE/redyx/gen/redyx/auth/v1"
)

// ---------- Hasher Tests ----------

func TestHashPassword_ProducesEncodedHash(t *testing.T) {
	hash, err := HashPassword("mysecretpassword", DefaultParams)
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestHashPassword_DifferentSaltEachTime(t *testing.T) {
	h1, err := HashPassword("same-password", DefaultParams)
	if err != nil {
		t.Fatalf("first HashPassword error: %v", err)
	}
	h2, err := HashPassword("same-password", DefaultParams)
	if err != nil {
		t.Fatalf("second HashPassword error: %v", err)
	}
	if h1 == h2 {
		t.Error("expected different hashes due to random salt, but got identical hashes")
	}
}

func TestVerifyPassword_CorrectPassword(t *testing.T) {
	password := "correcthorsebatterystaple"
	hash, err := HashPassword(password, DefaultParams)
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	valid, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword error: %v", err)
	}
	if !valid {
		t.Error("expected correct password to verify successfully")
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash, err := HashPassword("correct-password", DefaultParams)
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	valid, err := VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword error: %v", err)
	}
	if valid {
		t.Error("expected wrong password to fail verification")
	}
}

func TestVerifyPassword_InvalidHashFormat(t *testing.T) {
	_, err := VerifyPassword("password", "not-a-valid-hash")
	if err == nil {
		t.Error("expected error for invalid hash format, got nil")
	}
}

// ---------- JWT Tests ----------

func TestJWTManager_Generate_ReturnsToken(t *testing.T) {
	mgr := NewJWTManager("test-secret-key", 15*time.Minute)
	token, expiresAt, err := mgr.Generate("user-123", "testuser")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if expiresAt.Before(time.Now()) {
		t.Errorf("expected future expiration, got %v", expiresAt)
	}
}

func TestJWTManager_Generate_ExpiryRespected(t *testing.T) {
	ttl := 30 * time.Minute
	mgr := NewJWTManager("test-secret-key", ttl)
	_, expiresAt, err := mgr.Generate("user-123", "testuser")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	expectedExpiry := time.Now().Add(ttl)
	// Allow 5s clock skew
	if expiresAt.After(expectedExpiry.Add(5*time.Second)) || expiresAt.Before(expectedExpiry.Add(-5*time.Second)) {
		t.Errorf("expiry %v not within 5s of expected %v", expiresAt, expectedExpiry)
	}
}

func TestJWTManager_Generate_DifferentUsers_DifferentTokens(t *testing.T) {
	mgr := NewJWTManager("test-secret-key", 15*time.Minute)
	token1, _, _ := mgr.Generate("user-1", "alice")
	token2, _, _ := mgr.Generate("user-2", "bob")
	if token1 == token2 {
		t.Error("expected different tokens for different users")
	}
}

// ---------- OTP Tests ----------

func testRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestOTPManager_Generate_StoresOTP(t *testing.T) {
	rdb := testRedisClient(t)
	mgr := NewOTPManager(rdb, 5*time.Minute)

	code, err := mgr.Generate(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("expected 6-digit code, got %q (length %d)", code, len(code))
	}
}

func TestOTPManager_Verify_CorrectCode(t *testing.T) {
	rdb := testRedisClient(t)
	mgr := NewOTPManager(rdb, 5*time.Minute)
	ctx := context.Background()

	code, err := mgr.Generate(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	valid, err := mgr.Verify(ctx, "user@example.com", code)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if !valid {
		t.Error("expected correct OTP to verify successfully")
	}
}

func TestOTPManager_Verify_WrongCode(t *testing.T) {
	rdb := testRedisClient(t)
	mgr := NewOTPManager(rdb, 5*time.Minute)
	ctx := context.Background()

	_, err := mgr.Generate(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	valid, err := mgr.Verify(ctx, "user@example.com", "000000")
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if valid {
		t.Error("expected wrong OTP code to fail verification")
	}
}

func TestOTPManager_Verify_SingleUse(t *testing.T) {
	rdb := testRedisClient(t)
	mgr := NewOTPManager(rdb, 5*time.Minute)
	ctx := context.Background()

	code, err := mgr.Generate(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// First verification should succeed
	valid, err := mgr.Verify(ctx, "user@example.com", code)
	if err != nil {
		t.Fatalf("first Verify error: %v", err)
	}
	if !valid {
		t.Fatal("expected first verification to succeed")
	}

	// Second verification with same code should fail (single-use)
	valid, err = mgr.Verify(ctx, "user@example.com", code)
	if err != nil {
		t.Fatalf("second Verify error: %v", err)
	}
	if valid {
		t.Error("expected second verification to fail — OTP is single-use")
	}
}

func TestOTPManager_Verify_NonExistentCode(t *testing.T) {
	rdb := testRedisClient(t)
	mgr := NewOTPManager(rdb, 5*time.Minute)

	valid, err := mgr.Verify(context.Background(), "nobody@example.com", "123456")
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if valid {
		t.Error("expected missing OTP to return false")
	}
}

// ---------- hashRefreshToken Tests ----------

func TestHashRefreshToken_Deterministic(t *testing.T) {
	h1 := hashRefreshToken("some-token-value")
	h2 := hashRefreshToken("some-token-value")
	if h1 != h2 {
		t.Errorf("expected deterministic hash, got %q and %q", h1, h2)
	}
}

func TestHashRefreshToken_DifferentInputs_DifferentHashes(t *testing.T) {
	h1 := hashRefreshToken("token-a")
	h2 := hashRefreshToken("token-b")
	if h1 == h2 {
		t.Error("expected different tokens to produce different hashes")
	}
}

func TestHashRefreshToken_NonEmpty(t *testing.T) {
	h := hashRefreshToken("any-token")
	if h == "" {
		t.Error("expected non-empty hash")
	}
}

// ---------- Server Input Validation Tests ----------

// testServer creates a minimal Server with a Redis-backed OTP manager.
// Database operations will fail — these tests only cover pure validation logic.
func testAuthServer(t *testing.T) *Server {
	t.Helper()
	rdb := testRedisClient(t)
	jwtMgr := NewJWTManager("test-secret", 15*time.Minute)
	otpMgr := NewOTPManager(rdb, 5*time.Minute)
	return NewServer(nil, rdb, jwtMgr, otpMgr, nil, nil, nil, 7*24*time.Hour)
}

func TestRegister_InvalidEmail(t *testing.T) {
	s := testAuthServer(t)

	cases := []struct {
		name  string
		email string
	}{
		{"empty", ""},
		{"no-at", "notanemail"},
		{"no-domain", "user@"},
		{"no-tld", "user@domain"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.Register(context.Background(), &authv1.RegisterRequest{
				Email:    tc.email,
				Username: "validuser",
				Password: "validpassword123",
			})
			if err == nil {
				t.Errorf("expected error for email %q, got nil", tc.email)
			}
		})
	}
}

func TestRegister_InvalidUsername(t *testing.T) {
	s := testAuthServer(t)

	cases := []struct {
		name     string
		username string
	}{
		{"too-short", "ab"},
		{"too-long", "averylongusernamethatexceedslimit"},
		{"with-space", "user name"},
		{"with-dash", "user-name"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.Register(context.Background(), &authv1.RegisterRequest{
				Email:    "valid@example.com",
				Username: tc.username,
				Password: "validpassword123",
			})
			if err == nil {
				t.Errorf("expected error for username %q, got nil", tc.username)
			}
		})
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	s := testAuthServer(t)

	_, err := s.Register(context.Background(), &authv1.RegisterRequest{
		Email:    "valid@example.com",
		Username: "validuser",
		Password: "short",
	})
	if err == nil {
		t.Error("expected error for short password (< 8 chars), got nil")
	}
}

func TestLogin_EmptyEmail(t *testing.T) {
	s := testAuthServer(t)

	_, err := s.Login(context.Background(), &authv1.LoginRequest{
		Email:    "",
		Password: "somepassword",
	})
	if err == nil {
		t.Error("expected error for empty email, got nil")
	}
}

func TestLogin_EmptyPassword(t *testing.T) {
	s := testAuthServer(t)

	_, err := s.Login(context.Background(), &authv1.LoginRequest{
		Email:    "user@example.com",
		Password: "",
	})
	if err == nil {
		t.Error("expected error for empty password, got nil")
	}
}

func TestVerifyOTP_EmptyEmail(t *testing.T) {
	s := testAuthServer(t)

	_, err := s.VerifyOTP(context.Background(), &authv1.VerifyOTPRequest{
		Email: "",
		Code:  "123456",
	})
	if err == nil {
		t.Error("expected error for empty email, got nil")
	}
}

func TestVerifyOTP_EmptyCode(t *testing.T) {
	s := testAuthServer(t)

	_, err := s.VerifyOTP(context.Background(), &authv1.VerifyOTPRequest{
		Email: "user@example.com",
		Code:  "",
	})
	if err == nil {
		t.Error("expected error for empty code, got nil")
	}
}

func TestRefreshToken_EmptyToken(t *testing.T) {
	s := testAuthServer(t)

	_, err := s.RefreshToken(context.Background(), &authv1.RefreshTokenRequest{
		RefreshToken: "",
	})
	if err == nil {
		t.Error("expected error for empty refresh token, got nil")
	}
}

func TestLogout_EmptyToken_ReturnsSuccess(t *testing.T) {
	s := testAuthServer(t)

	// Logout with empty token is idempotent — should succeed
	resp, err := s.Logout(context.Background(), &authv1.LogoutRequest{
		RefreshToken: "",
	})
	if err != nil {
		t.Fatalf("expected success for empty token logout, got error: %v", err)
	}
	if resp == nil {
		t.Error("expected non-nil response")
	}
}

func TestResetPassword_NoFieldsProvided(t *testing.T) {
	s := testAuthServer(t)

	_, err := s.ResetPassword(context.Background(), &authv1.ResetPasswordRequest{
		Email: "",
	})
	if err == nil {
		t.Error("expected error when no email is provided, got nil")
	}
}

func TestResetPassword_OnlyTokenProvided(t *testing.T) {
	s := testAuthServer(t)

	// Token without new_password is invalid
	_, err := s.ResetPassword(context.Background(), &authv1.ResetPasswordRequest{
		Email: "user@example.com",
		Token: "some-token",
		// NewPassword is empty
	})
	if err == nil {
		t.Error("expected error when token is set but new_password is missing, got nil")
	}
}

func TestGoogleOAuth_NoOAuthConfigured(t *testing.T) {
	s := testAuthServer(t) // oauth is nil

	_, err := s.GoogleOAuth(context.Background(), &authv1.GoogleOAuthRequest{
		Code: "some-code",
	})
	if err == nil {
		t.Error("expected error when OAuth is not configured, got nil")
	}
}

func TestGoogleOAuth_EmptyCode(t *testing.T) {
	s := testAuthServer(t)

	_, err := s.GoogleOAuth(context.Background(), &authv1.GoogleOAuthRequest{
		Code: "",
	})
	if err == nil {
		t.Error("expected error for empty OAuth code, got nil")
	}
}
