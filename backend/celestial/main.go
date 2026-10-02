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

func resolvePath(p string) string {
	candidates := []string{
		p,
		"backend/" + p,
		"../backend/" + p,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return p
}

func main() {
	log.Println("Celestial Bridge Simulator - Starting")

	cfg, err := config.LoadConfig(resolvePath("configs/server.yaml"))
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	shipClasses, err := config.LoadShipClasses(resolvePath("configs/ships"))
	if err != nil {
		log.Fatalf("Failed to load ship classes: %v", err)
	}

	objectClasses, err := config.LoadObjectClasses(resolvePath("configs/objects"))
	if err != nil {
		log.Fatalf("Failed to load object classes: %v", err)
	}

	factions, err := config.LoadFactions(resolvePath("configs/factions"))
	if err != nil {
		log.Fatalf("Failed to load factions: %v", err)
	}

	panelMappings, err := config.LoadPanelMappings(resolvePath("configs/panels.yaml"))
	if err != nil {
		log.Fatalf("Failed to load panel mappings: %v", err)
	}

	sim := simulation.NewSimulator(cfg.TickRate, shipClasses, objectClasses, factions)
	go sim.Start()

	missionEngine := mission.NewEngine(sim)
	if err := missionEngine.LoadMissions(resolvePath("missions")); err != nil {
		log.Fatalf("Failed to load missions: %v", err)
	}
	sim.OnEvent = missionEngine.TriggerEvent

	gmController := gm.NewController(sim, missionEngine)

	wsServer := network.NewWebSocketServer(cfg.WebSocketPort, sim, gmController)
	wsServer.SetPanelMappings(panelMappings)
	missionEngine.OnEvent = wsServer.BroadcastMissionEvent
	sim.OnTick = wsServer.ActionRouter().Update
	go wsServer.Start()

	tcpServer := network.NewTCPServer(cfg.TCPPort, sim, panelMappings, wsServer.ActionRouter())
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
	gmController.Stop()
	time.Sleep(100 * time.Millisecond)
	log.Println("Shutdown complete")
}
