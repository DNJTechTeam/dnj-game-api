package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
)

// EmailService is a transactional-email client. It targets a Brevo-style HTTP
// API (POST {EMAIL_API_BASE_URL} with an "api-key" header). Swap the body of
// SendEmail if you use a different provider.
type EmailService struct{}

func NewEmailService() interfaces.EmailServiceInterface {
	return &EmailService{}
}

func buildEmailTemplate(title string, transactionalContent string) string {
	appName := common.GetEnv("APP_NAME")
	template := `
	<!DOCTYPE html>
	<html lang="en">
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
			<div style="background-color: #f4f4f4; padding: 20px; border-radius: 5px;">
				<div style="text-align: center; margin-bottom: 20px;">
					<strong style="font-size: 20px; color: #2c3e50;">#app_name</strong>
				</div>
				<h1 style="color: #2c3e50; text-align: center;">#title</h1>
				<div>
					#transactional_content
				</div>
			</div>
		</body>
	</html>`

	html := strings.ReplaceAll(template, "#app_name", appName)
	html = strings.ReplaceAll(html, "#title", title)
	html = strings.ReplaceAll(html, "#transactional_content", transactionalContent)
	return html
}

func (s *EmailService) SendVerificationCodeEmail(ctx context.Context, email string, verificationCode string) error {
	title := "Seu código de verificação"
	transactionalContent := "<p>Use o código abaixo para confirmar sua inscrição:</p>" +
		"<div style=\"text-align: center; margin: 30px 0;\">" +
		"<p style=\"font-size: 32px; font-weight: bold; letter-spacing: 8px;\">" + verificationCode + "</p>" +
		"</div>" +
		"<p style=\"font-size: 12px; color: #666;\">Se você não solicitou este código, pode ignorar este email.</p>"

	htmlContent := buildEmailTemplate(title, transactionalContent)
	return s.SendEmail(email, "Seu código de verificação", htmlContent)
}

// SendEmail is a no-op on localhost/test environments so local development and
// the test suite never hit a real provider.
func (s *EmailService) SendEmail(email string, subject string, htmlContent string) error {
	if common.EnvironmentIs(common.EnvironmentLocalhost) || common.EnvironmentIs(common.EnvironmentTest) {
		log.Printf("Skipping email sending in test/localhost environment for: %s", email)
		return nil
	}

	apiBaseURL := common.GetEnv("EMAIL_API_BASE_URL")
	apiKey := common.GetEnv("EMAIL_API_KEY")
	senderEmail := common.GetEnv("EMAIL_SENDER_EMAIL")
	appName := common.GetEnv("APP_NAME")

	payload := map[string]interface{}{
		"subject":     subject,
		"htmlContent": htmlContent,
		"to": []map[string]string{
			{"email": email},
		},
		"sender": map[string]string{
			"email": senderEmail,
			"name":  appName,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return errors.NewSimpleError(fmt.Sprintf("Failed to marshal email payload: %v", err))
	}

	req, err := http.NewRequest("POST", apiBaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return errors.NewSimpleError(fmt.Sprintf("Failed to create email request: %v", err))
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.NewSimpleError(fmt.Sprintf("Failed to send email request: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		log.Printf("Email provider returned status %d", resp.StatusCode)
		return errors.NewSimpleError("Failed to send email.")
	}

	return nil
}
