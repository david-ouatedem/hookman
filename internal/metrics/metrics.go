package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// EventsTotal counts events ingested, labeled by topic.
	EventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hookman_events_total",
		Help: "Total number of events ingested.",
	}, []string{"topic"})

	// DeliveryAttemptsTotal counts delivery attempts, labeled by status.
	DeliveryAttemptsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "hookman_delivery_attempts_total",
		Help: "Total number of delivery attempts.",
	}, []string{"status"})

	// DeliveryDuration tracks delivery duration in seconds.
	DeliveryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "hookman_delivery_duration_seconds",
		Help:    "Histogram of webhook delivery durations in seconds.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"status"})

	// QueueDepth tracks current queue depth by status.
	QueueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "hookman_queue_depth",
		Help: "Current number of events in each status.",
	}, []string{"status"})

	// DeadLetterTotal counts events that entered the dead letter queue.
	DeadLetterTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "hookman_dead_letter_total",
		Help: "Total number of events moved to dead letter queue.",
	})
)

// RecordEventCreated increments the events counter for the given topic.
func RecordEventCreated(topic string) {
	EventsTotal.WithLabelValues(topic).Inc()
}

// RecordDeliveryAttempt records a delivery attempt's outcome and duration.
func RecordDeliveryAttempt(status string, durationMs int) {
	DeliveryAttemptsTotal.WithLabelValues(status).Inc()
	DeliveryDuration.WithLabelValues(status).Observe(float64(durationMs) / 1000.0)
}

// RecordQueueDepth sets the current queue depth for a status.
func RecordQueueDepth(status string, count int) {
	QueueDepth.WithLabelValues(status).Set(float64(count))
}

// RecordDeadLetter increments the dead letter counter.
func RecordDeadLetter() {
	DeadLetterTotal.Inc()
}
