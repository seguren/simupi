package api

import (
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"simupi/internal/config"
	"simupi/internal/gpio"
	"simupi/internal/history"
	"simupi/internal/mqtt"
	"simupi/internal/sensor"
)

var safeName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)

// Server holds all runtime components and serves HTTP requests.
type Server struct {
	embedFS    embed.FS
	scenarios  *config.ScenarioManager
	mqttClient *mqtt.Client
	hist       *history.HistoryManager

	mu       sync.RWMutex
	gpio     *gpio.GPIOManager
	engine   *sensor.SensorEngine
	scenario string
	aliases  map[string]string
}

func NewServer(
	embedFS embed.FS,
	scenarios *config.ScenarioManager,
	mqttClient *mqtt.Client,
	hist *history.HistoryManager,
) *Server {
	return &Server{
		embedFS:    embedFS,
		scenarios:  scenarios,
		mqttClient: mqttClient,
		hist:       hist,
		aliases:    map[string]string{},
	}
}

// Init loads the default scenario, restores persisted state, and wires up GPIO.
// Must be called before Start.
func (s *Server) Init(defaultScenario string) error {
	sc, err := s.scenarios.Load(defaultScenario)
	if err != nil {
		return fmt.Errorf("load default scenario: %w", err)
	}

	s.gpio = gpio.NewGPIOManager(sc.Inputs, sc.Outputs)
	s.engine = sensor.NewSensorEngine(sc.Sensors, s.mqttClient, s.hist)
	s.scenario = sc.Name
	s.aliases = sc.Aliases

	if state, err := config.LoadState(); err != nil {
		log.Printf("warning: state restore: %v", err)
	} else if state != nil {
		s.gpio.Restore(state.GPIOInputs, state.GPIOOutputs)
		s.engine.Restore(state.Sensors)
	}

	s.mqttClient.SetGPIO(s.gpio)
	return nil
}

// Start launches the sensor engine and the autosave goroutine.
func (s *Server) Start() {
	s.engine.Start()
	go s.autosave()
}

func (s *Server) autosave() {
	for {
		time.Sleep(5 * time.Second)

		s.mu.RLock()
		g, e, sc := s.gpio, s.engine, s.scenario
		s.mu.RUnlock()

		inputs, outputs := g.GetState()
		state := &config.AppState{
			GPIOInputs:  inputs,
			GPIOOutputs: outputs,
			Sensors:     e.GetValues(),
			Scenario:    sc,
			Failures:    e.GetFailures(),
		}
		if err := config.SaveState(state); err != nil {
			log.Printf("autosave: %v", err)
		}
	}
}

// RegisterRoutes wires all HTTP endpoints onto mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	staticFS, _ := fs.Sub(s.embedFS, "web/static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("GET /{$}", s.handleIndex)

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/state", s.handleState)

	mux.HandleFunc("GET /api/scenarios", s.handleListScenarios)
	mux.HandleFunc("GET /api/scenarios/{name}", s.handleGetScenario)
	mux.HandleFunc("POST /api/scenarios/load/{name}", s.handleLoadScenario)

	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("POST /api/config/apply", s.handleApplyConfig)
	mux.HandleFunc("POST /api/config/save/{name}", s.handleSaveConfig)

	mux.HandleFunc("POST /api/gpio/{pin}", s.handleGPIOSet)
	mux.HandleFunc("POST /api/gpio/{pin}/toggle", s.handleGPIOToggle)
	mux.HandleFunc("POST /api/gpio/{pin}/pulse", s.handleGPIOPulse)

	mux.HandleFunc("POST /api/sensor/{name}", s.handleSetSensor)
	mux.HandleFunc("POST /api/failure/{sensor}", s.handleFailure)

	mux.HandleFunc("GET /api/history", s.handleAllHistory)
	mux.HandleFunc("GET /api/history/{sensor}", s.handleSensorHistory)
	mux.HandleFunc("GET /api/export/{sensor}", s.handleExportCSV)
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	data, err := s.embedFS.ReadFile("web/templates/index.html")
	if err != nil {
		http.Error(w, "not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleState(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	g, e, sc, aliases := s.gpio, s.engine, s.scenario, s.aliases
	s.mu.RUnlock()

	inputs, outputs := g.GetState()
	values := e.GetValues()
	failures := e.GetFailures()
	cfg := e.Config()

	sensorUnits := make(map[string]string, len(cfg))
	sensorModes := make(map[string]string, len(cfg))
	sensorPins := make(map[string]int)
	for name, c := range cfg {
		sensorUnits[name] = c.Unit
		sensorModes[name] = c.Mode
		if c.Pin != nil {
			sensorPins[name] = *c.Pin
		}
	}

	rounded := make(map[string]float64, len(values))
	for k, v := range values {
		rounded[k] = math.Round(v*100) / 100
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"gpio_inputs":  inputs,
		"gpio_outputs": outputs,
		"sensors":      rounded,
		"sensor_units": sensorUnits,
		"sensor_modes": sensorModes,
		"sensor_pins":  sensorPins,
		"aliases":      aliases,
		"scenario":     sc,
		"failures":     failures,
	})
}

func (s *Server) handleListScenarios(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.scenarios.List())
}

func (s *Server) handleGetScenario(w http.ResponseWriter, r *http.Request) {
	sc, err := s.scenarios.Load(r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (s *Server) handleLoadScenario(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	sc, err := s.scenarios.Load(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.reloadScenario(sc)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "scenario": name})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	g, e, aliases := s.gpio, s.engine, s.aliases
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"inputs":  g.InputPins(),
		"outputs": g.OutputPins(),
		"sensors": e.Config(),
		"aliases": aliases,
	})
}

func (s *Server) handleApplyConfig(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	s.mu.RLock()
	currentName := s.scenario
	s.mu.RUnlock()

	sc, err := buildScenario(payload, currentName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.reloadScenario(sc)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !safeName.MatchString(name) {
		writeError(w, http.StatusBadRequest, "invalid name (letters, numbers, - and _ only)")
		return
	}
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	sc, err := buildScenario(payload, name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.scenarios.Save(name, sc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.reloadScenario(sc)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "scenario": name})
}

func (s *Server) handleGPIOSet(w http.ResponseWriter, r *http.Request) {
	pin, err := strconv.Atoi(r.PathValue("pin"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid pin")
		return
	}
	s.mu.RLock()
	g := s.gpio
	s.mu.RUnlock()

	if !g.InputExists(pin) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("pin %d not configured as input", pin))
		return
	}
	var body struct {
		Value int `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "missing 'value'")
		return
	}
	g.SetInput(pin, body.Value)
	s.mqttClient.PublishInput(pin, body.Value)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleGPIOToggle(w http.ResponseWriter, r *http.Request) {
	pin, err := strconv.Atoi(r.PathValue("pin"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid pin")
		return
	}
	s.mu.RLock()
	g := s.gpio
	s.mu.RUnlock()

	if !g.InputExists(pin) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("pin %d not configured as input", pin))
		return
	}
	value := g.ToggleInput(pin)
	s.mqttClient.PublishInput(pin, value)
	writeJSON(w, http.StatusOK, map[string]any{"value": value})
}

func (s *Server) handleGPIOPulse(w http.ResponseWriter, r *http.Request) {
	pin, err := strconv.Atoi(r.PathValue("pin"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid pin")
		return
	}
	s.mu.RLock()
	g := s.gpio
	s.mu.RUnlock()

	if !g.InputExists(pin) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("pin %d not configured as input", pin))
		return
	}
	var body struct {
		Duration float64 `json:"duration"`
	}
	body.Duration = 0.5
	json.NewDecoder(r.Body).Decode(&body)

	mc := s.mqttClient
	g.PulseInput(pin, time.Duration(body.Duration*float64(time.Second)), func() {
		mc.PublishInput(pin, 0)
	})
	mc.PublishInput(pin, 1)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleSetSensor(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	s.mu.RLock()
	e := s.engine
	s.mu.RUnlock()

	if _, ok := e.Config()[name]; !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("sensor %q not configured", name))
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	fv, err := toFloat(body["value"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "value must be a number")
		return
	}
	e.SetValue(name, fv)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleFailure(w http.ResponseWriter, r *http.Request) {
	sensorName := r.PathValue("sensor")
	s.mu.RLock()
	e := s.engine
	s.mu.RUnlock()

	if _, ok := e.Config()[sensorName]; !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("sensor %q not configured", sensorName))
		return
	}
	var body struct {
		Mode string `json:"mode"`
	}
	body.Mode = "stuck"
	json.NewDecoder(r.Body).Decode(&body)

	if body.Mode == "clear" {
		e.ClearFailure(sensorName)
	} else {
		e.SetFailure(sensorName, body.Mode)
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "sensor": sensorName, "mode": body.Mode})
}

func (s *Server) handleAllHistory(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.hist.GetAll())
}

func (s *Server) handleSensorHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.hist.Get(r.PathValue("sensor")))
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	sensorName := r.PathValue("sensor")
	rows := s.hist.Get(sensorName)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, sensorName))
	wr := csv.NewWriter(w)
	wr.Write([]string{"timestamp", "value"})
	for _, row := range rows {
		wr.Write([]string{row.Timestamp, strconv.FormatFloat(row.Value, 'f', 2, 64)})
	}
	wr.Flush()
}

// ── Internal ──────────────────────────────────────────────────────────────────

func (s *Server) reloadScenario(sc *config.Scenario) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.engine.Stop()

	s.gpio = gpio.NewGPIOManager(sc.Inputs, sc.Outputs)
	s.mqttClient.SetGPIO(s.gpio)
	s.hist.Clear()

	s.engine = sensor.NewSensorEngine(sc.Sensors, s.mqttClient, s.hist)
	s.engine.Start()

	s.scenario = sc.Name
	if sc.Aliases != nil {
		s.aliases = sc.Aliases
	} else {
		s.aliases = map[string]string{}
	}
}

// buildScenario validates a raw JSON payload and returns a Scenario.
func buildScenario(payload map[string]any, name string) (*config.Scenario, error) {
	rawSensors, ok := payload["sensors"].(map[string]any)
	if !ok || len(rawSensors) == 0 {
		return nil, fmt.Errorf("missing 'sensors'")
	}

	sensors := make(map[string]config.SensorConfig, len(rawSensors))
	for sname, raw := range rawSensors {
		data, _ := json.Marshal(raw)
		var cfg config.SensorConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("sensor %q: %w", sname, err)
		}
		validModes := map[string]bool{
			"random": true, "wave": true, "ramp": true, "walk": true,
			"square": true, "triangle": true, "sawtooth": true,
		}
		if !validModes[cfg.Mode] {
			return nil, fmt.Errorf("sensor %q: invalid mode %q", sname, cfg.Mode)
		}
		if cfg.Min >= cfg.Max {
			return nil, fmt.Errorf("sensor %q: min must be less than max", sname)
		}
		sensors[sname] = cfg
	}

	inputs, err := parseIntList(payload["inputs"])
	if err != nil {
		return nil, fmt.Errorf("inputs: %w", err)
	}
	outputs, err := parseIntList(payload["outputs"])
	if err != nil {
		return nil, fmt.Errorf("outputs: %w", err)
	}

	aliases := map[string]string{}
	if raw, ok := payload["aliases"].(map[string]any); ok {
		for k, v := range raw {
			if str, ok := v.(string); ok {
				aliases[k] = str
			}
		}
	}

	return &config.Scenario{
		Name:    name,
		Inputs:  inputs,
		Outputs: outputs,
		Aliases: aliases,
		Sensors: sensors,
	}, nil
}

func parseIntList(raw any) ([]int, error) {
	if raw == nil {
		return []int{}, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("must be an array")
	}
	result := make([]int, 0, len(items))
	for _, item := range items {
		f, ok := item.(float64)
		if !ok {
			return nil, fmt.Errorf("elements must be integers")
		}
		result = append(result, int(f))
	}
	return result, nil
}

func toFloat(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
