package health

import (
	"fmt"
	"time"
)

// HealthState 后端健康状态
// 0: 健康
// 1: 可疑
// 2: 不健康
type HealthState int

const (
	Healthy HealthState = iota
	Suspect
	Unhealthy
)

func (h HealthState) String() string {
	switch h {
	case Healthy:
		return "healthy"
	case Suspect:
		return "suspect"
	case Unhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// Thresholds 阈值常量
const (
	HealthyToSuspectThreshold   = 3 // 健康到可疑阈值
	SuspectToUnhealthyThreshold = 2 // 可疑到不健康阈值
	UnhealthyToSuspectThreshold = 3 // 不健康到可疑阈值
	SuspectToHealthyThreshold   = 2 // 可疑到健康阈值
)

type BackendHealth struct {
	ID                 string
	Addr               string
	State              HealthState
	ConsecutiveSuccess int       // 连续成功次数h
	ConsecutiveFail    int       // 连续失败次数
	LastCheck          time.Time // 最后检查时间
	LastLatency        float64   // 最后检查延迟
}

// IsHealthy 判断后端是否可以接收流量
// Healthy 和 Suspect 都可以接收，只是 Suspect 打分会被降低
func (s HealthState) IsHealthy() bool {
	return s == Healthy || s == Suspect
}

// IsUnhealthy 判断后端是否已确认故障
func (s HealthState) IsUnhealthy() bool {
	return s == Unhealthy
}

func TransitionState(state HealthState, consecutiveSuccess int, consecutiveFail int, success bool) (HealthState, int, int, error) {
	switch state {
	case Healthy:
		if success {
			consecutiveSuccess++
			consecutiveFail = 0
		} else {
			consecutiveFail++
			consecutiveSuccess = 0
		}
		if consecutiveFail >= HealthyToSuspectThreshold {
			return Suspect, consecutiveSuccess, consecutiveFail, nil
		}
		return Healthy, consecutiveSuccess, consecutiveFail, nil
	case Suspect:
		if success {
			consecutiveSuccess++
			consecutiveFail = 0
		} else {
			consecutiveFail++
			consecutiveSuccess = 0
		}
		if consecutiveFail >= SuspectToUnhealthyThreshold {
			return Unhealthy, consecutiveSuccess, consecutiveFail, nil
		}
		if consecutiveSuccess >= SuspectToHealthyThreshold {
			return Healthy, consecutiveSuccess, consecutiveFail, nil
		}
		return Suspect, consecutiveSuccess, consecutiveFail, nil
	case Unhealthy:
		if success {
			consecutiveSuccess++
			consecutiveFail = 0
		} else {
			consecutiveFail++
			consecutiveSuccess = 0
		}
		if consecutiveSuccess >= UnhealthyToSuspectThreshold {
			return Suspect, consecutiveSuccess, consecutiveFail, nil
		}
		return Unhealthy, consecutiveSuccess, consecutiveFail, nil
	}
	return Suspect, consecutiveSuccess, consecutiveFail, fmt.Errorf("unknown health state")
}
