package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"seoblog/apps/backend/internal/store"
)

type ResponseCache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

const (
	publishedPostCacheTTL     = 24 * time.Hour
	publishedListCacheTTL     = 10 * time.Minute
	publishedLookupMissTTL    = 45 * time.Second
	publishedTaxonomyCacheTTL = 15 * time.Minute
)

type cachedPublishedPost struct {
	Found    bool                          `json:"found"`
	Envelope Envelope[store.PublishedPost] `json:"envelope,omitempty"`
}

type cacheFlightGroup struct {
	mu    sync.Mutex
	calls map[string]*cacheFlightCall
}

type cacheFlightCall struct {
	done  chan struct{}
	value any
	err   error
}

func (g *cacheFlightGroup) do(ctx context.Context, key string, fn func(context.Context) (any, error)) (any, error, bool) {
	if key == "" {
		value, err := fn(ctx)
		return value, err, false
	}

	g.mu.Lock()
	if g.calls == nil {
		g.calls = map[string]*cacheFlightCall{}
	}
	if call := g.calls[key]; call != nil {
		g.mu.Unlock()
		select {
		case <-call.done:
			return call.value, call.err, true
		case <-ctx.Done():
			return nil, ctx.Err(), true
		}
	}
	call := &cacheFlightCall{done: make(chan struct{})}
	g.calls[key] = call
	g.mu.Unlock()

	defer func() {
		if recovered := recover(); recovered != nil {
			call.err = errors.New("content cache fill failed")
			close(call.done)
			g.mu.Lock()
			delete(g.calls, key)
			g.mu.Unlock()
			panic(recovered)
		}
	}()
	call.value, call.err = fn(ctx)
	close(call.done)

	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
	return call.value, call.err, false
}

func contentCacheFill[T any](ctx context.Context, group *cacheFlightGroup, key string, fill func(context.Context) (T, error)) (T, error, bool) {
	var zero T
	value, err, shared := group.do(ctx, key, func(ctx context.Context) (any, error) {
		return fill(ctx)
	})
	if err != nil {
		return zero, err, shared
	}
	typed, ok := value.(T)
	if !ok {
		return zero, fmt.Errorf("content cache singleflight type mismatch for key %q", key), shared
	}
	return typed, nil, shared
}

func cachedContentJSON[T any](ctx context.Context, server *Server, key string, fill func(context.Context) (T, time.Duration, error)) (T, error) {
	var zero T
	if key != "" {
		var cached T
		if server.cacheGetJSON(ctx, key, &cached) {
			return cached, nil
		}
	}

	value, err, _ := contentCacheFill(ctx, &server.cacheFill, key, func(ctx context.Context) (T, error) {
		if key != "" {
			var cached T
			if server.cacheGetJSON(ctx, key, &cached) {
				return cached, nil
			}
		}
		value, ttl, err := fill(ctx)
		if err != nil {
			return zero, err
		}
		if ttl > 0 {
			server.cacheSetJSON(ctx, key, value, ttl)
		}
		return value, nil
	})
	return value, err
}

func (s *Server) projectGeneration(ctx context.Context, projectID string) int64 {
	generation, err := s.store.ContentGeneration(ctx, projectID)
	if err != nil {
		s.logCache("content generation lookup failed", slog.LevelWarn, "project_id", projectID, "error", err)
		return 0
	}
	return generation
}

func (s *Server) contentCacheKey(projectID string, generation int64, namespace string, vary ...string) (string, bool) {
	if s.cache == nil || generation <= 0 {
		return "", false
	}
	hash := sha256.Sum256([]byte(strings.Join(vary, "\x00")))
	return fmt.Sprintf("blog:v1:%s:%s:%d:%s", namespace, projectID, generation, hex.EncodeToString(hash[:12])), true
}

func (s *Server) cacheGetJSON(ctx context.Context, key string, destination any) bool {
	if s.cache == nil || key == "" {
		if s.metrics != nil {
			s.metrics.RecordCache("disabled")
		}
		return false
	}
	raw, ok, err := s.cache.Get(ctx, key)
	if err != nil {
		if s.metrics != nil {
			s.metrics.RecordCache("fallback")
		}
		s.logCache("content cache get failed", slog.LevelWarn, "key", key, "error", err)
		return false
	}
	if !ok {
		if s.metrics != nil {
			s.metrics.RecordCache("miss")
		}
		return false
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		if s.metrics != nil {
			s.metrics.RecordCache("fallback")
		}
		s.logCache("content cache decode failed", slog.LevelWarn, "key", key, "error", err)
		return false
	}
	if s.metrics != nil {
		s.metrics.RecordCache("hit")
	}
	return true
}

func (s *Server) cacheSetJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	if s.cache == nil || key == "" {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		s.logCache("content cache encode failed", slog.LevelWarn, "key", key, "error", err)
		return
	}
	if err := s.cache.Set(ctx, key, raw, jitterTTL(key, ttl)); err != nil {
		if s.metrics != nil {
			s.metrics.RecordCache("fallback")
		}
		s.logCache("content cache set failed", slog.LevelWarn, "key", key, "error", err)
	}
}

func (s *Server) logCache(message string, level slog.Level, args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Log(context.Background(), level, message, args...)
}

func jitterTTL(key string, ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return ttl
	}
	hash := sha256.Sum256([]byte(key))
	offset := int(hash[0]) - 128
	jitter := time.Duration(int64(ttl) * int64(offset) / 1280)
	return ttl + jitter
}

func normalizedContentQuery(c fiber.Ctx) string {
	values, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil {
		return string(c.Request().URI().QueryString())
	}
	for key := range values {
		sort.Strings(values[key])
	}
	return values.Encode()
}

func cacheLimit(limit int) string {
	return strconv.Itoa(limit)
}
