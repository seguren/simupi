# SimuPi — CLAUDE.md

Simulador de Raspberry Pi escrito en Go. Genera datos realistas de sensores y simula pines GPIO, expone un broker MQTT embebido y una API REST. Usado como paso previo al hardware real (Raspberry Pi 3 + kit de sensores) en la materia Cloud Computing & Cloud Robotics.

## Comandos esenciales

```bash
make run          # compila y ejecuta (Go detectado automáticamente)
make build        # solo compila → ./simupi
make linux        # cross-compile → dist/simupi-linux-amd64
make windows      # cross-compile → dist/simupi-windows-amd64.exe
make darwin       # cross-compile → dist/simupi-darwin-arm64 y -amd64
make tidy         # go mod tidy
make clean        # elimina binarios

go build -o simupi .          # alternativa directa
PORT=8080 ./simupi            # cambiar puerto HTTP (default: 5000)
```

El Go toolchain está en `~/go/bin/go` (v1.27.1) y el GOPATH en `~/gopath`. El Makefile los detecta automáticamente.

## Arquitectura

Binario único, sin dependencias externas en runtime:

```
main.go
  ├─ mqtt.Broker        → broker MQTT embebido (mochi-mqtt) en :1883
  ├─ config.ScenarioManager → carga escenarios embedded + data/scenarios/
  ├─ history.HistoryManager → buffer circular 500 entradas por sensor
  ├─ mqtt.Client        → publica sensores/GPIO, suscribe gpio/out/<pin>
  └─ api.Server
       ├─ sensor.SensorEngine  → goroutine por sensor, 8 modos de señal
       ├─ gpio.GPIOManager     → estado concurrente de pines in/out
       └─ HTTP handlers        → REST API + sirve web UI embebida
```

**Puertos fijos:** HTTP `:5000` (configurable con `PORT`), MQTT `:1883` (hardcoded).

**Estado persistido** automáticamente cada 5 segundos en `data/state.json` (escritura atómica vía `.tmp` + rename). Se restaura al reiniciar.

## Estructura de paquetes

| Paquete | Archivo | Responsabilidad |
|---------|---------|-----------------|
| `main` | `main.go` | Wiring, embedding, entry point |
| `config` | `types.go` | Structs: `SensorConfig`, `Scenario`, `FailureMode`, `AppState` |
| `config` | `scenario.go` | `ScenarioManager`: List/Load/Save. User scenarios en `data/scenarios/` sobreescriben los embedded de `scenarios/` |
| `config` | `state.go` | `SaveState`/`LoadState` con escritura atómica |
| `sensor` | `engine.go` | `SensorEngine`: una goroutine por sensor, 8 modos, mapa de overrides, mapa de failures |
| `gpio` | `gpio.go` | `GPIOManager`: estado concurrente con RWMutex, toggle, pulse |
| `mqtt` | `broker.go` | Broker embebido mochi-mqtt |
| `mqtt` | `client.go` | Publica `sensor/<name>` y `gpio/in/<pin>`; suscribe `gpio/out/<pin>` |
| `history` | `history.go` | Buffer circular 500 entradas por sensor |
| `api` | `server.go` | HTTP handlers, recarga de escenarios, `buildScenario` con validación |

## Modos de simulación de sensores

| Modo | Algoritmo | Parámetros clave |
|------|-----------|-----------------|
| `wave` | Senoide entre min/max | `period` (s) |
| `triangle` | Rampa lineal periódica | `period` (s) |
| `sawtooth` | Rampa que vuelve a 0 abruptamente | `period` (s) |
| `square` | Alterna min/max | `period`, `duty` (0–1) |
| `ramp` | Escalones discretos (max-min)/20 | — |
| `walk` | Random walk ±step | `step` (default: (max-min)/20) |
| `random` | Uniforme sin continuidad | — |
| `slider` | Controlado 100% por el usuario vía UI | — (inicializa en midpoint) |

**Importante:** los modos time-based (`wave`, `triangle`, `sawtooth`, `square`) no usan `e.values[name]` para calcular — usan `time.Since(startTime)`. Por eso el override se implementa como mapa separado (`e.overrides`) que bypasea `computeLocked` completamente.

## Tópicos MQTT

| Dirección | Tópico | Payload |
|-----------|--------|---------|
| SimuPi → clientes | `sensor/<nombre>` | float string con 2 decimales, ej. `"24.53"` |
| SimuPi → clientes | `gpio/in/<pin>` | `"0"` o `"1"` |
| Clientes → SimuPi | `gpio/out/<pin>` | `"0"` o `"1"` |

## API REST

Base: `http://localhost:5000`

```
GET  /api/health
GET  /api/state                        # incluye sensor_limits, overrides
GET  /api/scenarios
GET  /api/scenarios/{name}
POST /api/scenarios/load/{name}

GET  /api/config
POST /api/config/apply
POST /api/config/save/{name}           # regex: ^[a-zA-Z0-9_-]{1,50}$

POST /api/gpio/{pin}                   # body: {"value": 0|1}
POST /api/gpio/{pin}/toggle
POST /api/gpio/{pin}/pulse             # body: {"duration": 0.5}

POST   /api/sensor/{name}             # body: {"value": float}
                                      #   slider mode → SetValue
                                      #   otros modos → SetOverride (bypasa algoritmo)
DELETE /api/sensor/{name}             # limpia override, vuelve al algoritmo

POST /api/failure/{sensor}            # body: {"mode": "stuck"|"offline"|"clear"}

GET  /api/history
GET  /api/history/{sensor}
GET  /api/export/{sensor}             # descarga CSV
```

## Web UI

Servida desde `web/` embebido en el binario. Acceso en `http://localhost:5000`.

- **Tabs:** Histórico (Chart.js), Topics MQTT (con copy), Estado del sistema (JSON dump)
- **Sensores:** badge de modo, valor actual, override manual, badge "fijado" (naranja) cuando override activo, botón Liberar para limpiar override
- **Slider mode:** renderiza `<input type="range">` — display se actualiza en `input`, API se llama en `change`
- **GPIO inputs:** Toggle; GPIO outputs: LED indicador (solo lectura, controlado por MQTT)
- **Config modal:** editor JSON, Apply (temporal) o Save (persiste en `data/scenarios/`)

## Escenarios embebidos

| Escenario | Sensores | GPIO outputs |
|-----------|----------|-------------|
| `garage` | `distance` (ramp, 10–400 cm) | 22→door, 24→light |
| `greenhouse` | `temperature` (wave, 15–40°C), `humidity` (walk, 30–90%) | 22→fan, 24→pump |
| `smart_home` | `temperature` (wave, 18–30°C), `power` (random, 100–2500 W) | 22→living_light, 24→heater, 25→alarm |

Los escenarios de usuario en `data/scenarios/` sobreescriben los embedded del mismo nombre.

## Validación de escenarios

`buildScenario` en `server.go` y `Validate()` en `types.go` comparten la misma lista de modos válidos — si agregás un modo nuevo, actualizá **ambos**.

Modos válidos actuales: `random`, `wave`, `ramp`, `walk`, `square`, `triangle`, `sawtooth`, `slider`.

Restricción: `min < max` obligatorio en todo sensor.

## Archivos generados en runtime (no commitear)

```
data/state.json          # estado persistido (GPIO, sensores, failures, overrides)
data/scenarios/*.json    # escenarios creados por el usuario
simupi                   # binario compilado
dist/                    # binarios cross-compilados
```

El `.gitignore` debería excluir `data/` y `dist/`.

## Documentación para alumnos

```
docs/tutorial_simupi_nodered.html   # tutorial completo + ejercicios prácticos (abrir en browser → Print → PDF)
```

## Contexto del proyecto

- **Materia:** Cloud Computing & Cloud Robotics — UNLP (seguren@lidi.info.unlp.edu.ar)
- **Propósito:** step previo al hardware real; los flujos Node-RED construidos contra SimuPi funcionan sin cambios en la Raspberry Pi 3 física
- **Toolchain Go:** 1.27.1 (en `~/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.1.linux-amd64/`)
