package webhooks

import (
	"context"
	"errors"
	"net"
	"testing"
)

type staticResolver struct {
	addresses map[string][]net.IPAddr
	errors    map[string]error
}

func (r staticResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if err := r.errors[host]; err != nil {
		return nil, err
	}
	return r.addresses[host], nil
}

func TestDestinationPolicyRequiresPublicHTTPSAddresses(t *testing.T) {
	resolver := staticResolver{addresses: map[string][]net.IPAddr{
		"hooks.example.test": {{IP: net.ParseIP("93.184.216.34")}},
		"mixed.example.test": {
			{IP: net.ParseIP("93.184.216.34")},
			{IP: net.ParseIP("10.0.0.4")},
		},
	}}
	policy := DestinationPolicy{Environment: "production", Resolver: resolver}
	if _, err := policy.Validate(context.Background(), "https://hooks.example.test/revalidate"); err != nil {
		t.Fatalf("expected public HTTPS destination to pass: %v", err)
	}
	for _, destination := range []string{
		"http://hooks.example.test/revalidate",
		"https://user:password@hooks.example.test/revalidate",
		"https://127.0.0.1/revalidate",
		"https://169.254.169.254/latest/meta-data",
		"https://metadata.google.internal/computeMetadata/v1",
		"https://mixed.example.test/revalidate",
	} {
		if _, err := policy.Validate(context.Background(), destination); !IsUnsafeDestination(err) {
			t.Fatalf("expected %q to fail safety validation, got %v", destination, err)
		}
	}
}

func TestDestinationPolicyEnforcesStagingAllowlist(t *testing.T) {
	resolver := staticResolver{addresses: map[string][]net.IPAddr{
		"hooks.staging.example":    {{IP: net.ParseIP("93.184.216.34")}},
		"deploy.preview.example":   {{IP: net.ParseIP("93.184.216.35")}},
		"hooks.production.example": {{IP: net.ParseIP("93.184.216.36")}},
	}}
	emptyPolicy := DestinationPolicy{Environment: "staging", Resolver: resolver}
	if _, err := emptyPolicy.Validate(context.Background(), "https://hooks.staging.example"); !IsUnsafeDestination(err) {
		t.Fatalf("expected empty staging allowlist to block delivery, got %v", err)
	}
	policy := DestinationPolicy{
		Environment:  "staging",
		AllowedHosts: []string{"hooks.staging.example", "*.preview.example"},
		Resolver:     resolver,
	}
	for _, destination := range []string{
		"https://hooks.staging.example/revalidate",
		"https://deploy.preview.example/revalidate",
	} {
		if _, err := policy.Validate(context.Background(), destination); err != nil {
			t.Fatalf("expected allowlisted destination %q: %v", destination, err)
		}
	}
	if _, err := policy.Validate(context.Background(), "https://hooks.production.example/revalidate"); !IsUnsafeDestination(err) {
		t.Fatalf("expected production host to be blocked in staging, got %v", err)
	}
}

func TestDestinationPolicyReportsResolutionFailuresAsTransientErrors(t *testing.T) {
	resolver := staticResolver{errors: map[string]error{
		"missing.example.test": errors.New("temporary DNS failure"),
	}}
	_, err := (DestinationPolicy{Resolver: resolver}).Validate(context.Background(), "https://missing.example.test/hook")
	if err == nil || IsUnsafeDestination(err) {
		t.Fatalf("expected non-safety DNS failure, got %v", err)
	}
}
