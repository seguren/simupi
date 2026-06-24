package sensor

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"simupi/internal/config"
)

// Publisher is implemented by the MQTT client (Stage 6).
type Publisher interface {
	PublishSensor(name string, value float64)
}

// HistoryRecorder is implemented by HistoryManager (Stage 5).
type HistoryRecorder interface {
	Add(sensor string, value float64)
}

// SensorEngine runs one goroutine per sensor, computing and publishing
// values at each sensor's configured interval.
type SensorEngine struct {
	cfg       map[string]config.SensorConfig
	publisher Publisher
	history   HistoryRecorder

	mu        sync.RWMutex
	values    map[string]float64
	failures  map[string]config.FailureMode
	rampDir   map[string]int // +1 ascending, -1 descending

	startTime time.Time
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewSensorEngine(cfg map[string]config.SensorConfig, pub Publisher, hist HistoryRecorder) *SensorEngine {
	e := &SensorEngine{
		cfg:       cfg,
		publisher: pub,
		history:   hist,
		values:    make(map[string]float64, len(cfg)),
		failures:  make(map[string]config.FailureMode),
		rampDir:   make(map[string]int),
		startTime: time.Now(),
	}
	for name, c := range cfg {
		e.values[name] = c.Min
		e.rampDir[name] = 1
	}
	return e
}

func (e *SensorEngine) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	for name, c := range e.cfg {
		e.wg.Add(1)
		go e.runSensor(ctx, name, c)
	}
}

func (e *SensorEngine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
}

func (e *SensorEngine) runSensor(ctx context.Context, name string, cfg config.SensorConfig) {
	defer e.wg.Done()
	interval := cfg.Interval
	if interval <= 0 {
		interval = 1
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.mu.Lock()
			failure := e.failures[name]
			var value float64
			if failure.Mode == "offline" {
				e.mu.Unlock()
				continue
			}
			if failure.Mode == "stuck" {
				value = e.values[name]
			} else {
				value = e.computeLocked(name, cfg)
				e.values[name] = value
			}
			e.mu.Unlock()

			rounded := math.Round(value*100) / 100
			e.history.Add(name, rounded)
			e.publisher.PublishSensor(name, rounded)
		}
	}
}

// computeLocked returns the next value for a sensor. Must be called with e.mu held.
func (e *SensorEngine) computeLocked(name string, cfg config.SensorConfig) float64 {
	switch cfg.Mode {
	case "wave":
		period := cfg.Period
		if period <= 0 {
			period = 60
		}
		elapsed := time.Since(e.startTime).Seconds()
		center := (cfg.Min + cfg.Max) / 2
		amplitude := (cfg.Max - cfg.Min) / 2
		return center + amplitude*math.Sin(2*math.Pi*elapsed/period)

	case "ramp":
		step := (cfg.Max - cfg.Min) / 20
		v := e.values[name] + float64(e.rampDir[name])*step
		if v >= cfg.Max {
			v = cfg.Max
			e.rampDir[name] = -1
		} else if v <= cfg.Min {
			v = cfg.Min
			e.rampDir[name] = 1
		}
		return v

	case "walk":
		step := cfg.Step
		if step <= 0 {
			step = (cfg.Max - cfg.Min) / 20
		}
		delta := (rand.Float64()*2 - 1) * step
		v := e.values[name] + delta
		if v > cfg.Max {
			v = cfg.Max
		} else if v < cfg.Min {
			v = cfg.Min
		}
		return v

	case "square":
		period := cfg.Period
		if period <= 0 {
			period = 60
		}
		duty := cfg.Duty
		if duty <= 0 || duty >= 1 {
			duty = 0.5
		}
		elapsed := time.Since(e.startTime).Seconds()
		phase := math.Mod(elapsed, period) / period
		if phase < duty {
			return cfg.Max
		}
		return cfg.Min

	case "triangle":
		period := cfg.Period
		if period <= 0 {
			period = 60
		}
		elapsed := time.Since(e.startTime).Seconds()
		phase := math.Mod(elapsed, period) / period
		var t float64
		if phase < 0.5 {
			t = phase * 2
		} else {
			t = (1 - phase) * 2
		}
		return cfg.Min + (cfg.Max-cfg.Min)*t

	case "sawtooth":
		period := cfg.Period
		if period <= 0 {
			period = 60
		}
		elapsed := time.Since(e.startTime).Seconds()
		phase := math.Mod(elapsed, period) / period
		return cfg.Min + (cfg.Max-cfg.Min)*phase

	default: // random
		return cfg.Min + rand.Float64()*(cfg.Max-cfg.Min)
	}
}

// ── Public API ────────────────────────────────────────────────────────────────

func (e *SensorEngine) Config() map[string]config.SensorConfig {
	return e.cfg
}

func (e *SensorEngine) GetValues() map[string]float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make(map[string]float64, len(e.values))
	for k, v := range e.values {
		out[k] = v
	}
	return out
}

func (e *SensorEngine) SetValue(name string, value float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.values[name]; ok {
		e.values[name] = value
	}
}

func (e *SensorEngine) SetFailure(name, mode string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failures[name] = config.FailureMode{Mode: mode}
}

func (e *SensorEngine) ClearFailure(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.failures, name)
}

func (e *SensorEngine) GetFailures() map[string]config.FailureMode {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make(map[string]config.FailureMode, len(e.failures))
	for k, v := range e.failures {
		out[k] = v
	}
	return out
}

// Restore sets sensor values from a previously saved state.
// For ramp sensors the direction is inferred from the restored value.
func (e *SensorEngine) Restore(sensors map[string]float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for name, value := range sensors {
		if _, ok := e.values[name]; !ok {
			continue
		}
		e.values[name] = value
		if c, ok := e.cfg[name]; ok && c.Mode == "ramp" {
			mid := (c.Min + c.Max) / 2
			if value >= mid {
				e.rampDir[name] = -1
			} else {
				e.rampDir[name] = 1
			}
		}
	}
}
