package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	metricsPath     = flag.String("metrics-path", "/metrics", "")
	port            = flag.Int("port", 8080, "")
	rabbitmqctlPath = flag.String("rabbitmqctl-path", "/opt/rabbitmq/sbin/rabbitmqctl", "")
	refreshSecs     = flag.Int("refresh-seconds", 15, "")
)

const (
	namespace = "rabbitmq"
	subsystem = "queues_exporter"
)

var (
	consumers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "consumers",
	}, []string{"queue_name"})
	messagesTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "messages_total",
	}, []string{"queue_name"})
	messagesReady = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "messages_ready",
	}, []string{"queue_name"})
	messagesUnacknowledged = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "messages_unacknowledged",
	}, []string{"queue_name"})
)

type queueMetrics struct {
	name                                    string
	consumers, msgTotal, msgReady, msgUnack int
}

type healthStatus struct {
	sync.Mutex
	healthy bool
}

func (h *healthStatus) isHealthy() bool {
	h.Lock()
	defer h.Unlock()
	return h.healthy
}

func (h *healthStatus) setHealthy(hl bool) {
	h.Lock()
	defer h.Unlock()
	h.healthy = hl
}

func main() {
	flag.Parse()
	if nil == metricsPath || !strings.HasSuffix(*metricsPath, "/") {
		panic(`metrics path must be specified and starts with "/"`)
	}
	if nil == port || *port < 1 {
		panic("port must be specified and be greater then 0")
	}
	if nil == rabbitmqctlPath {
		panic("path to rabbitmqctl command must be specified")
	}
	if nil == refreshSecs || *refreshSecs < 5 {
		panic("refresh interval must be specified and be greater or equal then 5")
	}

	// test if rabbitmqctl is available
	_, err := execRabbitmqctlListQueues()
	if err != nil {
		panic(err)
	}

	healthStatus := &healthStatus{
		healthy: true,
	}

	go func() {
		for {
			out, err := execRabbitmqctlListQueues()
			if err != nil {
				log.Printf("failed exec rabbitmqctl command: %s\n", err.Error())
				healthStatus.setHealthy(false)
				time.Sleep(time.Duration(*refreshSecs) * time.Second)
				continue
			}
			metrics, err := parseRabbitmqctlOutput(out)
			if err != nil {
				log.Printf("failed parse rabbitmqctl output: %s\n%s", err.Error(), string(out))
				healthStatus.setHealthy(false)
				time.Sleep(time.Duration(*refreshSecs) * time.Second)
				continue
			}

			for _, m := range metrics {
				consumers.WithLabelValues(m.name).Set(float64(m.consumers))
				messagesTotal.WithLabelValues(m.name).Set(float64(m.msgTotal))
				messagesReady.WithLabelValues(m.name).Set(float64(m.msgReady))
				messagesUnacknowledged.WithLabelValues(m.name).Set(float64(m.msgUnack))
			}

			healthStatus.setHealthy(true)
			time.Sleep(time.Duration(*refreshSecs) * time.Second)
		}
	}()

	r := prometheus.NewRegistry()
	r.MustRegister(consumers, messagesTotal, messagesReady, messagesUnacknowledged)
	handler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})
	http.Handle(*metricsPath, handler)
	http.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthStatus.Lock()
		defer healthStatus.Unlock()

		s := http.StatusOK
		if !healthStatus.healthy {
			s = http.StatusInternalServerError
		}
		w.WriteHeader(s)
	}))
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

func execRabbitmqctlListQueues() ([]byte, error) {
	return exec.Command(*rabbitmqctlPath, "list_queues", "-s", "name,consumers,messages,messages_ready,messages_unacknowledged").Output()
}

func parseRabbitmqctlOutput(out []byte) ([]queueMetrics, error) {
	r := csv.NewReader(bytes.NewBuffer(out))
	r.Comma = '\t'
	r.FieldsPerRecord = 5
	var metrics []queueMetrics
	for {
		line, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		m := queueMetrics{
			name:      line[0],
			consumers: mustAtoi(line[1]),
			msgTotal:  mustAtoi(line[2]),
			msgReady:  mustAtoi(line[3]),
			msgUnack:  mustAtoi(line[4]),
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func mustAtoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return i
}
