package common

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Ensure you have these variables defined appropriately in your package
// var SMTPAccount, SMTPFrom, SMTPServer, SMTPToken, SystemName string
// var SMTPPort int
// var SMTPSSLEnabled bool

func generateMessageID() string {
	domain := strings.Split(SMTPAccount, "@")[1]
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), GetRandomString(12), domain)
}

func SendEmail(subject string, receiver string, content string) error {
	if SMTPFrom == "" { // for compatibility
		SMTPFrom = SMTPAccount
	}
	if SMTPServer == "" || SMTPAccount == "" {
		return fmt.Errorf("SMTP server or account not configured")
	}

	// Encode the subject to handle UTF-8 characters
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))

	// Construct the email headers and body
	mail := []byte(fmt.Sprintf(
		"To: %s\r\n"+
			"From: %s <%s>\r\n"+
			"Subject: %s\r\n"+
			"Date: %s\r\n"+
			"Message-ID: %s\r\n"+
			"Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n",
		receiver,
		SystemName,
		SMTPFrom,
		encodedSubject,
		time.Now().Format(time.RFC1123Z),
		generateMessageID(),
		content,
	))

	addr := fmt.Sprintf("%s:%d", SMTPServer, SMTPPort)
	to := strings.Split(receiver, ";")

	var client *smtp.Client
	var err error

	if SMTPPort == 465 || SMTPSSLEnabled { // Implicit TLS
		// Establish a TLS-encrypted connection
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true, // Consider setting this to false in production
			ServerName:         SMTPServer,
		}

		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to connect via TLS: %w", err)
		}

		client, err = smtp.NewClient(conn, SMTPServer)
		if err != nil {
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
	} else { // STARTTLS or unencrypted
		// Connect to the SMTP server without TLS
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}

		client, err = smtp.NewClient(conn, SMTPServer)
		if err != nil {
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}

		// Check if the server supports STARTTLS
		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsConfig := &tls.Config{
				InsecureSkipVerify: true, // Consider setting this to false in production
				ServerName:         SMTPServer,
			}

			if err = client.StartTLS(tlsConfig); err != nil {
				client.Close()
				return fmt.Errorf("failed to start TLS: %w", err)
			}
		}
	}

	defer client.Close()

	// Choose the appropriate authentication method
	var auth smtp.Auth
	if isOutlookServer(SMTPAccount) {
		auth = LoginAuth(SMTPAccount, SMTPToken)
	} else {
		auth = smtp.PlainAuth("", SMTPAccount, SMTPToken, SMTPServer)
	}

	// Authenticate with the SMTP server
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Set the sender
	if err = client.Mail(SMTPFrom); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set the recipients
	for _, addr := range to {
		addr = strings.TrimSpace(addr)
		if err = client.Rcpt(addr); err != nil {
			return fmt.Errorf("failed to add recipient %s: %w", addr, err)
		}
	}

	// Get the data writer
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	// Write the email content
	_, err = w.Write(mail)
	if err != nil {
		return fmt.Errorf("failed to write email content: %w", err)
	}

	// Close the writer to send the email
	if err = w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	// Quit the SMTP session
	if err = client.Quit(); err != nil {
		return fmt.Errorf("failed to quit SMTP session: %w", err)
	}

	return nil
}
