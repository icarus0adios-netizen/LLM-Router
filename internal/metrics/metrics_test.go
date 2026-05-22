package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRequestsTotal_Counter(t *testing.T) {
	RecordRequest("backend-1", 200, 0.5)
	RecordRequest("backend-1", 500, 1.0)
	RecordRequest("backend-2", 200, 0.3)

	expected := `
# HELP gateway_request_total 收到的请求总数，按后端和状态码分组。
# TYPE gateway_request_total counter
gateway_request_total{backend_id="backend-1",status="success"} 1
gateway_request_total{backend_id="backend-1",status="error"} 1
gateway_request_total{backend_id="backend-2",status="success"} 1
`
	if err := testutil.CollectAndCompare(RequestsTotal, strings.NewReader(expected)); err != nil {
		t.Errorf("RequestsTotal mismatch:\n%v", err)
	}
}

func TestRecordRequest_SuccessVsError(t *testing.T) {
	RecordRequest("b1", 200, 0.1)
	RecordRequest("b1", 201, 0.2)
	RecordRequest("b1", 399, 0.3)

	RecordRequest("b1", 400, 0.4)
	RecordRequest("b1", 500, 0.5)

	counter := RequestsTotal.WithLabelValues("b1", "success")
	if val := testutil.ToFloat64(counter); val != 3 {
		t.Errorf("expected success count=3, got %v", val)
	}

	counter = RequestsTotal.WithLabelValues("b1", "error")
	if val := testutil.ToFloat64(counter); val != 2 {
		t.Errorf("expected error count=2, got %v", val)
	}
}

func TestRequestDuration_Histogram(t *testing.T) {
	RequestDuration.Reset()
	RequestDuration.WithLabelValues("backend-1").Observe(0.05)
	RequestDuration.WithLabelValues("backend-1").Observe(0.5)
	RequestDuration.WithLabelValues("backend-2").Observe(2.0)

	expected := `
# HELP gateway_request_duration_seconds 请求延迟分布，单位秒。
# TYPE gateway_request_duration_seconds histogram
gateway_request_duration_seconds_bucket{backend_id="backend-1",le="0.005"} 0
gateway_request_duration_seconds_bucket{backend_id="backend-1",le="0.01"} 0
gateway_request_duration_seconds_bucket{backend_id="backend-1",le="0.025"} 0
gateway_request_duration_seconds_bucket{backend_id="backend-1",le="0.05"} 1
gateway_request_duration_seconds_bucket{backend_id="backend-1",le="0.1"} 1
gateway_request_duration_seconds_bucket{backend_id="backend-1",le="0.25"} 1
gateway_request_duration_seconds_bucket{backend_id="backend-1",le="0.5"} 2
gateway_request_duration_seconds_bucket{backend_id="backend-1",le="1"} 2
gateway_request_duration_seconds_bucket{backend_id="backend-1",le="2.5"} 2
gateway_request_duration_seconds_bucket{backend_id="backend-1",le="5"} 2
gateway_request_duration_seconds_bucket{backend_id="backend-1",le="10"} 2
gateway_request_duration_seconds_bucket{backend_id="backend-1",le="+Inf"} 2
gateway_request_duration_seconds_sum{backend_id="backend-1"} 0.55
gateway_request_duration_seconds_count{backend_id="backend-1"} 2
gateway_request_duration_seconds_bucket{backend_id="backend-2",le="0.005"} 0
gateway_request_duration_seconds_bucket{backend_id="backend-2",le="0.01"} 0
gateway_request_duration_seconds_bucket{backend_id="backend-2",le="0.025"} 0
gateway_request_duration_seconds_bucket{backend_id="backend-2",le="0.05"} 0
gateway_request_duration_seconds_bucket{backend_id="backend-2",le="0.1"} 0
gateway_request_duration_seconds_bucket{backend_id="backend-2",le="0.25"} 0
gateway_request_duration_seconds_bucket{backend_id="backend-2",le="0.5"} 0
gateway_request_duration_seconds_bucket{backend_id="backend-2",le="1"} 0
gateway_request_duration_seconds_bucket{backend_id="backend-2",le="2.5"} 1
gateway_request_duration_seconds_bucket{backend_id="backend-2",le="5"} 1
gateway_request_duration_seconds_bucket{backend_id="backend-2",le="10"} 1
gateway_request_duration_seconds_bucket{backend_id="backend-2",le="+Inf"} 1
gateway_request_duration_seconds_sum{backend_id="backend-2"} 2
gateway_request_duration_seconds_count{backend_id="backend-2"} 1
`
	if err := testutil.CollectAndCompare(RequestDuration, strings.NewReader(expected)); err != nil {
		t.Errorf("RequestDuration mismatch:\n%v", err)
	}
}

func TestGatewayErrorTotal_Counter(t *testing.T) {
	RecordError("admission_rejected")
	RecordError("admission_rejected")
	RecordError("no_backend")
	RecordError("backend_timeout")
	RecordError("backend_error")
	RecordError("backend_error")
	RecordError("backend_error")

	expected := `
# HELP gateway_error_total 路由错误总数，按错误类型分组。
# TYPE gateway_error_total counter
gateway_error_total{reason="admission_rejected"} 2
gateway_error_total{reason="no_backend"} 1
gateway_error_total{reason="backend_timeout"} 1
gateway_error_total{reason="backend_error"} 3
`
	if err := testutil.CollectAndCompare(GatewayErrorTotal, strings.NewReader(expected)); err != nil {
		t.Errorf("GatewayErrorTotal mismatch:\n%v", err)
	}
}

func TestBackendHealth_Gauge(t *testing.T) {
	SetBackendHealth("backend-1", 0)
	SetBackendHealth("backend-2", 1)
	SetBackendHealth("backend-3", 2)

	expected := `
# HELP gateway_backend_health 后端健康状态，0=Healthy, 1=Suspect, 2=Unhealthy
# TYPE gateway_backend_health gauge
gateway_backend_health{backend_id="backend-1"} 0
gateway_backend_health{backend_id="backend-2"} 1
gateway_backend_health{backend_id="backend-3"} 2
`
	if err := testutil.CollectAndCompare(BackendHealth, strings.NewReader(expected)); err != nil {
		t.Errorf("BackendHealth mismatch:\n%v", err)
	}

	SetBackendHealth("backend-1", 2)
	val := testutil.ToFloat64(BackendHealth.WithLabelValues("backend-1"))
	if val != 2 {
		t.Errorf("expected backend-1 health=2, got %v", val)
	}
}

func TestActiveRequests_Gauge(t *testing.T) {
	SetBackendActiveRequests("backend-1", 5)
	SetBackendActiveRequests("backend-2", 3)
	SetBackendActiveRequests("backend-3", 0)

	expected := `
# HELP gateway_active_requests 后端当前在飞请求数
# TYPE gateway_active_requests gauge
gateway_active_requests{backend_id="backend-1"} 5
gateway_active_requests{backend_id="backend-2"} 3
gateway_active_requests{backend_id="backend-3"} 0
`
	if err := testutil.CollectAndCompare(ActiveRequests, strings.NewReader(expected)); err != nil {
		t.Errorf("ActiveRequests mismatch:\n%v", err)
	}

	SetBackendActiveRequests("backend-1", 10)
	val := testutil.ToFloat64(ActiveRequests.WithLabelValues("backend-1"))
	if val != 10 {
		t.Errorf("expected backend-1 active=10, got %v", val)
	}
}

func TestRouterScore_Histogram(t *testing.T) {
	RouterScore.Reset()
	RecordRouterScore("backend-1", 25.5)
	RecordRouterScore("backend-1", 30.0)
	RecordRouterScore("backend-2", 10.0)

	if err := testutil.CollectAndCompare(RouterScore, strings.NewReader(`
# HELP gateway_router_score Router 后端选中的分数，分数越低越好
# TYPE gateway_router_score histogram
gateway_router_score_bucket{backend_id="backend-1",le="0.005"} 0
gateway_router_score_bucket{backend_id="backend-1",le="0.01"} 0
gateway_router_score_bucket{backend_id="backend-1",le="0.025"} 0
gateway_router_score_bucket{backend_id="backend-1",le="0.05"} 0
gateway_router_score_bucket{backend_id="backend-1",le="0.1"} 0
gateway_router_score_bucket{backend_id="backend-1",le="0.25"} 0
gateway_router_score_bucket{backend_id="backend-1",le="0.5"} 0
gateway_router_score_bucket{backend_id="backend-1",le="1"} 0
gateway_router_score_bucket{backend_id="backend-1",le="2.5"} 0
gateway_router_score_bucket{backend_id="backend-1",le="5"} 0
gateway_router_score_bucket{backend_id="backend-1",le="10"} 0
gateway_router_score_bucket{backend_id="backend-1",le="+Inf"} 2
gateway_router_score_sum{backend_id="backend-1"} 55.5
gateway_router_score_count{backend_id="backend-1"} 2
gateway_router_score_bucket{backend_id="backend-2",le="0.005"} 0
gateway_router_score_bucket{backend_id="backend-2",le="0.01"} 0
gateway_router_score_bucket{backend_id="backend-2",le="0.025"} 0
gateway_router_score_bucket{backend_id="backend-2",le="0.05"} 0
gateway_router_score_bucket{backend_id="backend-2",le="0.1"} 0
gateway_router_score_bucket{backend_id="backend-2",le="0.25"} 0
gateway_router_score_bucket{backend_id="backend-2",le="0.5"} 0
gateway_router_score_bucket{backend_id="backend-2",le="1"} 0
gateway_router_score_bucket{backend_id="backend-2",le="2.5"} 0
gateway_router_score_bucket{backend_id="backend-2",le="5"} 0
gateway_router_score_bucket{backend_id="backend-2",le="10"} 1
gateway_router_score_bucket{backend_id="backend-2",le="+Inf"} 1
gateway_router_score_sum{backend_id="backend-2"} 10
gateway_router_score_count{backend_id="backend-2"} 1
`)); err != nil {
		t.Errorf("RouterScore mismatch:\n%v", err)
	}
}

func TestRecordRequest_DurationTracking(t *testing.T) {
	RecordRequest("backend-test", 200, 0.123)

	metricFamily, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	found := false
	for _, mf := range metricFamily {
		if mf.GetName() == "gateway_request_duration_seconds" {
			found = true
			for _, m := range mf.GetMetric() {
				if m.GetHistogram().GetSampleCount() > 0 {
					return
				}
			}
		}
	}
	if !found {
		t.Error("gateway_request_duration_seconds metric not found")
	}
}

func TestMetricNames_NoTypos(t *testing.T) {
	metricNames := map[string]string{
		"gateway_request_total":            "RequestsTotal",
		"gateway_request_duration_seconds": "RequestDuration",
		"gateway_error_total":              "GatewayErrorTotal",
		"gateway_backend_health":           "BackendHealth",
		"gateway_active_requests":          "ActiveRequests",
		"gateway_router_score":             "RouterScore",
	}

	for expectedName, varName := range metricNames {
		t.Run(varName, func(t *testing.T) {
			metricFamily, err := prometheus.DefaultGatherer.Gather()
			if err != nil {
				t.Fatalf("gather failed: %v", err)
			}
			found := false
			for _, mf := range metricFamily {
				if mf.GetName() == expectedName {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("metric %q (from var %s) was not found in registry", expectedName, varName)
			}
		})
	}
}

func TestHelpStrings_AreSet(t *testing.T) {
	metricFamily, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	for _, mf := range metricFamily {
		if mf.GetHelp() == "" {
			t.Errorf("metric %q has empty help string", mf.GetName())
		}
	}
}
