package common

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// AdaptiveRetryDelayConfig controls dynamic retry delay tuning based on system CPU usage.
// It is intentionally env-driven (not option/db-driven) to keep rollout simple.
type AdaptiveRetryDelayConfig struct {
	Enabled      bool
	CPUThreshold int // 0-100 (%)
	Step         time.Duration
	Max          time.Duration
}

var adaptiveRetryDelayConfig atomic.Value // AdaptiveRetryDelayConfig
var adaptiveRetryDelayNS atomic.Int64     // current delay in nanoseconds

func init() {
	adaptiveRetryDelayConfig.Store(AdaptiveRetryDelayConfig{
		Enabled:      false,
		CPUThreshold: 50,
		Step:         10 * time.Millisecond,
		Max:          1 * time.Second,
	})
	adaptiveRetryDelayNS.Store(0)
}

func GetAdaptiveRetryDelayConfig() AdaptiveRetryDelayConfig {
	return adaptiveRetryDelayConfig.Load().(AdaptiveRetryDelayConfig)
}

func SetAdaptiveRetryDelayConfig(config AdaptiveRetryDelayConfig) {
	if config.CPUThreshold < 0 || config.CPUThreshold > 100 {
		SysError(fmt.Sprintf("invalid RETRY_DELAY_CPU_THRESHOLD=%d, fallback to 50", config.CPUThreshold))
		config.CPUThreshold = 50
	}
	if config.Step <= 0 {
		SysError(fmt.Sprintf("invalid RETRY_DELAY_STEP_MS=%s, fallback to 10ms", config.Step))
		config.Step = 10 * time.Millisecond
	}
	if config.Max <= 0 {
		SysError(fmt.Sprintf("invalid RETRY_DELAY_MAX_MS=%s, fallback to 1s", config.Max))
		config.Max = 1 * time.Second
	}
	if config.Max < config.Step {
		SysError(fmt.Sprintf("invalid retry delay config: max(%s) < step(%s), fallback to max=1s", config.Max, config.Step))
		config.Max = 1 * time.Second
		if config.Max < config.Step {
			config.Max = config.Step
		}
	}
	adaptiveRetryDelayConfig.Store(config)
	if !config.Enabled {
		adaptiveRetryDelayNS.Store(0)
	}
}

// GetAdaptiveRetryDelay returns the current delay to apply between retries.
func GetAdaptiveRetryDelay() time.Duration {
	config := GetAdaptiveRetryDelayConfig()
	if !config.Enabled {
		return 0
	}
	ns := adaptiveRetryDelayNS.Load()
	if ns <= 0 {
		return 0
	}
	return time.Duration(ns)
}

// AdjustAdaptiveRetryDelay should be called when system CPU usage is sampled.
// Rule:
//   - cpuUsage > threshold: delay += 10ms
//   - cpuUsage <= threshold: delay -= 10ms
//   - clamp delay to [0, 1s]
func AdjustAdaptiveRetryDelay(cpuUsage float64) {
	config := GetAdaptiveRetryDelayConfig()
	if !config.Enabled {
		return
	}

	stepNS := int64(config.Step)
	maxNS := int64(config.Max)

	current := adaptiveRetryDelayNS.Load()
	if cpuUsage > float64(config.CPUThreshold) {
		current += stepNS
	} else {
		current -= stepNS
	}

	if current < 0 {
		current = 0
	} else if current > maxNS {
		current = maxNS
	}
	adaptiveRetryDelayNS.Store(current)
}

// SleepAdaptiveRetryDelay sleeps for current delay between retries.
// It returns false if ctx is canceled while waiting.
func SleepAdaptiveRetryDelay(ctx context.Context) bool {
	delay := GetAdaptiveRetryDelay()
	if delay <= 0 {
		return true
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
