package webhooks

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	webhookRequestTimeout = 10 * time.Second
	maxRedirects          = 3
)

type SendResult struct {
	StatusCode     int
	DurationMillis int64
}

type DeliveryError struct {
	Category       string
	SafeMessage    string
	StatusCode     int
	DurationMillis int64
	Retryable      bool
	Cause          error
}

func (e *DeliveryError) Error() string {
	if e.Cause != nil {
		return e.SafeMessage + ": " + e.Cause.Error()
	}
	return e.SafeMessage
}

func (e *DeliveryError) Unwrap() error {
	return e.Cause
}

type Sender interface {
	Send(context.Context, string, http.Header, []byte) (SendResult, error)
}

type HTTPSender struct {
	policy DestinationPolicy
	client *http.Client
}

func NewHTTPSender(policy DestinationPolicy) *HTTPSender {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	transport.DialContext = secureDialContext(policy, dialer)
	sender := &HTTPSender{policy: policy}
	sender.client = &http.Client{
		Transport: transport,
		Timeout:   webhookRequestTimeout,
	}
	sender.client.CheckRedirect = sender.checkRedirect
	return sender
}

func (s *HTTPSender) Send(ctx context.Context, destination string, headers http.Header, body []byte) (SendResult, error) {
	requestContext, cancel := context.WithTimeout(ctx, webhookRequestTimeout)
	defer cancel()
	if _, err := s.policy.Validate(requestContext, destination); err != nil {
		return SendResult{}, deliveryErrorForTransport(err, 0)
	}
	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, destination, bytes.NewReader(body))
	if err != nil {
		return SendResult{}, &DeliveryError{
			Category:    "invalid_request",
			SafeMessage: "webhook request could not be created",
			Retryable:   false,
			Cause:       err,
		}
	}
	request.Header = headers.Clone()
	if request.Header == nil {
		request.Header = make(http.Header)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "SEOBlog-Webhook/1.0")

	started := time.Now()
	response, err := s.client.Do(request)
	duration := time.Since(started).Milliseconds()
	if err != nil {
		return SendResult{}, deliveryErrorForTransport(err, duration)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
	result := SendResult{StatusCode: response.StatusCode, DurationMillis: duration}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return result, nil
	}
	retryable := response.StatusCode == http.StatusRequestTimeout ||
		response.StatusCode == http.StatusTooEarly ||
		response.StatusCode == http.StatusTooManyRequests ||
		response.StatusCode >= 500
	category := "http_" + strconv.Itoa(response.StatusCode)
	return SendResult{}, &DeliveryError{
		Category:       category,
		SafeMessage:    "webhook receiver returned HTTP " + strconv.Itoa(response.StatusCode),
		StatusCode:     response.StatusCode,
		DurationMillis: duration,
		Retryable:      retryable,
	}
}

func (s *HTTPSender) checkRedirect(request *http.Request, previous []*http.Request) error {
	if len(previous) >= maxRedirects {
		return &UnsafeDestinationError{Reason: "redirect limit exceeded"}
	}
	if request.Response == nil ||
		(request.Response.StatusCode != http.StatusTemporaryRedirect &&
			request.Response.StatusCode != http.StatusPermanentRedirect) {
		return &UnsafeDestinationError{Reason: "only 307 and 308 redirects are allowed"}
	}
	_, err := s.policy.Validate(request.Context(), request.URL.String())
	return err
}

func secureDialContext(policy DestinationPolicy, dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		validated, err := policy.Validate(ctx, "https://"+net.JoinHostPort(host, port))
		if err != nil {
			return nil, err
		}
		resolver := policy.Resolver
		if resolver == nil {
			resolver = net.DefaultResolver
		}
		if literal := net.ParseIP(validated.Hostname()); literal != nil {
			return dialer.DialContext(ctx, network, net.JoinHostPort(literal.String(), port))
		}
		addresses, err := resolver.LookupIPAddr(ctx, validated.Hostname())
		if err != nil {
			return nil, fmt.Errorf("resolve webhook destination for connection: %w", err)
		}
		var dialErrors []string
		for _, resolved := range addresses {
			if !isPublicIP(resolved.IP) {
				return nil, &UnsafeDestinationError{Reason: "DNS changed to a private or special-purpose address"}
			}
			connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
			if err == nil {
				return connection, nil
			}
			dialErrors = append(dialErrors, err.Error())
		}
		return nil, fmt.Errorf("connect to webhook destination: %s", strings.Join(dialErrors, "; "))
	}
}

func deliveryErrorForTransport(err error, durationMillis int64) *DeliveryError {
	if IsUnsafeDestination(err) {
		return &DeliveryError{
			Category:       "unsafe_destination",
			SafeMessage:    "webhook destination failed the network safety policy",
			DurationMillis: durationMillis,
			Retryable:      false,
			Cause:          err,
		}
	}
	if errors.Is(err, context.Canceled) {
		return &DeliveryError{
			Category:       "cancelled",
			SafeMessage:    "webhook delivery was cancelled",
			DurationMillis: durationMillis,
			Retryable:      true,
			Cause:          err,
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &DeliveryError{
			Category:       "timeout",
			SafeMessage:    "webhook delivery timed out",
			DurationMillis: durationMillis,
			Retryable:      true,
			Cause:          err,
		}
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return &DeliveryError{
			Category:       "timeout",
			SafeMessage:    "webhook delivery timed out",
			DurationMillis: durationMillis,
			Retryable:      true,
			Cause:          err,
		}
	}
	return &DeliveryError{
		Category:       "transport",
		SafeMessage:    "webhook receiver could not be reached",
		DurationMillis: durationMillis,
		Retryable:      true,
		Cause:          err,
	}
}
