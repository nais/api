package agent

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Request metrics
	chatRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_chat_requests_total",
			Help: "Total number of chat requests",
		},
		[]string{"status", "streaming"},
	)

	chatDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agent_chat_duration_seconds",
			Help:    "End-to-end request duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~100s
		},
		[]string{"streaming"},
	)

	chatTimeToFirstTokenSeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "agent_chat_time_to_first_token_seconds",
			Help:    "Time until first content chunk in streaming responses",
			Buckets: prometheus.ExponentialBuckets(0.05, 2, 10), // 50ms to ~50s
		},
	)

	chatTokensTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_chat_tokens_total",
			Help: "Total number of tokens used in chat requests",
		},
		[]string{"type", "streaming"}, // type: input/output
	)

	// LLM metrics
	llmRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_llm_requests_total",
			Help: "Total number of requests to LLM provider",
		},
		[]string{"status", "model"},
	)

	llmDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agent_llm_duration_seconds",
			Help:    "LLM response time in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
		},
		[]string{"model"},
	)

	llmTokensTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_llm_tokens_total",
			Help: "Total number of tokens used",
		},
		[]string{"type", "model"}, // type: input/output
	)

	// Tool metrics
	toolCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_tool_calls_total",
			Help: "Total number of tool invocations",
		},
		[]string{"tool_name", "status"},
	)

	toolDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agent_tool_duration_seconds",
			Help:    "Tool execution time in seconds",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10), // 10ms to ~10s
		},
		[]string{"tool_name"},
	)

	toolIterationsTotal = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "agent_tool_iterations_total",
			Help:    "Number of tool call rounds per request",
			Buckets: []float64{0, 1, 2, 3, 4, 5},
		},
	)

	// RAG metrics
	ragSearchesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_rag_searches_total",
			Help: "Total number of document search requests",
		},
		[]string{"status"},
	)

	ragDurationSeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "agent_rag_duration_seconds",
			Help:    "Document search latency in seconds",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
		},
	)

	ragDocumentsReturned = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "agent_rag_documents_returned",
			Help:    "Number of documents returned per search",
			Buckets: []float64{0, 1, 2, 3, 4, 5, 10},
		},
	)

	// Conversation metrics
	conversationsCreatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "agent_conversations_created_total",
			Help: "Total number of new conversations created",
		},
	)

	conversationsDeletedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_conversations_deleted_total",
			Help: "Total number of conversations deleted",
		},
		[]string{"reason"}, // user/auto
	)
)

// RecordChatRequest records metrics for a chat request.
func RecordChatRequest(status string, streaming bool, durationSeconds float64, inputTokens, outputTokens int) {
	streamingStr := "false"
	if streaming {
		streamingStr = "true"
	}
	chatRequestsTotal.WithLabelValues(status, streamingStr).Inc()
	chatDurationSeconds.WithLabelValues(streamingStr).Observe(durationSeconds)

	if inputTokens > 0 {
		chatTokensTotal.WithLabelValues("input", streamingStr).Add(float64(inputTokens))
	}
	if outputTokens > 0 {
		chatTokensTotal.WithLabelValues("output", streamingStr).Add(float64(outputTokens))
	}
}

// RecordTimeToFirstToken records the time to first token for streaming responses.
func RecordTimeToFirstToken(durationSeconds float64) {
	chatTimeToFirstTokenSeconds.Observe(durationSeconds)
}

// RecordLLMRequest records metrics for an LLM request.
func RecordLLMRequest(status, model string, durationSeconds float64, inputTokens, outputTokens int) {
	llmRequestsTotal.WithLabelValues(status, model).Inc()
	llmDurationSeconds.WithLabelValues(model).Observe(durationSeconds)
	if inputTokens > 0 {
		llmTokensTotal.WithLabelValues("input", model).Add(float64(inputTokens))
	}
	if outputTokens > 0 {
		llmTokensTotal.WithLabelValues("output", model).Add(float64(outputTokens))
	}
}

// RecordToolCall records metrics for a tool call.
func RecordToolCall(toolName, status string, durationSeconds float64) {
	toolCallsTotal.WithLabelValues(toolName, status).Inc()
	toolDurationSeconds.WithLabelValues(toolName).Observe(durationSeconds)
}

// RecordToolIterations records the number of tool iterations for a request.
func RecordToolIterations(iterations int) {
	toolIterationsTotal.Observe(float64(iterations))
}

// RecordRAGSearch records metrics for a RAG search.
func RecordRAGSearch(status string, durationSeconds float64, documentsReturned int) {
	ragSearchesTotal.WithLabelValues(status).Inc()
	ragDurationSeconds.Observe(durationSeconds)
	ragDocumentsReturned.Observe(float64(documentsReturned))
}

// RecordConversationCreated records a new conversation creation.
func RecordConversationCreated() {
	conversationsCreatedTotal.Inc()
}

// RecordConversationDeleted records a conversation deletion.
func RecordConversationDeleted(reason string) {
	conversationsDeletedTotal.WithLabelValues(reason).Inc()
}
