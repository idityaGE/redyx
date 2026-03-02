package auth

import (
	"context"

	"go.uber.org/zap"
)

// EmailSender defines the interface for sending auth-related emails.
type EmailSender interface {
	// SendOTP sends a 6-digit OTP code to the given email address.
	SendOTP(ctx context.Context, email, code string) error
	// SendPasswordReset sends a password reset token/link to the given email.
	SendPasswordReset(ctx context.Context, email, token string) error
}

// LogSender implements EmailSender by logging emails to stdout via zap.
// Used in development mode instead of a real email provider.
type LogSender struct {
	logger *zap.Logger
}

// NewLogSender creates a LogSender that logs OTP and reset tokens at INFO level.
func NewLogSender(logger *zap.Logger) *LogSender {
	return &LogSender{logger: logger}
}

// SendOTP logs the OTP code to stdout (dev mode).
func (s *LogSender) SendOTP(_ context.Context, email, code string) error {
	s.logger.Info("OTP code generated (dev mode)",
		zap.String("email", email),
		zap.String("code", code),
	)
	return nil
}

// SendPasswordReset logs the reset token to stdout (dev mode).
func (s *LogSender) SendPasswordReset(_ context.Context, email, token string) error {
	s.logger.Info("Password reset token generated (dev mode)",
		zap.String("email", email),
		zap.String("token", token),
	)
	return nil
}
