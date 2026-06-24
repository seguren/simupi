# SimuPi Go

Simulador de Raspberry Pi para IoT escrito en Go. Genera datos de sensores, simula pines GPIO y expone un broker MQTT embebido — todo en un único binario sin dependencias externas ni Docker.

Es el port en Go del proyecto original [simupi](../simupi) (Python/Flask/Docker).

---

## Características

- **Broker MQTT embebido** (mochi-mqtt): no requiere Mosquitto ni ningún broker externo
- **Sensores simulados** con siete modos: `wave`, `square`, `triangle`, `sawtooth`, `ramp`, `walk` y `random`
- **GPIO inputs y outputs** simulados, controlables desde la UI o vía MQTT
- **Simulación de fallos** por sensor: `stuck` (valor congelado) u `offline` (deja de publicar)
- **UI web** incluida en el binario con `//go:embed`
- **Persistencia de estado** entre reinicios (escritura atómica con rename)
- **Escenarios intercambiables** en caliente sin reiniciar el proceso
- **Cross-compilación**: un solo `go build` produce el ejecutable para Linux, Windows o macOS

---

## Requisitos

- Go 1.24+

```bash
# Verificar versión
go version
```

---

## Compilar y ejecutar

```bash
# Clonar / entrar al directorio
cd simupi-go

# Compilar
go build -o simupi .

# Ejecutar
./simupi
```

El servidor arranca en `http://localhost:5000` y el broker MQTT en `localhost:1883`.

### Variables de entorno

| Variable | Default | Descripción |
|----------|---------|-------------|
| `PORT`   | `5000`  | Puerto HTTP |

---

## Makefile

```bash
make build    # binario local ./simupi (sistema actual)
make linux    # dist/simupi-linux-amd64
make windows  # dist/simupi-windows-amd64.exe
make darwin   # dist/simupi-darwin-arm64 + dist/simupi-darwin-amd64
make all      # los cuatro targets anteriores
make run      # compila y ejecuta localmente
make tidy     # go mod tidy
make clean    # elimina ./simupi y dist/
```

---

## Cross-compilación manual

```bash
# Linux (amd64)
GOOS=linux  GOARCH=amd64 go build -o simupi-linux .

# Windows
GOOS=windows GOARCH=amd64 go build -o simupi.exe .

# macOS (Apple Silicon)
GOOS=darwin  GOARCH=arm64 go build -o simupi-darwin .
```

El binario resultante incluye la UI, los escenarios por defecto y el broker MQTT. No necesita nada instalado en el sistema destino.

---

## Estructura del proyecto

```
simupi-go/
├── main.go                   # Punto de entrada: wireup de todos los componentes
├── go.mod / go.sum
├── web/
│   ├── static/
│   │   ├── app.js            # Frontend (embebido en el binario)
│   │   └── style.css
│   └── templates/
│       └── index.html
├── scenarios/                # Escenarios por defecto (embebidos, read-only)
│   ├── garage.json
│   ├── greenhouse.json
│   └── smart_home.json
├── data/                     # Generado en runtime (no versionado)
│   ├── state.json            # Estado persistido entre reinicios
│   └── scenarios/            # Escenarios guardados por el usuario
└── internal/
    ├── config/
    │   ├── types.go          # Structs: Scenario, SensorConfig, AppState, FailureMode
    │   ├── scenario.go       # ScenarioManager: List / Load / Save
    │   └── state.go          # SaveState / LoadState (escritura atómica)
    ├── gpio/
    │   └── gpio.go           # GPIOManager (sync.RWMutex, Toggle, Pulse)
    ├── sensor/
    │   └── engine.go         # SensorEngine: goroutine por sensor, modos, failures
    ├── history/
    │   └── history.go        # Buffer circular (500 entradas por sensor)
    ├── mqtt/
    │   ├── broker.go         # Broker embebido (mochi-mqtt)
    │   └── client.go         # Cliente paho: publica sensores y GPIO, suscribe outputs
    └── api/
        └── server.go         # Todos los handlers HTTP + lógica de reload
```

---

## Escenarios

Los escenarios definen qué sensores y pines GPIO simular. Se pueden cargar desde la UI o vía API.

**Escenarios embebidos** (read-only, incluidos en el binario):

| Nombre | Sensores | GPIO inputs | GPIO outputs |
|--------|----------|-------------|--------------|
| `garage` | distance (ramp) | 17, 18, 23 | 22, 24 |
| `greenhouse` | temperature (wave), humidity (walk) | 17, 18 | 22, 24 |
| `smart_home` | temperature (wave), power (random) | 17, 18, 23 | 22, 24, 25 |

**Escenarios de usuario** se guardan en `data/scenarios/` y tienen prioridad sobre los embebidos si tienen el mismo nombre.

### Formato de escenario

```json
{
  "name": "Mi escenario",
  "inputs":  [17, 18],
  "outputs": [22, 24],
  "aliases": {
    "22": "ventilador",
    "24": "bomba"
  },
  "sensors": {
    "temperatura": {
      "mode":     "wave",
      "min":      15,
      "max":      40,
      "interval": 1,
      "period":   60,
      "unit":     "°C",
      "pin":      4
    },
    "humedad": {
      "mode":     "walk",
      "min":      30,
      "max":      90,
      "interval": 1,
      "step":     2,
      "unit":     "%"
    }
  }
}
```

### Modos de sensor

El simulador funciona como un generador de señales configurable. Cada sensor puede usar un modo distinto de forma independiente.

#### Modos periódicos (generador de señales)

Todos usan el campo `period` (duración de un ciclo completo en segundos, default `60`).

| Modo | Forma de onda | Parámetros extra |
|------|--------------|-----------------|
| `wave` | **Senoidal** — oscila suavemente entre `min` y `max` | `period` |
| `square` | **Cuadrada** — alterna entre `max` y `min` con ciclo de trabajo configurable | `period`, `duty` |
| `triangle` | **Triangular** — sube y baja linealmente de forma simétrica | `period` |
| `sawtooth` | **Diente de sierra** — sube linealmente de `min` a `max` y reinicia | `period` |

El campo `duty` (solo para `square`) es la fracción del período en estado alto, entre `0.0` y `1.0`. Por defecto es `0.5` (señal simétrica).

```json
// Onda cuadrada: 10s en MAX, 2s en MIN (duty = 10/12 ≈ 0.83)
"presencia": { "mode": "square", "min": 0, "max": 1, "period": 12, "duty": 0.83, "interval": 1 }

// Triangular con período de 30 segundos
"nivel": { "mode": "triangle", "min": 0, "max": 100, "period": 30, "interval": 1, "unit": "%" }

// Diente de sierra: simula llenado continuo de un tanque
"caudal": { "mode": "sawtooth", "min": 0, "max": 50, "period": 20, "interval": 1, "unit": "L/min" }
```

#### Modos estocásticos

| Modo | Descripción | Parámetros extra |
|------|-------------|-----------------|
| `walk` | **Caminata aleatoria** — cada tick el valor cambia ±`step` desde el valor anterior. Variación suave y continua | `step` (default: `(max-min)/20`) |
| `ramp` | **Rampa escalonada** — sube y baja en pasos fijos de `(max-min)/20` | — |
| `random` | **Aleatorio uniforme** — salta sin continuidad entre `min` y `max` en cada tick | — |

El campo `step` (solo para `walk`) define cuánto puede cambiar el valor por tick como máximo. Un `step` pequeño produce variaciones muy suaves; uno grande produce cambios más bruscos pero siempre continuos.

```json
// Humedad que varía suavemente ±2% por segundo
"humedad": { "mode": "walk", "min": 30, "max": 90, "step": 2, "interval": 1, "unit": "%" }
```

#### Escenario con múltiples formas de onda

```json
{
  "name": "Laboratorio",
  "inputs": [17, 18],
  "outputs": [22, 24],
  "aliases": { "22": "alarma", "24": "ventilador" },
  "sensors": {
    "temperatura":  { "mode": "wave",     "min": 18, "max": 30, "period": 60,  "interval": 1, "unit": "°C" },
    "humedad":      { "mode": "walk",     "min": 40, "max": 80, "step": 1.5,   "interval": 1, "unit": "%" },
    "presencia":    { "mode": "square",   "min": 0,  "max": 1,  "period": 10,  "duty": 0.3,   "interval": 1 },
    "nivel_tanque": { "mode": "sawtooth", "min": 0,  "max": 100,"period": 120, "interval": 1, "unit": "%" },
    "vibracion":    { "mode": "triangle", "min": 0,  "max": 5,  "period": 30,  "interval": 1, "unit": "g" },
    "ruido":        { "mode": "random",   "min": 30, "max": 90, "interval": 2, "unit": "dB" }
  }
}
```

### Campo `pin` (opcional)

El campo `pin` en un sensor es **solo metadato** — indica a qué pin físico de la Raspberry Pi se conectaría el sensor real (I2C, 1-Wire, SPI). No afecta la simulación.

---

## MQTT

El broker corre embebido en el proceso en el puerto `1883`. Se puede conectar cualquier cliente MQTT externo (Node-RED, MQTT Explorer, etc.).

### Tópicos publicados por el simulador

| Tópico | Contenido | Ejemplo |
|--------|-----------|---------|
| `sensor/<nombre>` | Valor del sensor como float con 2 decimales | `sensor/temperature` → `24.53` |
| `gpio/in/<pin>` | Estado del pin de entrada (0 o 1) | `gpio/in/17` → `1` |

### Tópicos suscritos por el simulador

| Tópico | Acción | Ejemplo payload |
|--------|--------|-----------------|
| `gpio/out/<pin>` | Setea el pin de salida al valor recibido | `0` o `1` |

---

## API REST

Base URL: `http://localhost:5000`

### Estado

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| `GET` | `/api/health` | Estado del servidor |
| `GET` | `/api/state` | Estado completo en tiempo real |

### Escenarios

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| `GET` | `/api/scenarios` | Lista de escenarios disponibles |
| `GET` | `/api/scenarios/{name}` | Definición de un escenario |
| `POST` | `/api/scenarios/load/{name}` | Carga un escenario en caliente |

### Configuración (editor en la UI)

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| `GET` | `/api/config` | Configuración activa (sensores, pines, aliases) |
| `POST` | `/api/config/apply` | Aplica nueva config sin guardar |
| `POST` | `/api/config/save/{name}` | Guarda config como nuevo escenario y lo aplica |

Body para `apply` y `save`:
```json
{
  "inputs":  [17, 18],
  "outputs": [22, 24],
  "aliases": { "22": "ventilador" },
  "sensors": {
    "temperatura": { "mode": "wave", "min": 15, "max": 40, "interval": 1, "period": 60, "unit": "°C" }
  }
}
```

### GPIO

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| `POST` | `/api/gpio/{pin}` | Setea un input a un valor específico |
| `POST` | `/api/gpio/{pin}/toggle` | Invierte el valor de un input |
| `POST` | `/api/gpio/{pin}/pulse` | Pulso HIGH → LOW en un input |

```json
// POST /api/gpio/{pin}
{ "value": 1 }

// POST /api/gpio/{pin}/pulse  (duration en segundos, opcional)
{ "duration": 0.5 }
```

### Sensores

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| `POST` | `/api/sensor/{name}` | Fija el valor de un sensor manualmente |

```json
{ "value": 25.0 }
```

### Fallos

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| `POST` | `/api/failure/{sensor}` | Activa o limpia un modo de fallo |

```json
{ "mode": "stuck" }   // valor congelado
{ "mode": "offline" } // deja de publicar
{ "mode": "clear" }   // vuelve a la simulación normal
```

### Historial

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| `GET` | `/api/history` | Historial de todos los sensores |
| `GET` | `/api/history/{sensor}` | Historial de un sensor (hasta 500 entradas) |
| `GET` | `/api/export/{sensor}` | Descarga el historial como CSV |

---

## Persistencia

Al iniciar, el simulador intenta cargar `data/state.json` para restaurar el último estado conocido (valores de sensores, estado de pines, fallos activos). El estado se guarda automáticamente cada 5 segundos mediante escritura atómica (archivo temporal + rename POSIX).

Los escenarios guardados por el usuario se almacenan en `data/scenarios/` y persisten entre reinicios.

---

## Diferencias con la versión Python

| Aspecto | Python (simupi) | Go (simupi-go) |
|---------|-----------------|----------------|
| Runtime | Python 3.11 + Gunicorn | Binario estático |
| Broker MQTT | Mosquitto (Docker) | Embebido (mochi-mqtt) |
| Dependencias externas | Docker Compose | Ninguna |
| Concurrencia | Threads + GIL | Goroutines nativas |
| Distribución | Imagen Docker | Ejecutable único |

---

## Dependencias

| Paquete | Versión | Uso |
|---------|---------|-----|
| `github.com/mochi-mqtt/server/v2` | v2.7.9 | Broker MQTT embebido |
| `github.com/eclipse/paho.mqtt.golang` | v1.5.1 | Cliente MQTT |
| `golang.org/x/net` | v0.44.0 | Dependencia transitiva de paho |
| `github.com/gorilla/websocket` | v1.5.3 | Dependencia transitiva de paho |
