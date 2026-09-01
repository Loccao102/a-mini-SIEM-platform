package metrics

import (
	"context"
	"sync"
	"time"
)

// QueueMetrics tracks pipeline queue health
type QueueMetrics struct {
	mu sync.RWMutex

	// Ingest metrics
	IngestCount       int64
	IngestErrors      int64
	IngestLatency     []time.Duration
	IngestLatencyBuf  int

	// Parser metrics
	ParserCount       int64
	ParserErrors      int64
	ParserLatency     []time.Duration
	ParserLatencyBuf  int

	// Queue lag metrics
	IngestQueueLag    time.Duration
	ParserQueueLag    time.Duration
	RetryQueueCount   int64
	DLQCount          int64

	// Elasticsearch metrics
	ESIndexCount      int64
	ESIndexErrors     int64
	ESIndexLatency    []time.Duration
	ESIndexLatencyBuf int

	// Deduplication metrics
	DeduplicateHits   int64
	DeduplicateMisses int64

	lastReset time.Time
}

// New creates a new metrics tracker
func New() *QueueMetrics {
	return &QueueMetrics{
		IngestLatency:    make([]time.Duration, 1000),
		IngestLatencyBuf: 0,
		ParserLatency:    make([]time.Duration, 1000),
		ParserLatencyBuf: 0,
		ESIndexLatency:   make([]time.Duration, 1000),
		ESIndexLatencyBuf: 0,
		lastReset:        time.Now(),
	}
}

// RecordIngestEvent records an ingest operation
func (m *QueueMetrics) RecordIngestEvent(latency time.Duration, isError bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.IngestCount++
	if isError {
		m.IngestErrors++
	}

	// Store latency (circular buffer)
	if m.IngestLatencyBuf >= len(m.IngestLatency) {
		m.IngestLatencyBuf = 0
	}
	m.IngestLatency[m.IngestLatencyBuf] = latency
	m.IngestLatencyBuf++
}

// RecordParserEvent records a parser operation
func (m *QueueMetrics) RecordParserEvent(latency time.Duration, isError bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ParserCount++
	if isError {
		m.ParserErrors++
	}

	if m.ParserLatencyBuf >= len(m.ParserLatency) {
		m.ParserLatencyBuf = 0
	}
	m.ParserLatency[m.ParserLatencyBuf] = latency
	m.ParserLatencyBuf++
}

// RecordESIndexEvent records an Elasticsearch indexing operation
func (m *QueueMetrics) RecordESIndexEvent(latency time.Duration, isError bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ESIndexCount++
	if isError {
		m.ESIndexErrors++
	}

	if m.ESIndexLatencyBuf >= len(m.ESIndexLatency) {
		m.ESIndexLatencyBuf = 0
	}
	m.ESIndexLatency[m.ESIndexLatencyBuf] = latency
	m.ESIndexLatencyBuf++
}

// RecordDeduplication records a deduplication hit/miss
func (m *QueueMetrics) RecordDeduplication(isHit bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if isHit {
		m.DeduplicateHits++
	} else {
		m.DeduplicateMisses++
	}
}

// UpdateQueueStats updates queue-based metrics from Redis
func (m *QueueMetrics) UpdateQueueStats(ingestLag, parserLag time.Duration, retryCount, dlqCount int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.IngestQueueLag = ingestLag
	m.ParserQueueLag = parserLag
	m.RetryQueueCount = retryCount
	m.DLQCount = dlqCount
}

// GetSnapshot returns a copy of current metrics
func (m *QueueMetrics) GetSnapshot(ctx context.Context) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ingestErrorRate := float64(0)
	if m.IngestCount > 0 {
		ingestErrorRate = float64(m.IngestErrors) / float64(m.IngestCount) * 100
	}

	parserErrorRate := float64(0)
	if m.ParserCount > 0 {
		parserErrorRate = float64(m.ParserErrors) / float64(m.ParserCount) * 100
	}

	esErrorRate := float64(0)
	if m.ESIndexCount > 0 {
		esErrorRate = float64(m.ESIndexErrors) / float64(m.ESIndexCount) * 100
	}

	dedupHitRate := float64(0)
	totalDedup := m.DeduplicateHits + m.DeduplicateMisses
	if totalDedup > 0 {
		dedupHitRate = float64(m.DeduplicateHits) / float64(totalDedup) * 100
	}

	return map[string]interface{}{
		"timestamp":        time.Now(),
		"uptime_seconds":   time.Since(m.lastReset).Seconds(),
		"ingest": map[string]interface{}{
			"total_count":  m.IngestCount,
			"error_count":  m.IngestErrors,
			"error_rate":   ingestErrorRate,
			"avg_latency":  m.getAverageLatency(m.IngestLatency),
			"p95_latency":  m.getPercentileLatency(m.IngestLatency, 0.95),
			"p99_latency":  m.getPercentileLatency(m.IngestLatency, 0.99),
			"queue_lag":    m.IngestQueueLag.Milliseconds(),
		},
		"parser": map[string]interface{}{
			"total_count": m.ParserCount,
			"error_count": m.ParserErrors,
			"error_rate":  parserErrorRate,
			"avg_latency": m.getAverageLatency(m.ParserLatency),
			"p95_latency": m.getPercentileLatency(m.ParserLatency, 0.95),
			"p99_latency": m.getPercentileLatency(m.ParserLatency, 0.99),
			"queue_lag":   m.ParserQueueLag.Milliseconds(),
		},
		"elasticsearch": map[string]interface{}{
			"total_count": m.ESIndexCount,
			"error_count": m.ESIndexErrors,
			"error_rate":  esErrorRate,
			"avg_latency": m.getAverageLatency(m.ESIndexLatency),
			"p95_latency": m.getPercentileLatency(m.ESIndexLatency, 0.95),
			"p99_latency": m.getPercentileLatency(m.ESIndexLatency, 0.99),
		},
		"deduplication": map[string]interface{}{
			"total_hits":  m.DeduplicateHits,
			"total_misses": m.DeduplicateMisses,
			"hit_rate":    dedupHitRate,
		},
		"queues": map[string]interface{}{
			"retry_count": m.RetryQueueCount,
			"dlq_count":   m.DLQCount,
		},
	}
}

// Helper to calculate average latency
func (m *QueueMetrics) getAverageLatency(latencies []time.Duration) int64 {
	if len(latencies) == 0 {
		return 0
	}

	total := int64(0)
	count := 0
	for _, l := range latencies {
		if l > 0 {
			total += l.Milliseconds()
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return total / int64(count)
}

// Helper to calculate percentile latency
func (m *QueueMetrics) getPercentileLatency(latencies []time.Duration, percentile float64) int64 {
	if len(latencies) == 0 {
		return 0
	}

	// Collect non-zero latencies
	valid := []int64{}
	for _, l := range latencies {
		if l > 0 {
			valid = append(valid, l.Milliseconds())
		}
	}

	if len(valid) == 0 {
		return 0
	}

	// Simple percentile calculation (not optimal but good enough)
	idx := int(float64(len(valid)) * percentile)
	if idx >= len(valid) {
		idx = len(valid) - 1
	}

	return valid[idx]
}

// Reset clears all metrics
func (m *QueueMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.IngestCount = 0
	m.IngestErrors = 0
	m.IngestLatencyBuf = 0
	m.ParserCount = 0
	m.ParserErrors = 0
	m.ParserLatencyBuf = 0
	m.ESIndexCount = 0
	m.ESIndexErrors = 0
	m.ESIndexLatencyBuf = 0
	m.DeduplicateHits = 0
	m.DeduplicateMisses = 0
	m.lastReset = time.Now()
}
