package aijobs

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"seoblog/apps/backend/internal/store"
)

func TestOpenAICompatibleClientGeneratesBoundedJSONProposal(t *testing.T) {
	client, err := NewOpenAICompatibleClient(ClientConfig{
		BaseURL: "https://provider.example.test/v1", APIKey: "secret", Model: "test-model",
		MaxOutputTokens: 800, Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatal("missing provider authorization")
		}
		var input chatRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input.Model != "test-model" || input.MaxTokens != 800 {
			t.Fatalf("unexpected provider request: %#v", input)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
			"choices":[{"message":{"role":"assistant","content":"{\"title\":\"Evidence-bound outline\"}"}}],
			"usage":{"prompt_tokens":123,"completion_tokens":45}
		}`)),
		}, nil
	})}
	result, err := client.Generate(context.Background(), store.AIJobInputSnapshot{TaskType: "outline"})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Output) != `{"title":"Evidence-bound outline"}` {
		t.Fatalf("unexpected output %s", result.Output)
	}
	if result.InputTokens != 123 || result.OutputTokens != 45 {
		t.Fatalf("unexpected usage %#v", result)
	}
}

func TestOpenAICompatibleClientClassifiesRateLimitAsRetryable(t *testing.T) {
	client, err := NewOpenAICompatibleClient(ClientConfig{
		BaseURL: "https://provider.example.test", APIKey: "secret", Model: "test-model", Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("rate limited"))}, nil
	})}
	_, err = client.Generate(context.Background(), store.AIJobInputSnapshot{TaskType: "outline"})
	category, _, retryable := classifyGenerationError(err)
	if category != "provider_rate_limited" || !retryable {
		t.Fatalf("unexpected classification %q retryable=%v: %v", category, retryable, err)
	}
}

func TestOpenAICompatibleClientRejectsCredentialedOrAmbiguousBaseURL(t *testing.T) {
	for _, baseURL := range []string{
		"https://user:password@provider.example.test/v1",
		"https://provider.example.test/v1?tenant=other",
		"http://provider.example.test/v1",
	} {
		if _, err := NewOpenAICompatibleClient(ClientConfig{BaseURL: baseURL, APIKey: "secret", Model: "model"}); err == nil {
			t.Fatalf("expected base URL %q to be rejected", baseURL)
		}
	}
}

func TestProcessorBlocksOversizedImmutableInputBeforeProviderCall(t *testing.T) {
	jobStore := &fakeJobStore{jobs: []store.AIExecutionJob{{
		Job: store.AdminAIJob{ID: "job", ProjectID: "project"},
		Snapshot: store.AIJobInputSnapshot{
			Brief: store.AIJobBrief{Evidence: strings.Repeat("evidence", 50)},
		},
		Attempts: 1,
	}}}
	generator := &fakeGenerator{}
	result, err := (Processor{
		Store: jobStore, Generator: generator, WorkerID: "worker", Provider: "provider",
		Model: "model", MaxInputBytes: 32,
	}).Process(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if generator.calls != 0 {
		t.Fatal("provider must not be called for an over-budget input")
	}
	if result.Failed != 1 || jobStore.failedCategory != "budget_blocked" || jobStore.failedRetryable {
		t.Fatalf("unexpected result %#v / store %#v", result, jobStore)
	}
}

type fakeJobStore struct {
	jobs            []store.AIExecutionJob
	failedCategory  string
	failedRetryable bool
}

func (fake *fakeJobStore) ClaimAIJobs(context.Context, string, string, string, time.Duration, int) ([]store.AIExecutionJob, error) {
	if len(fake.jobs) == 0 {
		return nil, nil
	}
	job := fake.jobs[0]
	fake.jobs = fake.jobs[1:]
	return []store.AIExecutionJob{job}, nil
}

func (*fakeJobStore) CompleteAIJob(context.Context, string, store.AIExecutionJob, store.AIExecutionResult) error {
	return nil
}

func (fake *fakeJobStore) FailAIJob(_ context.Context, _ string, _ store.AIExecutionJob, category, _ string, retryable bool) error {
	fake.failedCategory = category
	fake.failedRetryable = retryable
	return nil
}

type fakeGenerator struct{ calls int }

func (fake *fakeGenerator) Generate(context.Context, store.AIJobInputSnapshot) (store.AIExecutionResult, error) {
	fake.calls++
	return store.AIExecutionResult{Output: json.RawMessage(`{}`)}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}
