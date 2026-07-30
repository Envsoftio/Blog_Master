package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

const defaultTimeout = 10 * time.Second

var ErrNotConfigured = errors.New("SMTP is not configured")

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Sender interface {
	Send(context.Context, Message) error
}

type SMTPConfig struct {
	Address         string
	Username        string
	Password        string
	From            string
	FromName        string
	RequireStartTLS bool
}

type SMTPSender struct {
	config SMTPConfig
}

func NewSMTP(config SMTPConfig) *SMTPSender {
	return &SMTPSender{config: config}
}

func (s *SMTPSender) Send(ctx context.Context, message Message) error {
	address := strings.TrimSpace(s.config.Address)
	if address == "" {
		return ErrNotConfigured
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse SMTP address: %w", err)
	}
	from, err := mail.ParseAddress(strings.TrimSpace(s.config.From))
	if err != nil {
		return fmt.Errorf("parse SMTP sender: %w", err)
	}
	to, err := mail.ParseAddress(strings.TrimSpace(message.To))
	if err != nil {
		return fmt.Errorf("parse SMTP recipient: %w", err)
	}
	formattedMessage, err := formatMessage(s.config, from, to, message)
	if err != nil {
		return fmt.Errorf("format SMTP message: %w", err)
	}

	dialer := net.Dialer{Timeout: defaultTimeout}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer connection.Close()

	deadline := time.Now().Add(defaultTimeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set SMTP deadline: %w", err)
	}

	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return fmt.Errorf("start SMTP client: %w", err)
	}
	defer client.Close()

	username := strings.TrimSpace(s.config.Username)
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	} else if s.config.RequireStartTLS || username != "" {
		return errors.New("SMTP server does not support required STARTTLS")
	}
	if username != "" {
		if err := client.Auth(smtp.PlainAuth("", username, s.config.Password, host)); err != nil {
			return fmt.Errorf("authenticate to SMTP server: %w", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP message: %w", err)
	}
	if _, err := writer.Write(formattedMessage); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("finish SMTP delivery: %w", err)
	}
	return nil
}

func formatMessage(config SMTPConfig, from, to *mail.Address, message Message) ([]byte, error) {
	fromHeader := from.String()
	if name := strings.TrimSpace(config.FromName); name != "" {
		fromHeader = (&mail.Address{Name: name, Address: from.Address}).String()
	}
	subject := sanitizeHeader(message.Subject)
	headers := "From: " + fromHeader + "\r\n" +
		"To: " + to.String() + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n"

	if strings.TrimSpace(message.HTML) == "" {
		return []byte(
			headers +
				"Content-Type: text/plain; charset=UTF-8\r\n" +
				"Content-Transfer-Encoding: 8bit\r\n" +
				"\r\n" +
				normalizeBody(message.Text) + "\r\n",
		), nil
	}

	var body bytes.Buffer
	multipartWriter := multipart.NewWriter(&body)
	textPart, err := multipartWriter.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {"text/plain; charset=UTF-8"},
		"Content-Transfer-Encoding": {"8bit"},
	})
	if err != nil {
		return nil, err
	}
	if _, err := textPart.Write([]byte(normalizeBody(message.Text))); err != nil {
		return nil, err
	}
	htmlPart, err := multipartWriter.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {"text/html; charset=UTF-8"},
		"Content-Transfer-Encoding": {"8bit"},
	})
	if err != nil {
		return nil, err
	}
	if _, err := htmlPart.Write([]byte(normalizeBody(message.HTML))); err != nil {
		return nil, err
	}
	if err := multipartWriter.Close(); err != nil {
		return nil, err
	}

	return []byte(
		headers +
			"Content-Type: multipart/alternative; boundary=\"" + multipartWriter.Boundary() + "\"\r\n" +
			"\r\n" +
			body.String(),
	), nil
}

func sanitizeHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.ReplaceAll(value, "\n", " ")
}

func normalizeBody(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.ReplaceAll(value, "\n", "\r\n")
}
