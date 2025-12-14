package network

import (
	"bufio"
	"celestial/internal/config"
	"celestial/internal/input"
	"celestial/internal/panel"
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
}

type PanelMessage struct {
	PanelID string      `json:"panel_id"`
	Action  string      `json:"action"`
	Value   interface{} `json:"value"`
}

func NewTCPServer(port int, sim *simulation.Simulator, mappings *config.PanelMapping) *TCPServer {
	return &TCPServer{
		port:              port,
		simulator:         sim,
		panelMappings:     mappings,
		connections:       make(map[string]*PanelConnection),
		stopChan:          make(chan struct{}),
		actionRouter:      input.NewActionRouter(sim),
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
	var msg PanelMessage
	if err := json.Unmarshal([]byte(message), &msg); err != nil {
		log.Printf("Error parsing panel message: %v", err)
		return
	}

	if msg.PanelID != "" && panelConn.panelID == "" {
		panelConn.panelID = msg.PanelID
		log.Printf("Panel %s registered", msg.PanelID)
	}

	if msg.Action == "register" {
		ts.sendFeedback(panelConn.conn, msg.PanelID, "registered", "")
		return
	}

	panelConfig, ok := ts.panelMappings.Panels[msg.PanelID]
	if !ok {
		log.Printf("Unknown panel ID: %s", msg.PanelID)
		return
	}

	actionDef, ok := panelConfig.Actions[msg.Action]
	if !ok {
		log.Printf("Unknown action %s for panel %s", msg.Action, msg.PanelID)
		return
	}

	action := &input.Action{
		Role:   panelConfig.Role,
		System: actionDef.System,
		Action: actionDef.Action,
		Value:  msg.Value,
	}

	if err := ts.actionRouter.RouteAction(action); err != nil {
		log.Printf("Error routing action: %v", err)
		ts.sendFeedback(panelConn.conn, msg.PanelID, "error", err.Error())
	} else {
		ts.sendFeedback(panelConn.conn, msg.PanelID, "success", "")
	}
}

func (ts *TCPServer) sendFeedback(conn net.Conn, panelID, status, message string) {
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

	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
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
			ships := ts.simulator.GetAllShips()
			for _, sh := range ships {
				if !sh.IsPlayer {
					continue
				}

				ts.mu.RLock()
				for _, panelConn := range ts.connections {
					if panelConn.panelID == "" {
						continue
					}

					state := ts.panelStateManager.UpdateFromShip(panelConn.panelID, sh, ts.simulator.CurrentTime)
					ts.sendPanelState(panelConn.conn, state)
				}
				ts.mu.RUnlock()
			}
		}
	}
}

func (ts *TCPServer) sendPanelState(conn net.Conn, state *panel.PanelState) {
	message := map[string]interface{}{
		"type":  "state_update",
		"state": state,
	}

	data, err := json.Marshal(message)
	if err != nil {
		return
	}

	data = append(data, '\n')
	conn.Write(data)
}
