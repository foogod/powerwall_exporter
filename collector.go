package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
)

type powerwallCollector struct{
	dummy float64
}

var (
	dummyDesc = prometheus.NewDesc("powerwall_dummy", "Dummy metric", []string{"target"}, nil)
)

func (p *powerwallCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- dummyDesc
}

func (p *powerwallCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debug("Collecting metrics...")
	ch <- prometheus.MustNewConstMetric(dummyDesc, prometheus.GaugeValue, p.dummy, "foo")
}
