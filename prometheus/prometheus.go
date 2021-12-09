package plmxs

import (
	"emt/log"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

const (
	readSuccess = 200
	listenAddr  = "0.0.0.0:6061"
)

func metris() {
	go func() {
		http.Handle("/heart", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(readSuccess)
		}))

		http.Handle("/metrics", promhttp.Handler())

		if err := http.ListenAndServe(listenAddr, nil); err != nil {
			log.Error("ListenAndServe", zap.String("data", err.Error()))
		}
	}()
}

type PrometheusMonitor struct {
	sync.RWMutex
	ServiceName string // 监控服务的名称

	APIRequestsCounter *prometheus.CounterVec // 每个接口的连接总数
	RequestDuration    *prometheus.HistogramVec
	RequestSize        *prometheus.HistogramVec
	ResponseSize       *prometheus.HistogramVec

	APIRequestsGauge       *prometheus.GaugeVec   // 接口每分钟的请求数
	ServiceRequestsCounter *prometheus.CounterVec // 接口连接总数

	APIRequestSuccessCounter *prometheus.CounterVec // 接口的连接成功率总数  单表

	handlerRequestsNum map[string]float64

	MemoryUseGauge *prometheus.GaugeVec // 内存使用量/分钟

	MemoryPercent *prometheus.GaugeVec // 内存百分比
	CPUPercent    *prometheus.GaugeVec // cpu百分比

}

func NewPrometheusMonitor(namespace string) *PrometheusMonitor {
	// 每个接口的连接总数  单表
	APIRequestsCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "单个接口的连接总数",
		},
		[]string{"handler", "method", "code", "micro_name"},
	)

	// 接口的连接成功率总数  单表
	APIRequestSuccessCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_success_total",
			Help:      "单个接口的连接总数",
		},
		[]string{"code", "micro_name"},
	)

	// 单表
	APIRequestsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "http_requests_gauge",
			Help:      "接口调用情况",
		},
		[]string{"handler", "micro_name"},
	)

	// 接口连接总数  总表
	ServiceRequestsCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_service_total",
			Help: "接口连接总数",
		},
		[]string{"micro_name"},
	)

	// 总表
	memoryUseGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_use_gauge",
			Help: "内存",
		},
		[]string{"micro_name"},
	)

	memoryPercent := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_percent",
			Help: "内存占用百分比",
		},
		[]string{"micro_name"},
	)

	cpuPercent := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_percent",
			Help: "cpu占用百分比",
		},
		[]string{"micro_name"},
	)

	RequestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "A histogram of latencies for requests.",
		},
		[]string{"handler", "method", "code", "micro_name"},
	)

	RequestSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_size_bytes",
			Help:      "A histogram of request sizes for requests.",
		},
		[]string{"handler", "method", "code", "micro_name"},
	)

	ResponseSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_response_size_bytes",
			Help:      "A histogram of response sizes for requests.",
		},
		[]string{"handler", "method", "code", "micro_name"},
	)

	// 注册指标
	prometheus.MustRegister(APIRequestsCounter, RequestDuration, RequestSize, ResponseSize,
		APIRequestsGauge, memoryUseGauge, ServiceRequestsCounter, APIRequestSuccessCounter,
		memoryPercent, cpuPercent)

	p := &PrometheusMonitor{
		ServiceName:              namespace,
		APIRequestsCounter:       APIRequestsCounter,
		RequestDuration:          RequestDuration,
		RequestSize:              RequestSize,
		ResponseSize:             ResponseSize,
		APIRequestsGauge:         APIRequestsGauge,
		MemoryUseGauge:           memoryUseGauge,
		ServiceRequestsCounter:   ServiceRequestsCounter,
		APIRequestSuccessCounter: APIRequestSuccessCounter,
		MemoryPercent:            memoryPercent,
		CPUPercent:               cpuPercent,

		handlerRequestsNum: make(map[string]float64),
	}

	metris()

	p.system()

	return p
}
