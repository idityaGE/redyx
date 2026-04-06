package auth

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"

	"go.uber.org/zap"
	"gopkg.in/gomail.v2"
)

//go:embed template/*.html
var templateFS embed.FS

var (
	otpTemplate   *template.Template
	resetTemplate *template.Template
)

func init() {
	var err error
	otpTemplate, err = template.ParseFS(templateFS, "template/send-otp.html")
	if err != nil {
		panic(fmt.Sprintf("failed to parse OTP template: %v", err))
	}
	resetTemplate, err = template.ParseFS(templateFS, "template/send-reset-password.html")
	if err != nil {
		panic(fmt.Sprintf("failed to parse reset password template: %v", err))
	}
}

// EmailSender defines the interface for sending auth-related emails.
type EmailSender interface {
	// SendOTP sends a 6-digit OTP code to the given email address.
	SendOTP(ctx context.Context, email, code string) error
	// SendPasswordReset sends a password reset token/link to the given email.
	SendPasswordReset(ctx context.Context, email, token string) error
}

// Mailer implements EmailSender using gomail.
type Mailer struct {
	dialer *gomail.Dialer
	logger *zap.Logger
}

// NewEmailSender creates a new Mailer instance.
func NewEmailSender(dialer *gomail.Dialer, logger *zap.Logger) *Mailer {
	return &Mailer{
		dialer: dialer,
		logger: logger,
	}
}

// OTPData holds data for OTP email template
type OTPData struct {
	Code string
}

// ResetData holds data for password reset email template
type ResetData struct {
	Token string
}

// renderTemplate executes a template with data and returns the rendered HTML.
func renderTemplate(tmpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}
	return buf.String(), nil
}

// newMessage creates a new gomail message.
func (m *Mailer) newMessage(to, subject, body string) *gomail.Message {
	msg := gomail.NewMessage()
	msg.SetHeader("From", "am44910606@gmail.com")
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/html", body)
	return msg
}

// SendOTP sends an OTP verification email.
func (m *Mailer) SendOTP(ctx context.Context, email, code string) error {
	m.logger.Info("sending OTP email",
		zap.String("email", email),
	)

	body, err := renderTemplate(otpTemplate, OTPData{Code: code})
	if err != nil {
		m.logger.Error("failed to render OTP template", zap.Error(err))
		return err
	}

	msg := m.newMessage(email, "Your Redyx Verification Code", body)

	if err := m.dialer.DialAndSend(msg); err != nil {
		m.logger.Error("failed to send OTP email",
			zap.String("email", email),
			zap.Error(err),
		)
		return fmt.Errorf("failed to send OTP email: %w", err)
	}

	m.logger.Info("OTP email sent successfully", zap.String("email", email))
	return nil
}

// SendPasswordReset sends a password reset email.
func (m *Mailer) SendPasswordReset(ctx context.Context, email, token string) error {
	m.logger.Info("sending password reset email",
		zap.String("email", email),
	)

	body, err := renderTemplate(resetTemplate, ResetData{Token: token})
	if err != nil {
		m.logger.Error("failed to render reset template", zap.Error(err))
		return err
	}

	msg := m.newMessage(email, "Reset Your Redyx Password", body)

	if err := m.dialer.DialAndSend(msg); err != nil {
		m.logger.Error("failed to send password reset email",
			zap.String("email", email),
			zap.Error(err),
		)
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	m.logger.Info("password reset email sent successfully", zap.String("email", email))
	return nil
}
