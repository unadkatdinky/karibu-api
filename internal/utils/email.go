package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// We use Brevo's HTTP API rather than SMTP on purpose.
// Render's free tier blocks outbound SMTP ports (25/465/587), so any
// net/smtp or gomail approach will work locally and silently time out
// in production. This goes over normal HTTPS (443), which is not blocked.

const brevoEndpoint = "https://api.brevo.com/v3/smtp/email"

// httpClient is shared so we reuse connections instead of creating a new
// transport per email. The timeout stops a hung request from blocking a
// registration response indefinitely.
var httpClient = &http.Client{Timeout: 10 * time.Second}

type brevoContact struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type brevoPayload struct {
	Sender      brevoContact   `json:"sender"`
	To          []brevoContact `json:"to"`
	Subject     string         `json:"subject"`
	HTMLContent string         `json:"htmlContent"`
}

// SendEmail delivers a single transactional email through Brevo.
// It returns an error rather than logging-and-swallowing so the caller
// decides whether a failure should block the user's request.
func SendEmail(toEmail, toName, subject, htmlBody string) error {
	apiKey := os.Getenv("BREVO_API_KEY")
	senderEmail := os.Getenv("EMAIL_FROM")
	senderName := os.Getenv("EMAIL_FROM_NAME")

	// Dev fallback: with no API key configured, print to the console
	// instead of failing. This keeps local development working without
	// needing credentials, and matches the old mock behaviour.
	if apiKey == "" {
		log.Println("=======================================================")
		log.Printf("📧 [DEV MODE - no BREVO_API_KEY] would send to: %s", toEmail)
		log.Printf("SUBJECT: %s", subject)
		log.Printf("BODY:\n%s", htmlBody)
		log.Println("=======================================================")
		return nil
	}

	if senderName == "" {
		senderName = "Karibu"
	}

	payload := brevoPayload{
		Sender:      brevoContact{Email: senderEmail, Name: senderName},
		To:          []brevoContact{{Email: toEmail, Name: toName}},
		Subject:     subject,
		HTMLContent: htmlBody,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding email payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, brevoEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building email request: %w", err)
	}
	req.Header.Set("api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending email: %w", err)
	}
	defer resp.Body.Close()

	// Brevo returns 201 Created on success. Anything else means the message
	// was not accepted, and the response body explains why (unverified
	// sender, bad key, daily quota reached).
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("brevo rejected the email (status %d): %s", resp.StatusCode, string(respBody))
	}

	log.Printf("✅ Email sent to %s (%s)", toEmail, subject)
	return nil
}

// ============================================
// TEMPLATES
// ============================================
// Inline styles only — email clients strip <style> blocks and ignore
// external CSS, so every rule has to live on the element itself.

const emailShellOpen = `<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;max-width:480px;margin:0 auto;padding:40px 24px;color:#1a1a1a;">
  <p style="font-size:13px;letter-spacing:0.12em;text-transform:uppercase;color:#8a8a8a;margin:0 0 32px;">Karibu</p>`

const emailShellClose = `
  <p style="font-size:12px;line-height:1.6;color:#8a8a8a;margin:40px 0 0;border-top:1px solid #e8e8e8;padding-top:20px;">
    If you didn't request this, you can safely ignore this email.
  </p>
</div>`

// OTPEmailBody builds the verification-code email.
func OTPEmailBody(code string) string {
	return emailShellOpen + fmt.Sprintf(`
  <h1 style="font-size:24px;font-weight:600;margin:0 0 12px;">Verify your account</h1>
  <p style="font-size:15px;line-height:1.6;color:#4a4a4a;margin:0 0 28px;">
    Enter this code to finish setting up your account. It expires in 15 minutes.
  </p>
  <p style="font-size:32px;font-weight:600;letter-spacing:0.25em;margin:0;padding:20px 0;text-align:center;background:#f7f7f5;border-radius:6px;">%s</p>`, code) + emailShellClose
}

// ResetEmailBody builds the password-reset email.
func ResetEmailBody(resetLink string) string {
	return emailShellOpen + fmt.Sprintf(`
  <h1 style="font-size:24px;font-weight:600;margin:0 0 12px;">Reset your password</h1>
  <p style="font-size:15px;line-height:1.6;color:#4a4a4a;margin:0 0 28px;">
    Click the button below to choose a new password. This link expires in 1 hour.
  </p>
  <a href="%s" style="display:inline-block;background:#1a1a1a;color:#ffffff;text-decoration:none;font-size:15px;font-weight:500;padding:14px 28px;border-radius:6px;">Reset password</a>
  <p style="font-size:13px;line-height:1.6;color:#8a8a8a;margin:28px 0 0;word-break:break-all;">
    Or paste this link into your browser:<br>%s
  </p>`, resetLink, resetLink) + emailShellClose
}