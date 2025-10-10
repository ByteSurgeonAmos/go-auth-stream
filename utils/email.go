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

func emailTemplate(title, username, bodyContent, buttonText, buttonLink string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>%s</title>
</head>
<body style="margin:0; padding:0; background-color:#0b0b0b; font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif;">
  <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%" style="background-color:#0b0b0b; min-height:100vh;">
    <tr>
      <td align="center" style="padding:40px 20px;">
        <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%" style="max-width:500px; background-color:#0b0b0b; color:#ffffff; text-align:center;">
          <!-- Logo (Orb) -->
          <tr>
            <td style="padding-bottom:30px;">
              <div style="width:60px; height:60px; margin:0 auto; background:linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); border-radius:50%%; box-shadow: 0 8px 16px rgba(102, 126, 234, 0.4);"></div>
            </td>
          </tr>
          <!-- Title -->
          <tr>
            <td style="font-size:24px; font-weight:bold; padding-bottom:10px; color:#ffffff;">
              %s
            </td>
          </tr>
          <!-- Body -->
          <tr>
            <td style="font-size:15px; line-height:22px; color:#cfcfcf; padding:20px 0;">
              %s
            </td>
          </tr>
          <!-- Button -->
          %s
          <!-- Footer -->
          <tr>
            <td style="padding-top:20px; font-size:14px; color:#a0a0a0;">
              Cheers,<br>
              The Agent4 Team
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>
	`, title, title, bodyContent, buttonHTML(buttonText, buttonLink))
}

func buttonHTML(buttonText, buttonLink string) string {
	if buttonText == "" {
		return ""
	}
	return fmt.Sprintf(`
          <tr>
            <td align="center" style="padding:20px 0;">
              <a href="%s" 
                 style="background-color:#ffffff; color:#000000; text-decoration:none; padding:12px 30px; border-radius:6px; font-weight:bold; display:inline-block;">
                 %s
              </a>
            </td>
          </tr>`, buttonLink, buttonText)
}

func Send2FACode(to, code string) error {
	title := "Your Two-Factor Authentication Code"
	bodyContent := fmt.Sprintf(`
		Hello,<br><br>
		Your verification code is:<br>
		<div style="background-color:#1a1a1a; border-radius:8px; padding:20px; margin:20px 0; font-size:32px; font-weight:bold; letter-spacing:8px; color:#ffffff;">
			%s
		</div>
		This code will expire in <b>10 minutes</b>.<br><br>
		If you didn't request this code, please ignore this email.
	`, code)

	body := emailTemplate(title, "", bodyContent, "", "")
	return SendEmail(to, title, body)
}

func SendVerificationCode(to, code string) error {
	title := "Verify Your Account"
	bodyContent := fmt.Sprintf(`
		Hello,<br><br>
		Thank you for signing up! Please verify your account using the code below:<br>
		<div style="background-color:#1a1a1a; border-radius:8px; padding:20px; margin:20px 0; font-size:32px; font-weight:bold; letter-spacing:8px; color:#ffffff;">
			%s
		</div>
		This code will expire in <b>10 minutes</b>.<br><br>
		If you didn't create an account, please ignore this email.
	`, code)

	body := emailTemplate(title, "", bodyContent, "", "")
	return SendEmail(to, title, body)
}

func SendWelcomeEmail(to, username string) error {
	title := fmt.Sprintf("Welcome to <span style='color:#ffffff;'>Agent4</span>, %s!", username)
	bodyContent := fmt.Sprintf(`
		Hello %s,<br><br>
		We're excited to have you onboard at <b>Agent4</b>. We hope you enjoy your journey with us.<br><br>
		Start by setting up your company profile and connecting your social media accounts.<br><br>
		If you have any questions or need assistance, feel free to reach out.
	`, username)

	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "https://agent4.com"
	}

	body := emailTemplate("Welcome to Agent4", username, bodyContent, "Get Started", appURL+"/dashboard")
	return SendEmail(to, title, body)
}

func SendSubscriptionConfirmation(to, planName, amount string) error {
	title := "Subscription Confirmed!"
	bodyContent := fmt.Sprintf(`
		Hello,<br><br>
		Your subscription to the <b>%s</b> plan has been confirmed.<br><br>
		<div style="background-color:#1a1a1a; border-radius:8px; padding:15px; margin:15px 0; text-align:left;">
			<div style="margin-bottom:8px;"><span style="color:#a0a0a0;">Plan:</span> <b>%s</b></div>
			<div><span style="color:#a0a0a0;">Amount:</span> <b>%s</b></div>
		</div>
		Thank you for your payment! You now have full access to all features.
	`, planName, planName, amount)

	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "https://agent4.com"
	}

	body := emailTemplate(title, "", bodyContent, "View Dashboard", appURL+"/dashboard")
	return SendEmail(to, title, body)
}

func SendPaymentReceipt(to, transactionRef, amount, planName string) error {
	title := "Payment Receipt"
	bodyContent := fmt.Sprintf(`
		Hello,<br><br>
		Thank you for your payment! Here are your transaction details:<br><br>
		<div style="background-color:#1a1a1a; border-radius:8px; padding:15px; margin:15px 0; text-align:left;">
			<div style="margin-bottom:8px;"><span style="color:#a0a0a0;">Transaction Reference:</span> <b>%s</b></div>
			<div style="margin-bottom:8px;"><span style="color:#a0a0a0;">Plan:</span> <b>%s</b></div>
			<div><span style="color:#a0a0a0;">Amount:</span> <b>%s</b></div>
		</div>
		Keep this email for your records.
	`, transactionRef, planName, amount)

	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "https://agent4.com"
	}

	body := emailTemplate(title, "", bodyContent, "View Receipt", appURL+"/billing")
	return SendEmail(to, title, body)
}
