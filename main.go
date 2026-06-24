package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"time"

	"simupi/internal/api"
	"simupi/internal/config"
	"simupi/internal/history"
	mqttpkg "simupi/internal/mqtt"
)

//go:embed web/static web/templates scenarios
var embedFS embed.FS

// version is set at build time via -ldflags "-X main.version=<tag>"
var version = "dev"

func main() {
	httpPort := envOr("PORT", "5000")
	mqttPort := 1883

	// Embedded MQTT broker
	broker, err := mqttpkg.NewBroker(mqttPort)
	if err != nil {
		log.Fatalf("mqtt broker: %v", err)
	}
	broker.Start()
	time.Sleep(100 * time.Millisecond) // let TCP listener bind

	// Core components
	scenarioMgr := config.NewScenarioManager(embedFS)
	scenarios := scenarioMgr.List()
	if len(scenarios) == 0 {
		log.Fatal("no scenarios found")
	}

	hist := history.NewHistoryManager()
	mqttClient := mqttpkg.NewClient("tcp://localhost:1883", nil)

	// HTTP / API server
	srv := api.NewServer(embedFS, scenarioMgr, mqttClient, hist)
	if err := srv.Init(scenarios[0]); err != nil {
		log.Fatalf("init: %v", err)
	}

	if err := mqttClient.Start(); err != nil {
		log.Printf("warning: mqtt client connect: %v", err)
	}

	srv.Start()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	log.Printf("SimuPi %s — http://localhost:%s  |  MQTT on :%d  |  scenario: %s",
		version, httpPort, mqttPort, scenarios[0])
	log.Fatal(http.ListenAndServe(":"+httpPort, mux))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
