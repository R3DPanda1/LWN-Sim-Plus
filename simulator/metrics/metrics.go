package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	DevicesTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lwnsim_devices_total",
		Help: "Total number of devices by state",
	}, []string{"state"})

	GatewaysTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lwnsim_gateways_total",
		Help: "Total number of gateways by state",
	}, []string{"state"})

	UplinksTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "lwnsim_uplinks_total",
		Help: "Total uplinks sent",
	})

	DownlinksTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "lwnsim_downlinks_total",
		Help: "Total downlinks received",
	})

	WorkQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "lwnsim_work_queue_depth",
		Help: "Current depth of scheduler work queue",
	})

	WorkQueueCapacity = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "lwnsim_work_queue_capacity",
		Help: "Capacity of scheduler work queue",
	})

	JobExecutionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "lwnsim_job_execution_duration_seconds",
		Help:    "Duration of device job execution",
		Buckets: prometheus.DefBuckets,
	})

	EventsPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lwnsim_events_published_total",
		Help: "Total events published by type",
	}, []string{"type"})

	EventSubscriptions = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "lwnsim_event_subscriptions_total",
		Help: "Current number of active event subscriptions",
	})
)
