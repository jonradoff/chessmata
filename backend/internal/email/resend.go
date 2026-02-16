package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type ResendService struct {
	apiKey      string
	fromEmail   string
	baseURL     string
	frontendURL string
}

type EmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

type EmailResponse struct {
	ID string `json:"id"`
}

func NewResendService(apiKey, frontendURL string) *ResendService {
	return &ResendService{
		apiKey:      apiKey,
		fromEmail:   "Chessmata <noreply@chessmata.metavert.io>",
		baseURL:     "https://api.resend.com",
		frontendURL: frontendURL,
	}
}

func (s *ResendService) SendEmail(to, subject, html string) error {
	reqBody := EmailRequest{
		From:    s.fromEmail,
		To:      []string{to},
		Subject: subject,
		HTML:    html,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal email request: %w", err)
	}

	req, err := http.NewRequest("POST", s.baseURL+"/emails", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Email API error: status %d, body: %s", resp.StatusCode, string(body))
		return fmt.Errorf("email API returned status %d", resp.StatusCode)
	}

	log.Printf("Email sent successfully to %s", to)
	return nil
}

func (s *ResendService) SendVerificationEmail(to, displayName, token string) error {
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.frontendURL, token)

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #1a1a2e; color: #ffffff; margin: 0; padding: 40px 20px;">
    <div style="max-width: 600px; margin: 0 auto; background: linear-gradient(135deg, #1a1a2e 0%%, #16213e 100%%); border: 1px solid rgba(100, 150, 255, 0.3); border-radius: 16px; padding: 40px;">
        <div style="text-align: center; margin-bottom: 30px;">
            <h1 style="color: #ffffff; margin: 0; font-size: 28px;">♔ Chessmata ♚</h1>
            <p style="color: rgba(255, 255, 255, 0.6); margin-top: 8px;">Chess for Humans and AI Agents</p>
        </div>

        <h2 style="color: #ffffff; margin-bottom: 20px;">Verify Your Email</h2>

        <p style="color: rgba(255, 255, 255, 0.8); line-height: 1.6;">
            Hi %s,
        </p>

        <p style="color: rgba(255, 255, 255, 0.8); line-height: 1.6;">
            Thanks for signing up for Chessmata! Please verify your email address by clicking the button below:
        </p>

        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" style="display: inline-block; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 600; font-size: 16px;">Verify Email Address</a>
        </div>

        <p style="color: rgba(255, 255, 255, 0.6); font-size: 14px; line-height: 1.6;">
            If you didn't create an account on Chessmata, you can safely ignore this email.
        </p>

        <p style="color: rgba(255, 255, 255, 0.6); font-size: 14px; line-height: 1.6;">
            This link will expire in 24 hours.
        </p>

        <hr style="border: none; border-top: 1px solid rgba(255, 255, 255, 0.1); margin: 30px 0;">

        <p style="color: rgba(255, 255, 255, 0.4); font-size: 12px; text-align: center;">
            © 2026 Metavert LLC. All rights reserved.
        </p>
    </div>
</body>
</html>
`, displayName, verifyURL)

	return s.SendEmail(to, "Verify your Chessmata account", html)
}

func (s *ResendService) SendPasswordResetEmail(to, displayName, token string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.frontendURL, token)

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #1a1a2e; color: #ffffff; margin: 0; padding: 40px 20px;">
    <div style="max-width: 600px; margin: 0 auto; background: linear-gradient(135deg, #1a1a2e 0%%, #16213e 100%%); border: 1px solid rgba(100, 150, 255, 0.3); border-radius: 16px; padding: 40px;">
        <div style="text-align: center; margin-bottom: 30px;">
            <h1 style="color: #ffffff; margin: 0; font-size: 28px;">♔ Chessmata ♚</h1>
            <p style="color: rgba(255, 255, 255, 0.6); margin-top: 8px;">Chess for Humans and AI Agents</p>
        </div>

        <h2 style="color: #ffffff; margin-bottom: 20px;">Reset Your Password</h2>

        <p style="color: rgba(255, 255, 255, 0.8); line-height: 1.6;">
            Hi %s,
        </p>

        <p style="color: rgba(255, 255, 255, 0.8); line-height: 1.6;">
            We received a request to reset your password. Click the button below to choose a new password:
        </p>

        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" style="display: inline-block; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 600; font-size: 16px;">Reset Password</a>
        </div>

        <p style="color: rgba(255, 255, 255, 0.6); font-size: 14px; line-height: 1.6;">
            If you didn't request a password reset, you can safely ignore this email. Your password will remain unchanged.
        </p>

        <p style="color: rgba(255, 255, 255, 0.6); font-size: 14px; line-height: 1.6;">
            This link will expire in 1 hour.
        </p>

        <hr style="border: none; border-top: 1px solid rgba(255, 255, 255, 0.1); margin: 30px 0;">

        <p style="color: rgba(255, 255, 255, 0.4); font-size: 12px; text-align: center;">
            © 2026 Metavert LLC. All rights reserved.
        </p>
    </div>
</body>
</html>
`, displayName, resetURL)

	return s.SendEmail(to, "Reset your Chessmata password", html)
}
