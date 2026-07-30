package mailer

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestSMTPSenderRequiresConfiguration(t *testing.T) {
	err := NewSMTP(SMTPConfig{}).Send(context.Background(), Message{
		To:      "owner@example.test",
		Subject: "Reset",
		Text:    "Body",
	})
	if err != ErrNotConfigured {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestFormatMessageUsesMultipartAlternativeAndSanitizesSubject(t *testing.T) {
	config := SMTPConfig{From: "no-reply@example.test", FromName: "SEO Blog"}
	from := mustAddress(t, config.From)
	to := mustAddress(t, "owner@example.test")
	formatted, err := formatMessage(config, from, to, Message{
		Subject: "Reset\r\nBcc: attacker@example.test",
		Text:    "First line\nSecond line",
		HTML:    "<p>First line<br>Second line</p>",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := string(formatted)

	if strings.Contains(raw, "\r\nBcc:") {
		t.Fatalf("subject injected an email header:\n%s", raw)
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(formatted))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Header.Get("Bcc") != "" {
		t.Fatalf("subject injected a Bcc header: %#v", parsed.Header)
	}
	mediaType, parameters, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	if mediaType != "multipart/alternative" {
		t.Fatalf("expected multipart/alternative, got %q", mediaType)
	}
	parts := multipart.NewReader(parsed.Body, parameters["boundary"])
	textPart, err := parts.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	textBody, err := io.ReadAll(textPart)
	if err != nil {
		t.Fatal(err)
	}
	if textPart.Header.Get("Content-Type") != "text/plain; charset=UTF-8" ||
		string(textBody) != "First line\r\nSecond line" {
		t.Fatalf("unexpected plain-text part: %q, %q", textPart.Header.Get("Content-Type"), textBody)
	}
	htmlPart, err := parts.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	htmlBody, err := io.ReadAll(htmlPart)
	if err != nil {
		t.Fatal(err)
	}
	if htmlPart.Header.Get("Content-Type") != "text/html; charset=UTF-8" ||
		string(htmlBody) != "<p>First line<br>Second line</p>" {
		t.Fatalf("unexpected HTML part: %q, %q", htmlPart.Header.Get("Content-Type"), htmlBody)
	}
	if _, err := parts.NextPart(); err != io.EOF {
		t.Fatalf("expected exactly two MIME parts, got %v", err)
	}

	for _, expected := range []string{
		"From: \"SEO Blog\" <no-reply@example.test>\r\n",
		"To: <owner@example.test>\r\n",
	} {
		if !strings.Contains(raw, expected) {
			t.Fatalf("expected message to contain %q:\n%s", expected, raw)
		}
	}
}

func TestSMTPSenderDeliversMessage(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	type serverResult struct {
		message string
		err     error
	}
	result := make(chan serverResult, 1)
	go func() {
		message, serverErr := acceptSMTPMessage(listener)
		result <- serverResult{message: message, err: serverErr}
	}()

	sender := NewSMTP(SMTPConfig{
		Address:  listener.Addr().String(),
		From:     "no-reply@example.test",
		FromName: "SEO Blog",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sender.Send(ctx, Message{
		To:      "owner@example.test",
		Subject: "Reset your password",
		Text:    "Use https://admin.example.test/reset-password?token=secret",
		HTML:    `<p>Use <a href="https://admin.example.test/reset-password?token=secret">this link</a>.</p>`,
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case delivered := <-result:
		if delivered.err != nil {
			t.Fatal(delivered.err)
		}
		for _, expected := range []string{
			"Subject: Reset your password",
			"To: <owner@example.test>",
			"https://admin.example.test/reset-password?token=secret",
			"Content-Type: text/html; charset=UTF-8",
		} {
			if !strings.Contains(delivered.message, expected) {
				t.Fatalf("expected delivered message to contain %q:\n%s", expected, delivered.message)
			}
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for SMTP delivery")
	}
}

func TestSMTPSenderRejectsServerWithoutRequiredSTARTTLS(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	result := make(chan error, 1)
	go func() {
		_, serverErr := acceptSMTPMessage(listener)
		result <- serverErr
	}()

	sender := NewSMTP(SMTPConfig{
		Address:         listener.Addr().String(),
		From:            "no-reply@example.test",
		RequireStartTLS: true,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = sender.Send(ctx, Message{
		To:      "owner@example.test",
		Subject: "Reset your password",
		Text:    "Body",
	})
	if err == nil || !strings.Contains(err.Error(), "required STARTTLS") {
		t.Fatalf("expected required STARTTLS error, got %v", err)
	}

	select {
	case <-result:
	case <-ctx.Done():
		t.Fatal("timed out waiting for SMTP server to stop")
	}
}

func mustAddress(t *testing.T, value string) *mail.Address {
	t.Helper()
	address, err := mail.ParseAddress(value)
	if err != nil {
		t.Fatal(err)
	}
	return address
}

func acceptSMTPMessage(listener net.Listener) (string, error) {
	connection, err := listener.Accept()
	if err != nil {
		return "", err
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return "", err
	}
	reader := bufio.NewReader(connection)
	writer := bufio.NewWriter(connection)
	writeResponse := func(response string) error {
		if _, err := writer.WriteString(response + "\r\n"); err != nil {
			return err
		}
		return writer.Flush()
	}
	if err := writeResponse("220 localhost ESMTP"); err != nil {
		return "", err
	}

	var message strings.Builder
	inData := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if inData {
			if line == "." {
				inData = false
				if err := writeResponse("250 queued"); err != nil {
					return "", err
				}
				continue
			}
			message.WriteString(line)
			message.WriteByte('\n')
			continue
		}

		command := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(command, "EHLO "), strings.HasPrefix(command, "HELO "):
			if err := writeResponse("250 localhost"); err != nil {
				return "", err
			}
		case strings.HasPrefix(command, "MAIL FROM:"), strings.HasPrefix(command, "RCPT TO:"):
			if err := writeResponse("250 ok"); err != nil {
				return "", err
			}
		case command == "DATA":
			inData = true
			if err := writeResponse("354 end with <CRLF>.<CRLF>"); err != nil {
				return "", err
			}
		case command == "QUIT":
			if err := writeResponse("221 bye"); err != nil {
				return "", err
			}
			return message.String(), nil
		default:
			return "", fmt.Errorf("unexpected SMTP command %q", line)
		}
	}
}
