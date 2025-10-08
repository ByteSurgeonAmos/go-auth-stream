package utils

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

type EmailService struct {
	dialer *gomail.Dialer
	from   string
}

var emailService *EmailService

func InitEmailService() error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPortStr := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	fromEmail := os.Getenv("FROM_EMAIL")

	if smtpHost == "" || smtpPortStr == "" || smtpUser == "" || smtpPass == "" {
		return fmt.Errorf("SMTP configuration not found in environment variables")
	}

	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return fmt.Errorf("invalid SMTP_PORT: %v", err)
	}

	if fromEmail == "" {
		fromEmail = smtpUser
	}

	emailService = &EmailService{
		dialer: gomail.NewDialer(smtpHost, smtpPort, smtpUser, smtpPass),
		from:   fromEmail,
	}

	return nil
}

func SendEmail(to, subject, body string) error {
	if emailService == nil {
		return fmt.Errorf("email service not initialized")
	}

	m := gomail.NewMessage()
	m.SetHeader("From", emailService.from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	if err := emailService.dialer.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	return nil
}

func Send2FACode(to, code string) error {
	subject := "Your Two-Factor Authentication Code"
	body := fmt.Sprintf(`
		<html>
			<body>
				<h2>Two-Factor Authentication</h2>
				<p>Your verification code is: <strong style="font-size: 24px; color: #007bff;">%s</strong></p>
				<p>This code will expire in 10 minutes.</p>
				<p>If you didn't request this code, please ignore this email.</p>
			</body>
		</html>
	`, code)

	return SendEmail(to, subject, body)
}

func SendWelcomeEmail(to, username string) error {
	subject := "Welcome to Agent4!"
	body := fmt.Sprintf(`
		<html>
			<body>
				<h2>Welcome to Agent4, %s!</h2>
				<p>Thank you for signing up. We're excited to have you on board.</p>
				<p>Start by setting up your company profile and connecting your social media accounts.</p>
				<br>
				<p>Best regards,<br>The Agent4 Team</p>
			</body>
		</html>
	`, username)

	return SendEmail(to, subject, body)
}

func SendSubscriptionConfirmation(to, planName, amount string) error {
	subject := "Subscription Confirmation"
	body := fmt.Sprintf(`
		<html>
			<body>
				<h2>Subscription Confirmed!</h2>
				<p>Your subscription to the <strong>%s</strong> plan has been confirmed.</p>
				<p>Amount: <strong>%s</strong></p>
				<p>Thank you for your payment!</p>
				<br>
				<p>Best regards,<br>The Agent4 Team</p>
			</body>
		</html>
	`, planName, amount)

	return SendEmail(to, subject, body)
}

func SendPaymentReceipt(to, transactionRef, amount, planName string) error {
	subject := "Payment Receipt"
	body := fmt.Sprintf(`
		<html>
			<body>
				<h2>Payment Receipt</h2>
				<p>Thank you for your payment!</p>
				<p><strong>Transaction Reference:</strong> %s</p>
				<p><strong>Plan:</strong> %s</p>
				<p><strong>Amount:</strong> %s</p>
				<br>
				<p>Best regards,<br>The Agent4 Team</p>
			</body>
		</html>
	`, transactionRef, planName, amount)

	return SendEmail(to, subject, body)
}
