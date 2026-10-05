let chart = null;
let historyTick = 0;

// =====================================================
// CHART
// =====================================================

function initChart() {
    const ctx = document.getElementById("sensorChart");
    chart = new Chart(ctx, {
        type: "line",
        data: {
            labels: [],
            datasets: [{
                label: "Sensor",
                data: [],
                borderColor: "#1565c0",
                backgroundColor: "rgba(21,101,192,0.07)",
                borderWidth: 2,
                tension: 0.35,
                pointRadius: 0,
                pointHoverRadius: 4,
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: false,
            interaction: { mode: "index", intersect: false },
            plugins: {
                legend: { display: false },
                tooltip: { backgroundColor: "#1565c0", padding: 8, cornerRadius: 6 }
            },
            scales: {
                x: { ticks: { maxTicksLimit: 8, color: "#9e9e9e", font: { size: 11 } }, grid: { color: "#f0f0f0" } },
                y: { ticks: { color: "#9e9e9e", font: { size: 11 } }, grid: { color: "#f0f0f0" } }
            }
        }
    });
}

async function loadHistory() {
    const sensor = document.getElementById("chart-sensor").value;
    if (!sensor) return;
    const res = await fetch(`/api/history/${sensor}`);
    const history = await res.json();
    chart.data.labels = history.map(x => x.timestamp.slice(11, 19));
    chart.data.datasets[0].data = history.map(x => x.value);
    chart.data.datasets[0].label = sensor;
    chart.update();
}

// =====================================================
// STATE
// =====================================================

async function loadState() {
    const res = await fetch("/api/state");
    const state = await res.json();
    document.getElementById("scenario-name").textContent = state.scenario || "—";
    renderInputs(state);
    renderOutputs(state);
    renderSensors(state);
    renderMqttTopics(state);
    document.getElementById("status").textContent = JSON.stringify(state, null, 2);
}

// =====================================================
// GPIO INPUTS
// =====================================================

function renderInputs(state) {
    const container = document.getElementById("gpio-inputs");
    container.innerHTML = "";
    const aliases = state.aliases || {};

    if (Object.keys(state.gpio_inputs).length === 0) {
        container.innerHTML = "<p class='empty-msg'>Sin pines configurados</p>";
        return;
    }

    Object.entries(state.gpio_inputs).forEach(([pin, value]) => {
        const row = document.createElement("div");
        row.className = "gpio-input-row";

        const pinLabel = document.createElement("span");
        pinLabel.className = "gpio-pin-label";
        pinLabel.textContent = `GPIO ${pin}`;

        const aliasLabel = document.createElement("span");
        aliasLabel.className = "gpio-alias";
        aliasLabel.textContent = aliases[pin] || "";

        const badge = document.createElement("span");
        badge.className = "gpio-value-badge";
        badge.textContent = value;

        const btn = document.createElement("button");
        btn.className = "btn-toggle";
        btn.textContent = "Toggle";
        btn.addEventListener("click", () => toggleGPIO(Number(pin)));

        row.appendChild(pinLabel);
        row.appendChild(aliasLabel);
        row.appendChild(badge);
        row.appendChild(btn);
        container.appendChild(row);
    });
}

// =====================================================
// GPIO OUTPUTS
// =====================================================

function renderOutputs(state) {
    const container = document.getElementById("gpio-outputs");
    container.innerHTML = "";
    const aliases = state.aliases || {};

    if (Object.keys(state.gpio_outputs).length === 0) {
        container.innerHTML = "<p class='empty-msg'>Sin pines configurados</p>";
        return;
    }

    Object.entries(state.gpio_outputs).forEach(([pin, value]) => {
        const alias = aliases[pin] || `GPIO ${pin}`;
        const isOn = Boolean(value);

        const row = document.createElement("div");
        row.className = "gpio-output-row" + (isOn ? " is-on" : "");

        const led = document.createElement("span");
        led.className = "led" + (isOn ? " on" : "");

        const aliasLabel = document.createElement("span");
        aliasLabel.className = "gpio-output-alias " + (isOn ? "is-on" : "is-off");
        aliasLabel.textContent = alias;

        const pinLabel = document.createElement("span");
        pinLabel.className = "gpio-output-pin";
        pinLabel.textContent = `GPIO ${pin}`;

        const val = document.createElement("span");
        val.className = "gpio-output-val";
        val.textContent = value;

        row.appendChild(led);
        row.appendChild(aliasLabel);
        row.appendChild(pinLabel);
        row.appendChild(val);
        container.appendChild(row);
    });
}

// =====================================================
// SENSORS
// =====================================================

function renderSensors(state) {
    const select = document.getElementById("chart-sensor");
    const currentSelection = select.value;

    if (select.options.length === 0) {
        Object.keys(state.sensors).forEach(sensor => {
            const opt = document.createElement("option");
            opt.value = sensor;
            opt.textContent = sensor;
            select.appendChild(opt);
        });
        loadHistory();
    } else if (currentSelection && state.sensors[currentSelection] !== undefined) {
        select.value = currentSelection;
    }

    const container = document.getElementById("sensors");
    container.innerHTML = "";
    const failures  = state.failures      || {};
    const units     = state.sensor_units  || {};
    const modes     = state.sensor_modes  || {};
    const pins      = state.sensor_pins   || {};
    const limits    = state.sensor_limits || {};
    const overrides = state.overrides     || {};

    Object.entries(state.sensors).forEach(([sensor, value]) => {
        const unit        = units[sensor] || "";
        const mode        = modes[sensor] || "";
        const pin         = pins[sensor];
        const failureMode = (failures[sensor] || {}).mode;
        const isSlider    = mode === "slider";
        const hasOverride = overrides[sensor] !== undefined;

        let cardClass = "sensor-card";
        if (failureMode)  cardClass += ` failure-${failureMode}`;
        if (hasOverride)  cardClass += " has-override";

        const card = document.createElement("div");
        card.className = cardClass;

        // ── Header ──────────────────────────────────────────
        const header = document.createElement("div");
        header.className = "sensor-card-header";

        const name = document.createElement("span");
        name.className = "sensor-name";
        name.textContent = sensor;

        const modeBadge = document.createElement("span");
        modeBadge.className = "mode-badge" + (isSlider ? " mode-badge-slider" : "");
        modeBadge.textContent = mode;

        header.appendChild(name);
        header.appendChild(modeBadge);

        if (pin !== undefined) {
            const pinBadge = document.createElement("span");
            pinBadge.className = "pin-badge";
            pinBadge.title = "Pin GPIO físico al que se conectaría el sensor";
            pinBadge.textContent = `GPIO ${pin}`;
            header.appendChild(pinBadge);
        }

        if (hasOverride) {
            const ob = document.createElement("span");
            ob.className = "override-badge";
            ob.textContent = "fijado";
            header.appendChild(ob);
        }

        if (failureMode) {
            const fb = document.createElement("span");
            fb.className = `failure-badge ${failureMode}`;
            fb.textContent = failureMode;
            header.appendChild(fb);
        }

        // ── Value row ────────────────────────────────────────
        const valueRow = document.createElement("div");
        valueRow.className = "sensor-value-row";

        const val = document.createElement("span");
        val.className = "sensor-value";
        val.textContent = value;

        const unitSpan = document.createElement("span");
        unitSpan.className = "sensor-unit";
        unitSpan.textContent = unit;

        valueRow.appendChild(val);
        valueRow.appendChild(unitSpan);

        card.appendChild(header);
        card.appendChild(valueRow);

        // ── Controls ─────────────────────────────────────────
        if (isSlider) {
            const lim = limits[sensor] || { min: 0, max: 100 };
            const step = (lim.max - lim.min) >= 100 ? 1 : 0.1;

            const sliderWrap = document.createElement("div");
            sliderWrap.className = "sensor-slider-wrap";

            const sliderInput = document.createElement("input");
            sliderInput.type  = "range";
            sliderInput.className = "sensor-slider";
            sliderInput.min   = lim.min;
            sliderInput.max   = lim.max;
            sliderInput.step  = step;
            sliderInput.value = value;

            // Update display live while dragging
            sliderInput.addEventListener("input", () => {
                val.textContent = parseFloat(sliderInput.value).toFixed(2);
            });
            // Send to API on release
            sliderInput.addEventListener("change", () => {
                setSensor(sensor, parseFloat(sliderInput.value));
            });

            sliderWrap.appendChild(sliderInput);

            const sliderLabels = document.createElement("div");
            sliderLabels.className = "sensor-slider-labels";
            sliderLabels.innerHTML =
                `<span>${lim.min} ${unit}</span><span>${lim.max} ${unit}</span>`;
            sliderWrap.appendChild(sliderLabels);

            card.appendChild(sliderWrap);
        } else {
            const overrideRow = document.createElement("div");
            overrideRow.className = "sensor-override";

            const input = document.createElement("input");
            input.id    = `sensor_${sensor}`;
            input.type  = "number";
            input.step  = "0.1";
            input.value = value;

            const setBtn = document.createElement("button");
            setBtn.className = "btn-set";
            setBtn.textContent = "Fijar";
            setBtn.addEventListener("click", () => setSensor(sensor));

            overrideRow.appendChild(input);
            overrideRow.appendChild(setBtn);

            if (hasOverride) {
                const clearBtn = document.createElement("button");
                clearBtn.className = "btn-release";
                clearBtn.textContent = "Liberar";
                clearBtn.addEventListener("click", () => clearSensorOverride(sensor));
                overrideRow.appendChild(clearBtn);
            }

            card.appendChild(overrideRow);
        }

        container.appendChild(card);
    });
}

// =====================================================
// MQTT TOPICS
// =====================================================

function renderMqttTopics(state) {
    const container = document.getElementById("mqtt-topics");
    if (!container) return;
    container.innerHTML = "";

    const aliases = state.aliases      || {};
    const units   = state.sensor_units || {};
    const pins    = state.sensor_pins  || {};

    // --- Publicados por el simulador ---
    addMqttSection(container, "Publicados por el simulador");

    Object.entries(state.sensors || {}).forEach(([name, value]) => {
        const unit    = units[name] ? ` · ${value} ${units[name]}` : ` · ${value}`;
        const pinInfo = pins[name] !== undefined ? ` · GPIO ${pins[name]}` : "";
        addMqttRow(container, `sensor/${name}`, `sensor${pinInfo}${unit}`, "pub");
    });

    Object.entries(state.gpio_inputs || {}).forEach(([pin, value]) => {
        const alias = aliases[pin] ? ` · ${aliases[pin]}` : "";
        addMqttRow(container, `gpio/in/${pin}`, `GPIO input${alias} · valor ${value}`, "pub");
    });

    // --- Recibidos por el simulador ---
    addMqttSection(container, "Recibidos por el simulador (Node-RED → outputs)");

    if (Object.keys(state.gpio_outputs || {}).length === 0) {
        const empty = document.createElement("p");
        empty.className = "empty-msg";
        empty.textContent = "Sin GPIO outputs configurados";
        container.appendChild(empty);
        return;
    }

    Object.entries(state.gpio_outputs || {}).forEach(([pin, value]) => {
        const alias = aliases[pin] ? ` · ${aliases[pin]}` : "";
        addMqttRow(container, `gpio/out/${pin}`, `GPIO output${alias} · publicar 0 o 1`, "sub");
    });
}

function addMqttSection(container, title) {
    const h = document.createElement("p");
    h.className = "mqtt-section-title";
    h.textContent = title;
    container.appendChild(h);
}

function addMqttRow(container, topic, description, type) {
    const row = document.createElement("div");
    row.className = `mqtt-topic-row mqtt-${type}`;

    const topicEl = document.createElement("span");
    topicEl.className = "mqtt-topic-name";
    topicEl.textContent = topic;

    const descEl = document.createElement("span");
    descEl.className = "mqtt-topic-desc";
    descEl.textContent = description;

    const copyBtn = document.createElement("button");
    copyBtn.className = "btn-copy";
    copyBtn.textContent = "Copiar";
    copyBtn.addEventListener("click", () => copyToClipboard(topic, copyBtn));

    row.appendChild(topicEl);
    row.appendChild(descEl);
    row.appendChild(copyBtn);
    container.appendChild(row);
}

async function copyToClipboard(text, btn) {
    try {
        await navigator.clipboard.writeText(text);
    } catch {
        // Fallback para contextos sin HTTPS
        const ta = document.createElement("textarea");
        ta.value = text;
        ta.style.cssText = "position:fixed;opacity:0";
        document.body.appendChild(ta);
        ta.select();
        document.execCommand("copy");
        document.body.removeChild(ta);
    }
    btn.textContent = "¡Copiado!";
    btn.classList.add("copied");
    setTimeout(() => { btn.textContent = "Copiar"; btn.classList.remove("copied"); }, 1500);
}

// =====================================================
// SCENARIOS
// =====================================================

async function loadScenarios() {
    const res = await fetch("/api/scenarios");
    const scenarios = await res.json();
    const select = document.getElementById("scenario-select");
    select.innerHTML = "";
    scenarios.forEach(s => {
        const opt = document.createElement("option");
        opt.value = s;
        opt.textContent = s;
        select.appendChild(opt);
    });
}

async function changeScenario() {
    const scenario = document.getElementById("scenario-select").value;
    await fetch(`/api/scenarios/load/${scenario}`, { method: "POST" });
    document.getElementById("chart-sensor").innerHTML = "";
    await loadState();
    await loadHistory();
}

// =====================================================
// GPIO / SENSOR ACTIONS
// =====================================================

async function toggleGPIO(pin) {
    await fetch(`/api/gpio/${pin}/toggle`, { method: "POST" });
    loadState();
}

async function setSensor(sensor, explicitValue) {
    const value = explicitValue !== undefined
        ? explicitValue
        : document.getElementById(`sensor_${sensor}`).value;
    await fetch(`/api/sensor/${sensor}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ value: parseFloat(value) })
    });
    loadState();
}

async function clearSensorOverride(sensor) {
    await fetch(`/api/sensor/${sensor}`, { method: "DELETE" });
    loadState();
}

// =====================================================
// TABS
// =====================================================

function initTabs() {
    document.querySelectorAll(".tab-btn").forEach(btn => {
        btn.addEventListener("click", () => {
            document.querySelectorAll(".tab-btn").forEach(b => b.classList.remove("active"));
            document.querySelectorAll(".tab-panel").forEach(p => p.classList.remove("active"));
            btn.classList.add("active");
            document.getElementById(`tab-${btn.dataset.tab}`).classList.add("active");
            if (btn.dataset.tab === "history") loadHistory();
        });
    });
}

// =====================================================
// CONFIG MODAL
// =====================================================

function openConfigModal() {
    loadConfig();
    const modal = document.getElementById("config-modal");
    modal.classList.add("open");
    modal.setAttribute("aria-hidden", "false");
    document.body.style.overflow = "hidden";
    setTimeout(() => document.getElementById("config-textarea").focus(), 250);
}

function closeConfigModal() {
    const modal = document.getElementById("config-modal");
    modal.classList.remove("open");
    modal.setAttribute("aria-hidden", "true");
    document.body.style.overflow = "";
}

async function loadConfig() {
    const res = await fetch("/api/config");
    const config = await res.json();
    document.getElementById("config-textarea").value = JSON.stringify(
        {
            inputs:  config.inputs  || [],
            outputs: config.outputs || [],
            sensors: config.sensors || {},
            aliases: config.aliases || {}
        },
        null,
        2
    );
}

function getConfigFromTextarea() {
    try {
        return JSON.parse(document.getElementById("config-textarea").value);
    } catch {
        return null;
    }
}

function showConfigMsg(text, isError = false) {
    const el = document.getElementById("config-msg");
    el.textContent = text;
    el.className = "config-msg " + (isError ? "error" : "ok");
    if (isError) setTimeout(() => { el.textContent = ""; el.className = "config-msg"; }, 4000);
}

async function applyConfig() {
    const payload = getConfigFromTextarea();
    if (!payload) { showConfigMsg("JSON inválido — revisá la sintaxis", true); return; }

    const res = await fetch("/api/config/apply", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
    });
    const result = await res.json();
    if (result.error) { showConfigMsg(result.error, true); return; }

    showConfigMsg("Configuración aplicada");
    document.getElementById("chart-sensor").innerHTML = "";
    await loadState();
    await loadHistory();
    setTimeout(closeConfigModal, 1200);
}

async function saveConfig() {
    const name = document.getElementById("cfg-save-name").value.trim();
    if (!name) { showConfigMsg("Ingresá un nombre para el escenario", true); return; }

    const payload = getConfigFromTextarea();
    if (!payload) { showConfigMsg("JSON inválido — revisá la sintaxis", true); return; }

    const res = await fetch(`/api/config/save/${encodeURIComponent(name)}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
    });
    const result = await res.json();
    if (result.error) { showConfigMsg(result.error, true); return; }

    document.getElementById("cfg-save-name").value = "";
    showConfigMsg(`Escenario "${name}" guardado`);
    document.getElementById("chart-sensor").innerHTML = "";
    await loadScenarios();
    await loadState();
    await loadHistory();
    setTimeout(closeConfigModal, 1200);
}

// =====================================================
// INIT
// =====================================================

document.getElementById("scenario-select").addEventListener("change", changeScenario);
document.getElementById("btn-open-config").addEventListener("click", openConfigModal);
document.getElementById("btn-close-modal").addEventListener("click", closeConfigModal);
document.getElementById("btn-apply-config").addEventListener("click", applyConfig);
document.getElementById("btn-save-config").addEventListener("click", saveConfig);

document.getElementById("config-modal").addEventListener("click", e => {
    if (e.target === e.currentTarget) closeConfigModal();
});

document.addEventListener("keydown", e => {
    if (e.key === "Escape") closeConfigModal();
});

initTabs();
loadState();
loadScenarios();
initChart();

setInterval(() => {
    loadState();
    historyTick++;
    if (historyTick % 5 === 0) loadHistory();
}, 2000);
