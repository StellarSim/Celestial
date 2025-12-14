package main

import (
	"celestial/internal/config"
	"celestial/internal/gm"
	"celestial/internal/mission"
	"celestial/internal/network"
	"celestial/internal/simulation"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Println("Celestial Bridge Simulator - Starting")

	cfg, err := config.LoadConfig("configs/server.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	shipClasses, err := config.LoadShipClasses("configs/ships")
	if err != nil {
		log.Fatalf("Failed to load ship classes: %v", err)
	}

	panelMappings, err := config.LoadPanelMappings("configs/panels.yaml")
	if err != nil {
		log.Fatalf("Failed to load panel mappings: %v", err)
	}

	sim := simulation.NewSimulator(cfg.TickRate, shipClasses)
	go sim.Start()

	missionEngine := mission.NewEngine(sim)
	if err := missionEngine.LoadMissions("missions"); err != nil {
		log.Fatalf("Failed to load missions: %v", err)
	}

	gmController := gm.NewController(sim, missionEngine)

	wsServer := network.NewWebSocketServer(cfg.WebSocketPort, sim, gmController)
	go wsServer.Start()

	tcpServer := network.NewTCPServer(cfg.TCPPort, sim, panelMappings)
	go tcpServer.Start()

	log.Printf("WebSocket server listening on :%d", cfg.WebSocketPort)
	log.Printf("TCP server listening on :%d", cfg.TCPPort)
	log.Println("Celestial Bridge Simulator - Running")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	sim.Stop()
	wsServer.Stop()
	tcpServer.Stop()
	time.Sleep(100 * time.Millisecond)
	log.Println("Shutdown complete")
}
