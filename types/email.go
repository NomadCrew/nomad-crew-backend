package types

import "context"

type EmailService interface {
    SendInvitationEmail(ctx context.Context, data EmailData) error
}

type EmailData struct {
    To           string
    Subject      string
    TemplateData map[string]interface{}
}