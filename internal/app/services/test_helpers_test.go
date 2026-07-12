package services

import (
	"github.com/scienceandcode/nucleus-api/internal/app/interfaces"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/email"
)

// newTestEmailService returns the real EmailService. SendEmail is a no-op in
// the test environment (see EmailService.SendEmail), so it is safe to use here.
func newTestEmailService() interfaces.EmailServiceInterface {
	return email.NewEmailService()
}
