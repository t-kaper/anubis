package providers

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

const (
	smtpServer = "smtp.mail.ru"
	smtpPort   = 465
	fromEmail  = "support@yocha.fun"
	password   = "wchgxKCNGuEbhwtb93QT"
)

func SenderEmail(body string, email string) (string, string, error) {
	// Prepare email content
	to := []string{email}
	subject := "Verification Code"

	// Create email message
	header := make(map[string]string)
	header["From"] = fromEmail
	header["To"] = strings.Join(to, ",")
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=utf-8"

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// Configure TLS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         smtpServer,
	}

	// Connect to SMTP server
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", smtpServer, smtpPort), tlsConfig)
	if err != nil {
		return "", "", fmt.Errorf("failed to connect to SMTP server: %v", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, smtpServer)
	if err != nil {
		return "", "", fmt.Errorf("failed to create SMTP client: %v", err)
	}
	defer client.Close()

	// Authenticate
	auth := smtp.PlainAuth("", fromEmail, password, smtpServer)
	if err = client.Auth(auth); err != nil {
		return "", "", fmt.Errorf("SMTP authentication failed: %v", err)
	}

	// Set sender and recipient
	if err = client.Mail(fromEmail); err != nil {
		return "", "", fmt.Errorf("failed to set sender: %v", err)
	}

	for _, addr := range to {
		if err = client.Rcpt(addr); err != nil {
			return "", "", fmt.Errorf("failed to set recipient: %v", err)
		}
	}

	// Send message body
	w, err := client.Data()
	if err != nil {
		return "", "", fmt.Errorf("failed to create message writer: %v", err)
	}

	_, err = w.Write([]byte(message))
	if err != nil {
		return "", "", fmt.Errorf("failed to write message: %v", err)
	}

	err = w.Close()
	if err != nil {
		return "", "", fmt.Errorf("failed to close message writer: %v", err)
	}

	// Quit SMTP session
	client.Quit()
	return "IdSend", "yocha.fun", nil
}
