// internal/metrics/metrics.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HttpRequestsTotal 记录 HTTP 请求的总数
	HttpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of http requests handled by the service.",
		},
		[]string{"path", "method", "code"}, // 按路径、方法、状态码分类
	)

	// JobExecutionTotal 记录任务执行的总数
	JobExecutionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_executions_total",
			Help: "Total number of cron job executions.",
		},
		[]string{"job_name", "status"}, // 按任务名、执行状态 (success/failed) 分类
	)

	// IsLeader 标记当前节点是否为 Leader
	IsLeader = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "is_leader",
			Help: "Is this node currently the leader. 1 if leader, 0 otherwise.",
		},
		[]string{"node_id"},
	)
)

// Register a new function to be called from main.go
// This is not strictly necessary with promauto, but it's good practice
// to have an explicit registration point.
func Register() {
	// promauto.New... automatically registers the metric.
}
