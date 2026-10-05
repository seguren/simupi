package config

import "fmt"

// SensorConfig defines a single sensor's simulation parameters.
type SensorConfig struct {
	Mode     string  `json:"mode"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Interval int     `json:"interval"`
	Period   float64 `json:"period,omitempty"`
	// Step controls the max change per tick in "walk" mode. Defaults to (max-min)/20.
	Step     float64 `json:"step,omitempty"`
	// Duty is the high fraction (0–1) for "square" mode. Defaults to 0.5.
	Duty     float64 `json:"duty,omitempty"`
	Unit     string  `json:"unit,omitempty"`
	Pin      *int    `json:"pin,omitempty"`
}

// Scenario holds the full definition of a simulation scenario.
type Scenario struct {
	Name    string                  `json:"name"`
	Inputs  []int                   `json:"inputs"`
	Outputs []int                   `json:"outputs"`
	Aliases map[string]string       `json:"aliases"`
	Sensors map[string]SensorConfig `json:"sensors"`
}

// Validate checks the scenario for basic correctness.
func (s *Scenario) Validate() error {
	if len(s.Sensors) == 0 {
		return fmt.Errorf("scenario must have at least one sensor")
	}
	validModes := map[string]bool{
		"random": true, "wave": true, "ramp": true, "walk": true,
		"square": true, "triangle": true, "sawtooth": true, "slider": true,
	}
	for name, cfg := range s.Sensors {
		if !validModes[cfg.Mode] {
			return fmt.Errorf("sensor %q: invalid mode %q", name, cfg.Mode)
		}
		if cfg.Min >= cfg.Max {
			return fmt.Errorf("sensor %q: min (%.2f) must be less than max (%.2f)", name, cfg.Min, cfg.Max)
		}
	}
	return nil
}

// FailureMode holds the failure simulation state for a sensor.
type FailureMode struct {
	Mode string `json:"mode"` // stuck | offline
}

// AppState holds the runtime state persisted between restarts.
type AppState struct {
	GPIOInputs  map[string]int         `json:"gpio_inputs"`
	GPIOOutputs map[string]int         `json:"gpio_outputs"`
	Sensors     map[string]float64     `json:"sensors"`
	Scenario    string                 `json:"scenario"`
	Failures    map[string]FailureMode `json:"failures"`
	Overrides   map[string]float64     `json:"overrides,omitempty"`
}
