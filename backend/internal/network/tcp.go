package network

import (
	"bufio"
	"celestial/internal/config"
	"celestial/internal/input"
	"celestial/internal/panel"
	"celestial/internal/ship"
	"celestial/internal/simulation"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type TCPServer struct {
	port              int
	simulator         *simulation.Simulator
	panelMappings     *config.PanelMapping
	listener          net.Listener
	connections       map[string]*PanelConnection
	mu                sync.RWMutex
	stopChan          chan struct{}
	actionRouter      *input.ActionRouter
	panelStateManager *panel.PanelStateManager
}

type PanelConnection struct {
	conn    net.Conn
	panelID string
	writeMu sync.Mutex
}

type PanelMessage struct {
	PanelID string      `json:"panel_id"`
	Action  string      `json:"action"`
	Value   interface{} `json:"value"`
}

// NewTCPServer shares the single action catalog with the WebSocket server so a
// panel input and a screen input take exactly the same path.
func NewTCPServer(port int, sim *simulation.Simulator, mappings *config.PanelMapping, router *input.ActionRouter) *TCPServer {
	return &TCPServer{
		port:              port,
		simulator:         sim,
		panelMappings:     mappings,
		connections:       make(map[string]*PanelConnection),
		stopChan:          make(chan struct{}),
		actionRouter:      router,
		panelStateManager: panel.NewPanelStateManager(),
	}
}

func (ts *TCPServer) Start() {
	var err error
	ts.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", ts.port))
	if err != nil {
		log.Fatalf("TCP server failed to start: %v", err)
	}

	log.Printf("TCP server listening on port %d", ts.port)

	go ts.broadcastPanelStates()

	for {
		select {
		case <-ts.stopChan:
			return
		default:
			conn, err := ts.listener.Accept()
			if err != nil {
				select {
				case <-ts.stopChan:
					return
				default:
					log.Printf("Error accepting TCP connection: %v", err)
					continue
				}
			}

			go ts.handleConnection(conn)
		}
	}
}

func (ts *TCPServer) Stop() {
	close(ts.stopChan)
	if ts.listener != nil {
		ts.listener.Close()
	}

	ts.mu.Lock()
	for _, panelConn := range ts.connections {
		panelConn.conn.Close()
	}
	ts.mu.Unlock()
}

func (ts *TCPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	log.Printf("New TCP connection from %s", remoteAddr)

	panelConn := &PanelConnection{
		conn:    conn,
		panelID: "",
	}

	ts.mu.Lock()
	ts.connections[remoteAddr] = panelConn
	ts.mu.Unlock()

	defer func() {
		ts.mu.Lock()
		delete(ts.connections, remoteAddr)
		ts.mu.Unlock()
		log.Printf("TCP connection closed: %s", remoteAddr)
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		ts.handleMessage(panelConn, line)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("TCP connection error: %v", err)
	}
}

func (ts *TCPServer) handleMessage(panelConn *PanelConnection, message string) {
	// A malformed panel message must never take the server down.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panel %s sent a message that could not be handled: %v", panelConn.panelID, r)
			ts.sendFeedback(panelConn, panelConn.panelID, "error", "message could not be handled")
		}
	}()
	var msg PanelMessage
	if err := json.Unmarshal([]byte(message), &msg); err != nil {
		log.Printf("Error parsing panel message: %v", err)
		ts.sendFeedback(panelConn, msg.PanelID, "error", "invalid JSON")
		return
	}

	if msg.PanelID != "" && panelConn.panelID == "" {
		panelConn.panelID = msg.PanelID
		log.Printf("Panel %s registered", msg.PanelID)
	}

	if msg.Action == "register" {
		if _, ok := ts.panelMappings.Panels[msg.PanelID]; !ok {
			ts.sendFeedback(panelConn, msg.PanelID, "error", "unknown panel_id")
			return
		}
		ts.sendFeedback(panelConn, msg.PanelID, "registered", "")
		return
	}

	panelConfig, ok := ts.panelMappings.Panels[msg.PanelID]
	if !ok {
		ts.sendFeedback(panelConn, msg.PanelID, "error", fmt.Sprintf("unknown panel_id: %s", msg.PanelID))
		return
	}

	actionDef, ok := panelConfig.Actions[msg.Action]
	if !ok {
		ts.sendFeedback(panelConn, msg.PanelID, "error", fmt.Sprintf("unknown action %s for panel %s", msg.Action, msg.PanelID))
		return
	}

	action := &input.Action{
		Role:   panelConfig.Role,
		System: actionDef.System,
		Action: actionDef.Action,
		Value:  mergeValue(actionDef.Value, msg.Value),
	}

	if err := ts.actionRouter.RouteAction(action); err != nil {
		log.Printf("Error routing panel action: %v", err)
		ts.sendFeedback(panelConn, msg.PanelID, "error", err.Error())
		return
	}
	ts.sendFeedback(panelConn, msg.PanelID, "success", "")
}

// mergeValue lets panels.yaml supply a static value (for example the breaker
// name) while the panel's own payload supplies the variable part. The static
// value comes from shared config, so it is copied rather than mutated.
func mergeValue(static interface{}, dynamic interface{}) interface{} {
	staticMap, _ := static.(map[string]interface{})
	dynamicMap, _ := dynamic.(map[string]interface{})

	if staticMap == nil && dynamicMap == nil {
		if dynamic != nil {
			return dynamic
		}
		return static
	}

	merged := make(map[string]interface{}, len(staticMap)+len(dynamicMap))
	for k, v := range staticMap {
		merged[k] = v
	}
	for k, v := range dynamicMap {
		merged[k] = v
	}
	return merged
}

func (ts *TCPServer) sendFeedback(panelConn *PanelConnection, panelID, status, message string) {
	feedback := map[string]interface{}{
		"type":     "feedback",
		"panel_id": panelID,
		"status":   status,
		"message":  message,
	}

	data, err := json.Marshal(feedback)
	if err != nil {
		log.Printf("Error marshaling feedback: %v", err)
		return
	}

	panelConn.writeMu.Lock()
	defer panelConn.writeMu.Unlock()
	if _, err := panelConn.conn.Write(append(data, '\n')); err != nil {
		log.Printf("Error sending feedback: %v", err)
	}
}

func (ts *TCPServer) broadcastPanelStates() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ts.stopChan:
			return
		case <-ticker.C:
			player := ts.playerShip()
			if player == nil {
				continue
			}
			// One deep copy per tick so panel serialization never races the sim.
			snapshot := player.Clone()
			now := ts.simulator.GetCurrentTime()

			ts.mu.RLock()
			conns := make([]*PanelConnection, 0, len(ts.connections))
			for _, c := range ts.connections {
				if c.panelID != "" {
					conns = append(conns, c)
				}
			}
			ts.mu.RUnlock()

			for _, c := range conns {
				state := ts.panelStateManager.UpdateFromShip(c.panelID, snapshot, now, ts.stationState())
				ts.sendPanelState(c, state)
			}
		}
	}
}

func (ts *TCPServer) stationState() panel.StationState {
	scanActive, scanTarget, scanProgress, scanMode := ts.actionRouter.ScanState()
	hailing, hailTarget, frequency := ts.actionRouter.CommState()
	transporterActive, transporterEmergency := ts.actionRouter.TransporterState()
	return panel.StationState{
		ScanActive: scanActive, ScanTarget: scanTarget,
		ScanProgress: scanProgress, ScanMode: scanMode,
		Hailing: hailing, HailTarget: hailTarget, Frequency: frequency,
		TransporterActive: transporterActive, TransporterEmergency: transporterEmergency,
	}
}

func (ts *TCPServer) playerShip() *ship.Ship {
	for _, sh := range ts.simulator.GetAllShips() {
		if sh.IsPlayer {
			return sh
		}
	}
	return nil
}

func (ts *TCPServer) sendPanelState(panelConn *PanelConnection, state *panel.PanelState) {
	message := map[string]interface{}{
		"type":  "state_update",
		"state": state,
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling panel state: %v", err)
		return
	}

	panelConn.writeMu.Lock()
	defer panelConn.writeMu.Unlock()
	if _, err := panelConn.conn.Write(append(data, '\n')); err != nil {
		log.Printf("Error sending panel state: %v", err)
	}
}
