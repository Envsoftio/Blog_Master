package webhooks

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestHTTPSenderValidatesAndFollowsOnlySafePreservingRedirects(t *testing.T) {
	resolver := staticResolver{addresses: map[string][]net.IPAddr{
		"first.example.test":  {{IP: net.ParseIP("93.184.216.34")}},
		"second.example.test": {{IP: net.ParseIP("93.184.216.35")}},
	}}
	sender := NewHTTPSender(DestinationPolicy{Resolver: resolver})
	var requests []*http.Request
	sender.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.Clone(request.Context()))
		switch request.URL.Hostname() {
		case "first.example.test":
			return &http.Response{
				StatusCode: http.StatusTemporaryRedirect,
				Header:     http.Header{"Location": []string{"https://second.example.test/final"}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    request,
			}, nil
		case "second.example.test":
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    request,
			}, nil
		default:
			t.Fatalf("unexpected redirect host %q", request.URL.Hostname())
			return nil, nil
		}
	})
	headers := make(http.Header)
	headers.Set("X-SEOBlog-Signature", "v1=signature")
	result, err := sender.Send(context.Background(), "https://first.example.test/start", headers, []byte(`{"ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != http.StatusNoContent || len(requests) != 2 {
		t.Fatalf("unexpected redirect delivery result %#v with %d requests", result, len(requests))
	}
	if requests[1].Method != http.MethodPost ||
		requests[1].Header.Get("X-SEOblog-Signature") != "v1=signature" {
		t.Fatalf("expected 307 redirect to preserve signed POST, got %s %#v", requests[1].Method, requests[1].Header)
	}
}

func TestHTTPSenderBlocksUnsafeRedirects(t *testing.T) {
	resolver := staticResolver{addresses: map[string][]net.IPAddr{
		"first.example.test": {{IP: net.ParseIP("93.184.216.34")}},
	}}
	sender := NewHTTPSender(DestinationPolicy{Resolver: resolver})
	sender.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTemporaryRedirect,
			Header:     http.Header{"Location": []string{"https://127.0.0.1/internal"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})
	_, err := sender.Send(context.Background(), "https://first.example.test/start", nil, []byte(`{}`))
	var deliveryErr *DeliveryError
	if !errors.As(err, &deliveryErr) ||
		deliveryErr.Category != "unsafe_destination" ||
		deliveryErr.Retryable {
		t.Fatalf("expected permanent unsafe redirect failure, got %#v", err)
	}
}

func TestHTTPSenderClassifiesReceiverStatus(t *testing.T) {
	resolver := staticResolver{addresses: map[string][]net.IPAddr{
		"hooks.example.test": {{IP: net.ParseIP("93.184.216.34")}},
	}}
	for _, test := range []struct {
		status    int
		retryable bool
	}{
		{status: http.StatusBadRequest, retryable: false},
		{status: http.StatusTooManyRequests, retryable: true},
		{status: http.StatusServiceUnavailable, retryable: true},
	} {
		sender := NewHTTPSender(DestinationPolicy{Resolver: resolver})
		sender.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: test.status,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("ignored")),
				Request:    request,
			}, nil
		})
		_, err := sender.Send(context.Background(), "https://hooks.example.test/deliver", nil, []byte(`{}`))
		var deliveryErr *DeliveryError
		if !errors.As(err, &deliveryErr) ||
			deliveryErr.StatusCode != test.status ||
			deliveryErr.Retryable != test.retryable {
			t.Fatalf("unexpected classification for %d: %#v", test.status, err)
		}
	}
}
