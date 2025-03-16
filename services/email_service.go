package services

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"time"

	"github.com/NomadCrew/nomad-crew-backend/config"
	"github.com/NomadCrew/nomad-crew-backend/logger"
	"github.com/NomadCrew/nomad-crew-backend/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/resend/resend-go/v2"
)

type EmailMetrics struct {
	sendLatency prometheus.Histogram
	errorCount  prometheus.Counter
	sentCount   prometheus.Counter
}

type EmailService struct {
	config  *config.EmailConfig
	client  *resend.Client
	metrics *EmailMetrics
}

func NewEmailService(cfg *config.EmailConfig) *EmailService {
	return NewEmailServiceWithRegistry(cfg, prometheus.DefaultRegisterer)
}

func NewEmailServiceWithRegistry(cfg *config.EmailConfig, reg prometheus.Registerer) *EmailService {
	logger.GetLogger().Infow("Initializing email service",
		"from", cfg.FromAddress, "apikey", cfg.ResendAPIKey)
	client := resend.NewClient(cfg.ResendAPIKey)
	metrics := &EmailMetrics{
		sendLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "nomadcrew_email_send_duration_seconds",
			Help:    "Time taken to send emails",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10},
		}),
		errorCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nomadcrew_email_errors_total",
			Help: "Total number of email sending errors",
		}),
		sentCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "nomadcrew_emails_sent_total",
			Help: "Total number of emails sent",
		}),
	}

	reg.MustRegister(metrics.sendLatency)
	reg.MustRegister(metrics.errorCount)
	reg.MustRegister(metrics.sentCount)

	return &EmailService{
		config:  cfg,
		client:  client,
		metrics: metrics,
	}
}

func (s *EmailService) SendInvitationEmail(ctx context.Context, data types.EmailData) error {
	startTime := time.Now()
	log := logger.GetLogger()
	defer func() {
		s.metrics.sendLatency.Observe(time.Since(startTime).Seconds())
	}()

	// Validate required template data
	requiredFields := []string{"UserEmail", "TripName", "AcceptanceURL"}
	for _, field := range requiredFields {
		if _, ok := data.TemplateData[field]; !ok {
			s.metrics.errorCount.Inc()
			err := fmt.Errorf("missing required template field: %s", field)
			log.Errorw("Invalid template data", "error", err)
			return err
		}
	}

	// Parse and execute template
	tmpl, err := template.New("invitation").Parse(invitationEmailTemplate)
	if err != nil {
		s.metrics.errorCount.Inc()
		log.Errorw("Failed to parse email template", "error", err)
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var htmlContent bytes.Buffer
	if err := tmpl.Execute(&htmlContent, data.TemplateData); err != nil {
		s.metrics.errorCount.Inc()
		log.Errorw("Failed to execute email template", "error", err)
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Build Resend email request
	params := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.config.FromName, s.config.FromAddress),
		To:      []string{data.To},
		Subject: data.Subject,
		Html:    htmlContent.String(),
	}

	// Send email through Resend
	_, err = s.client.Emails.Send(params)
	if err != nil {
		s.metrics.errorCount.Inc()
		log.Errorw("Failed to send email",
			"error", err,
			"to", data.To,
			"subject", data.Subject)
		return fmt.Errorf("email send failed: %w", err)
	}

	s.metrics.sentCount.Inc()
	log.Infow("Email sent successfully",
		"to", data.To,
		"subject", data.Subject)

	return nil
}

// Template constants
const invitationEmailTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Join Your NomadCrew Trip!</title>
    <style>
        body {
            font-family: 'sans-serif';
            background-color: #f7f7f7;
            color: #333333;
            margin: 0;
            padding: 20px;
            text-align: center;
        }
        .container {
            max-width: 600px;
            margin: 20px auto;
            background-color: #ffffff;
            padding: 30px;
            border-radius: 12px;
            box-shadow: 0 4px 8px rgba(0, 0, 0, 0.05);
        }
        h1 {
            color: #F46315;
            font-size: 28px;
            margin-bottom: 20px;
        }
        p {
            font-size: 16px;
            line-height: 1.6;
            margin-bottom: 25px;
        }
        .button {
            display: inline-block;
            padding: 12px 24px;
            font-size: 16px;
            font-weight: bold;
            text-decoration: none;
            background-color: #F46315;
            color: #ffffff;
            border-radius: 8px;
            transition: background-color 0.3s ease;
        }
        .button:hover {
            background-color: #E05A10;
        }
        .link {
            margin-top: 20px;
            font-size: 14px;
            color: #777777;
            word-break: break-all;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>You're Invited to Join a Trip!</h1>
        <p>Hi {{.UserEmail}}!</p>
        <p>You've been invited to join the trip "{{.TripName}}". Click below to accept:</p>
        <p>
            <a href="{{.AcceptanceURL}}" class="button">
                Accept Invitation
            </a>
        </p>
        <p class="link">
            Or copy this link:<br/>
            {{.AcceptanceURL}}
        </p>
    </div>
</body>
</html>`
