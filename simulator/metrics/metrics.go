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
)
