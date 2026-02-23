package main

import (
	"log"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	cnt "github.com/R3DPanda1/LWN-Sim-Plus/controllers"
	"github.com/R3DPanda1/LWN-Sim-Plus/models"
	repo "github.com/R3DPanda1/LWN-Sim-Plus/repositories"
	"github.com/R3DPanda1/LWN-Sim-Plus/shared"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/logging"
	ws "github.com/R3DPanda1/LWN-Sim-Plus/webserver"
)

// Entry point of the program.
func main() {
	// Load the configuration file, and if there is an error, log it and terminate the program.
	cfg, err := models.GetConfigFile("config.json")
	if err != nil {
		log.Fatal(err)
	}

	logging.Setup(cfg.Logging)
	slog.Info("simulator starting", "version", shared.Version)

	// Check if the verbose flag is set to true, and if so, enable verbose logging.
	if cfg.Verbose {
		shared.Verbose = true
		shared.DebugPrint("Verbose mode enabled")
	}
	// Create a new simulator controller and repository.
	simulatorRepository := repo.NewSimulatorRepository()
	simulatorController := cnt.NewSimulatorController(simulatorRepository)
	simulatorController.GetInstance()
	simulatorController.SetPerformance(cfg.Performance)
	simulatorController.SetEvents(cfg.Events)
	slog.Info("simulator ready", "version", shared.Version)
	// Start the metrics server.
	go startMetrics(cfg)
	// If the autoStart flag is set to true, start the simulator automatically.
	if cfg.AutoStart {
		slog.Info("auto-starting simulation")
		simulatorController.Run()
	} else {
		slog.Info("autostart not enabled")
	}
	// Start the web server and serve WebUI
	WebServer := ws.NewWebServer(cfg, simulatorController)
	WebServer.Run()
	slog.Info("webUI online")
}

// Prometheus metrics server
func startMetrics(cfg *models.ServerConfig) {
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(cfg.Address+":"+strconv.Itoa(cfg.MetricsPort), nil)
	if err != nil {
		slog.Error("metrics server failed", "error", err)
	}
}
