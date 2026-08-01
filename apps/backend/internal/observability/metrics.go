package observability

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var durationBuckets = [...]float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type requestKey struct {
	method string
	route  string
	status int
}

type durationSeries struct {
	count   uint64
	sum     float64
	buckets [len(durationBuckets)]uint64
}

// Registry keeps process-local counters and renders them together with durable
// delivery state from SQLite. Labels intentionally exclude project, actor,
// content and job IDs so the scrape surface remains bounded.
type Registry struct {
	mu             sync.Mutex
	db             *sql.DB
	dbPath         string
	service        string
	environment    string
	startedAt      time.Time
	requests       map[requestKey]*durationSeries
	cacheEvents    map[string]uint64
	workerCycles   map[string]map[string]uint64
	workerItems    map[string]map[string]uint64
	workerDuration map[string]*durationSeries
}

func NewRegistry(db *sql.DB, dbPath, service, environment string) *Registry {
	return &Registry{
		db:             db,
		dbPath:         strings.TrimSpace(dbPath),
		service:        strings.TrimSpace(service),
		environment:    strings.TrimSpace(environment),
		startedAt:      time.Now(),
		requests:       make(map[requestKey]*durationSeries),
		cacheEvents:    make(map[string]uint64),
		workerCycles:   make(map[string]map[string]uint64),
		workerItems:    make(map[string]map[string]uint64),
		workerDuration: make(map[string]*durationSeries),
	}
}

func (registry *Registry) RecordHTTPRequest(method, route string, status int, duration time.Duration) {
	if registry == nil {
		return
	}
	key := requestKey{method: strings.ToUpper(strings.TrimSpace(method)), route: boundedRoute(route), status: status}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	recordDuration(registry.requests, key, duration)
}

func (registry *Registry) RecordCache(outcome string) {
	if registry == nil {
		return
	}
	outcome = boundedOutcome(outcome)
	registry.mu.Lock()
	registry.cacheEvents[outcome]++
	registry.mu.Unlock()
}

func (registry *Registry) RecordWorkerCycle(component, outcome string, duration time.Duration, items int) {
	if registry == nil {
		return
	}
	component = boundedComponent(component)
	outcome = boundedOutcome(outcome)
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.workerCycles[component] == nil {
		registry.workerCycles[component] = make(map[string]uint64)
	}
	registry.workerCycles[component][outcome]++
	if registry.workerItems[component] == nil {
		registry.workerItems[component] = make(map[string]uint64)
	}
	if items > 0 {
		registry.workerItems[component][outcome] += uint64(items)
	}
	series := registry.workerDuration[component]
	if series == nil {
		series = &durationSeries{}
		registry.workerDuration[component] = series
	}
	recordDurationSeries(series, duration)
}

func (registry *Registry) Render(ctx context.Context, writer io.Writer) error {
	if registry == nil {
		return nil
	}
	registry.renderProcess(writer)
	registry.renderApplication(writer)
	return registry.renderDurable(ctx, writer)
}

func (registry *Registry) renderProcess(writer io.Writer) {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	writeMetricHeader(writer, "seoblog_process_uptime_seconds", "Seconds since this service process started.", "gauge")
	writeSample(writer, "seoblog_process_uptime_seconds", registry.baseLabels(), time.Since(registry.startedAt).Seconds())
	writeMetricHeader(writer, "seoblog_process_resident_memory_bytes", "Go heap bytes currently allocated by the service.", "gauge")
	writeSample(writer, "seoblog_process_resident_memory_bytes", registry.baseLabels(), float64(memory.Alloc))
	writeMetricHeader(writer, "seoblog_process_goroutines", "Current goroutine count.", "gauge")
	writeSample(writer, "seoblog_process_goroutines", registry.baseLabels(), float64(runtime.NumGoroutine()))
}

func (registry *Registry) renderApplication(writer io.Writer) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	writeMetricHeader(writer, "seoblog_http_requests_total", "Completed HTTP requests by normalized route and status.", "counter")
	writeMetricHeader(writer, "seoblog_http_request_duration_seconds", "HTTP request latency by normalized route and status.", "histogram")
	requestKeys := make([]requestKey, 0, len(registry.requests))
	for key := range registry.requests {
		requestKeys = append(requestKeys, key)
	}
	sort.Slice(requestKeys, func(i, j int) bool {
		if requestKeys[i].route != requestKeys[j].route {
			return requestKeys[i].route < requestKeys[j].route
		}
		if requestKeys[i].method != requestKeys[j].method {
			return requestKeys[i].method < requestKeys[j].method
		}
		return requestKeys[i].status < requestKeys[j].status
	})
	for _, key := range requestKeys {
		labels := registry.baseLabels()
		labels["method"] = key.method
		labels["route"] = key.route
		labels["status"] = strconv.Itoa(key.status)
		series := registry.requests[key]
		writeSample(writer, "seoblog_http_requests_total", labels, float64(series.count))
		writeHistogram(writer, "seoblog_http_request_duration_seconds", labels, series)
	}

	writeMetricHeader(writer, "seoblog_content_cache_events_total", "Content cache outcomes; fallback means SQLite served after a cache failure.", "counter")
	for _, outcome := range sortedCounterKeys(registry.cacheEvents) {
		labels := registry.baseLabels()
		labels["outcome"] = outcome
		writeSample(writer, "seoblog_content_cache_events_total", labels, float64(registry.cacheEvents[outcome]))
	}

	writeMetricHeader(writer, "seoblog_worker_cycles_total", "Worker processing cycles by bounded component and outcome.", "counter")
	writeMetricHeader(writer, "seoblog_worker_items_total", "Items handled by worker component and outcome.", "counter")
	writeMetricHeader(writer, "seoblog_worker_cycle_duration_seconds", "Worker cycle latency by bounded component.", "histogram")
	components := make([]string, 0, len(registry.workerCycles))
	for component := range registry.workerCycles {
		components = append(components, component)
	}
	sort.Strings(components)
	for _, component := range components {
		for _, outcome := range sortedCounterKeys(registry.workerCycles[component]) {
			labels := registry.baseLabels()
			labels["component"] = component
			labels["outcome"] = outcome
			writeSample(writer, "seoblog_worker_cycles_total", labels, float64(registry.workerCycles[component][outcome]))
			writeSample(writer, "seoblog_worker_items_total", labels, float64(registry.workerItems[component][outcome]))
		}
		labels := registry.baseLabels()
		labels["component"] = component
		writeHistogram(writer, "seoblog_worker_cycle_duration_seconds", labels, registry.workerDuration[component])
	}
}

func (registry *Registry) renderDurable(ctx context.Context, writer io.Writer) error {
	if registry.db == nil {
		return nil
	}
	stats := registry.db.Stats()
	writeMetricHeader(writer, "seoblog_sqlite_connections", "SQLite database/sql connection state.", "gauge")
	for state, value := range map[string]int{"open": stats.OpenConnections, "in_use": stats.InUse, "idle": stats.Idle} {
		labels := registry.baseLabels()
		labels["state"] = state
		writeSample(writer, "seoblog_sqlite_connections", labels, float64(value))
	}
	writeMetricHeader(writer, "seoblog_sqlite_wait_total", "Number of waits for the single SQLite connection.", "counter")
	writeSample(writer, "seoblog_sqlite_wait_total", registry.baseLabels(), float64(stats.WaitCount))
	writeMetricHeader(writer, "seoblog_sqlite_wait_seconds_total", "Time spent waiting for a SQLite connection.", "counter")
	writeSample(writer, "seoblog_sqlite_wait_seconds_total", registry.baseLabels(), stats.WaitDuration.Seconds())
	writeMetricHeader(writer, "seoblog_sqlite_database_bytes", "SQLite primary and WAL file sizes.", "gauge")
	for kind, path := range map[string]string{"primary": registry.dbPath, "wal": registry.dbPath + "-wal"} {
		if info, err := os.Stat(path); err == nil {
			labels := registry.baseLabels()
			labels["file"] = kind
			writeSample(writer, "seoblog_sqlite_database_bytes", labels, float64(info.Size()))
		}
	}

	if err := registry.renderOutbox(ctx, writer); err != nil {
		return err
	}
	if err := registry.renderStatuses(ctx, writer, "seoblog_webhook_attempts", "webhook_attempts"); err != nil {
		return err
	}
	if err := registry.renderStatuses(ctx, writer, "seoblog_media_assets", "assets"); err != nil {
		return err
	}
	if err := registry.renderStatuses(ctx, writer, "seoblog_ai_jobs", "ai_jobs"); err != nil {
		return err
	}
	if err := registry.renderStatuses(ctx, writer, "seoblog_email_notifications", "review_assignment_notifications"); err != nil {
		return err
	}

	writeMetricHeader(writer, "seoblog_webhook_attempt_retries_total", "Durable webhook delivery attempts beyond the initial try.", "gauge")
	var retries float64
	if err := registry.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(MAX(attempt_count - 1, 0)), 0) FROM webhook_attempts`).Scan(&retries); err != nil {
		return fmt.Errorf("collect webhook retries: %w", err)
	}
	writeSample(writer, "seoblog_webhook_attempt_retries_total", registry.baseLabels(), retries)

	writeMetricHeader(writer, "seoblog_webhook_delivery_duration_seconds", "Completed webhook receiver latency.", "summary")
	var durationCount int64
	var durationSumMillis float64
	if err := registry.db.QueryRowContext(ctx, `SELECT COUNT(response_duration_ms), COALESCE(SUM(response_duration_ms), 0) FROM webhook_attempts WHERE response_duration_ms IS NOT NULL`).Scan(&durationCount, &durationSumMillis); err != nil {
		return fmt.Errorf("collect webhook duration: %w", err)
	}
	writeSample(writer, "seoblog_webhook_delivery_duration_seconds_count", registry.baseLabels(), float64(durationCount))
	writeSample(writer, "seoblog_webhook_delivery_duration_seconds_sum", registry.baseLabels(), durationSumMillis/1000)

	writeMetricHeader(writer, "seoblog_ai_estimated_cost_cents_total", "Recorded estimated AI spend in cents.", "gauge")
	var cost float64
	if err := registry.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(estimated_cost_cents), 0) FROM ai_jobs`).Scan(&cost); err != nil {
		return fmt.Errorf("collect AI cost: %w", err)
	}
	writeSample(writer, "seoblog_ai_estimated_cost_cents_total", registry.baseLabels(), cost)
	return nil
}

func (registry *Registry) renderOutbox(ctx context.Context, writer io.Writer) error {
	writeMetricHeader(writer, "seoblog_outbox_pending_events", "Events not yet fanned out to all configured webhook destinations.", "gauge")
	writeMetricHeader(writer, "seoblog_outbox_oldest_pending_age_seconds", "Age of the oldest event awaiting webhook fan-out.", "gauge")
	var pending int64
	var oldest sql.NullFloat64
	err := registry.db.QueryRowContext(ctx, `
		SELECT COUNT(1), MAX(0, COALESCE((julianday(?) - julianday(MIN(created_at))) * 86400, 0))
		FROM outbox_events
		WHERE webhook_fanned_out_at IS NULL
	`, time.Now().UTC().Format(time.RFC3339Nano)).Scan(&pending, &oldest)
	if err != nil {
		return fmt.Errorf("collect outbox state: %w", err)
	}
	writeSample(writer, "seoblog_outbox_pending_events", registry.baseLabels(), float64(pending))
	writeSample(writer, "seoblog_outbox_oldest_pending_age_seconds", registry.baseLabels(), oldest.Float64)
	return nil
}

func (registry *Registry) renderStatuses(ctx context.Context, writer io.Writer, metric, table string) error {
	writeMetricHeader(writer, metric, "Durable records by processing status.", "gauge")
	rows, err := registry.db.QueryContext(ctx, "SELECT status, COUNT(1) FROM "+table+" GROUP BY status ORDER BY status")
	if err != nil {
		return fmt.Errorf("collect %s statuses: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return err
		}
		labels := registry.baseLabels()
		labels["status"] = status
		writeSample(writer, metric, labels, float64(count))
	}
	return rows.Err()
}

func (registry *Registry) baseLabels() map[string]string {
	return map[string]string{"environment": registry.environment, "service": registry.service}
}

func recordDuration[K comparable](series map[K]*durationSeries, key K, duration time.Duration) {
	item := series[key]
	if item == nil {
		item = &durationSeries{}
		series[key] = item
	}
	recordDurationSeries(item, duration)
}

func recordDurationSeries(series *durationSeries, duration time.Duration) {
	seconds := duration.Seconds()
	series.count++
	series.sum += seconds
	for index, upperBound := range durationBuckets {
		if seconds <= upperBound {
			series.buckets[index]++
		}
	}
}

func writeHistogram(writer io.Writer, metric string, labels map[string]string, series *durationSeries) {
	if series == nil {
		return
	}
	for index, upperBound := range durationBuckets {
		bucketLabels := copyLabels(labels)
		bucketLabels["le"] = strconv.FormatFloat(upperBound, 'g', -1, 64)
		writeSample(writer, metric+"_bucket", bucketLabels, float64(series.buckets[index]))
	}
	infinityLabels := copyLabels(labels)
	infinityLabels["le"] = "+Inf"
	writeSample(writer, metric+"_bucket", infinityLabels, float64(series.count))
	writeSample(writer, metric+"_sum", labels, series.sum)
	writeSample(writer, metric+"_count", labels, float64(series.count))
}

func writeMetricHeader(writer io.Writer, name, help, metricType string) {
	_, _ = fmt.Fprintf(writer, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, metricType)
}

func writeSample(writer io.Writer, name string, labels map[string]string, value float64) {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	_, _ = io.WriteString(writer, name)
	if len(keys) > 0 {
		_, _ = io.WriteString(writer, "{")
		for index, key := range keys {
			if index > 0 {
				_, _ = io.WriteString(writer, ",")
			}
			_, _ = fmt.Fprintf(writer, `%s="%s"`, key, escapeLabel(labels[key]))
		}
		_, _ = io.WriteString(writer, "}")
	}
	_, _ = fmt.Fprintf(writer, " %s\n", strconv.FormatFloat(value, 'g', -1, 64))
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func copyLabels(labels map[string]string) map[string]string {
	result := make(map[string]string, len(labels)+1)
	for key, value := range labels {
		result[key] = value
	}
	return result
}

func sortedCounterKeys(values map[string]uint64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func boundedRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return "unmatched"
	}
	if len(route) > 200 {
		return "unmatched"
	}
	return route
}

func boundedOutcome(outcome string) string {
	switch outcome {
	case "success", "error", "retry", "cancelled", "hit", "miss", "fallback", "disabled":
		return outcome
	default:
		return "other"
	}
}

func boundedComponent(component string) string {
	switch component {
	case "scheduler", "webhook", "email", "media", "ai":
		return component
	default:
		return "other"
	}
}
