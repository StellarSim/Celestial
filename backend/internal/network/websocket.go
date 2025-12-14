package network

import (
	"celestial/internal/gm"
	"celestial/internal/ship"
	"celestial/internal/simulation"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketServer struct {
	port         int
	simulator    *simulation.Simulator
	gmController *gm.Controller
	clients      map[*Client]bool
	mu           sync.RWMutex
	upgrader     websocket.Upgrader
	stopChan     chan struct{}
	server       *http.Server
}

type Client struct {
	conn          *websocket.Conn
	clientType    string
	stationRole   string
	send          chan []byte
	lastHeartbeat time.Time
}

type Message struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

func NewWebSocketServer(port int, sim *simulation.Simulator, gmCtrl *gm.Controller) *WebSocketServer {
	return &WebSocketServer{
		port:         port,
		simulator:    sim,
		gmController: gmCtrl,
		clients:      make(map[*Client]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		stopChan: make(chan struct{}),
	}
}

func (ws *WebSocketServer) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.handleWebSocket)

	ws.server = &http.Server{
		Addr:    ":" + string(rune(ws.port)),
		Handler: mux,
	}

	go ws.broadcastLoop()
	go ws.heartbeatLoop()

	log.Printf("WebSocket server starting on port %d", ws.port)
	if err := http.ListenAndServe(":8080", mux); err != nil && err != http.ErrServerClosed {
		log.Printf("WebSocket server error: %v", err)
	}
}

func (ws *WebSocketServer) Stop() {
	close(ws.stopChan)
	if ws.server != nil {
		ws.server.Close()
	}

	ws.mu.Lock()
	for client := range ws.clients {
		client.conn.Close()
	}
	ws.mu.Unlock()
}

func (ws *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		conn:          conn,
		send:          make(chan []byte, 256),
		lastHeartbeat: time.Now(),
	}

	ws.mu.Lock()
	ws.clients[client] = true
	ws.mu.Unlock()

	log.Printf("New WebSocket client connected from %s", conn.RemoteAddr())

	go ws.writePump(client)
	go ws.readPump(client)

	ws.sendFullState(client)
}

func (ws *WebSocketServer) readPump(client *Client) {
	defer func() {
		ws.mu.Lock()
		delete(ws.clients, client)
		ws.mu.Unlock()
		client.conn.Close()
		log.Printf("Client disconnected: %s", client.conn.RemoteAddr())
	}()

	client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		client.lastHeartbeat = time.Now()
		return nil
	})

	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		ws.handleMessage(client, &msg)
	}
}

func (ws *WebSocketServer) writePump(client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (ws *WebSocketServer) handleMessage(client *Client, msg *Message) {
	switch msg.Type {
	case "register":
		clientType, _ := msg.Payload["client_type"].(string)
		stationRole, _ := msg.Payload["station_role"].(string)
		client.clientType = clientType
		client.stationRole = stationRole
		log.Printf("Client registered as %s (role: %s)", clientType, stationRole)

	case "input":
		ws.handleInput(client, msg.Payload)

	case "gm_command":
		ws.handleGMCommand(client, msg.Payload)

	case "request_state":
		ws.sendFullState(client)

	case "heartbeat":
		client.lastHeartbeat = time.Now()
	}
}

func (ws *WebSocketServer) handleInput(client *Client, payload map[string]interface{}) {
	inputType, _ := payload["input_type"].(string)

	switch inputType {
	case "hotas":
		ws.handleHOTASInput(payload)
	case "ui_action":
		ws.handleUIAction(payload)
	}
}

func (ws *WebSocketServer) handleHOTASInput(payload map[string]interface{}) {
	shipID, _ := payload["ship_id"].(string)
	pitch, _ := payload["pitch"].(float64)
	yaw, _ := payload["yaw"].(float64)
	roll, _ := payload["roll"].(float64)
	thrust, _ := payload["thrust"].(float64)

	sh := ws.simulator.GetShip(shipID)
	if sh != nil {
		sh.ApplyRotation(pitch, yaw, roll)
		sh.ApplyThrust(0, 0, thrust)
	}
}

func (ws *WebSocketServer) handleUIAction(payload map[string]interface{}) {
	action, _ := payload["action"].(string)
	log.Printf("UI action: %s", action)
}

func (ws *WebSocketServer) handleGMCommand(client *Client, payload map[string]interface{}) {
	command, _ := payload["command"].(string)

	switch command {
	case "pause":
		ws.simulator.Pause()
	case "resume":
		ws.simulator.Resume()
	case "create_snapshot":
		ws.simulator.CreateSnapshot()
	case "restore_snapshot":
		index, _ := payload["index"].(float64)
		ws.simulator.RestoreSnapshot(int(index))
		ws.broadcastFullState()
	case "spawn_ship":
		ws.handleSpawnShip(payload)
	case "remove_ship":
		shipID, _ := payload["ship_id"].(string)
		ws.simulator.RemoveShip(shipID)
	}
}

func (ws *WebSocketServer) handleSpawnShip(payload map[string]interface{}) {
	shipID, _ := payload["ship_id"].(string)
	classID, _ := payload["class_id"].(string)
	name, _ := payload["name"].(string)
	isPlayer, _ := payload["is_player"].(bool)

	posData, _ := payload["position"].(map[string]interface{})
	position := ship.Vector3{
		X: posData["x"].(float64),
		Y: posData["y"].(float64),
		Z: posData["z"].(float64),
	}

	ws.simulator.SpawnShip(shipID, classID, name, isPlayer, position)
}

func (ws *WebSocketServer) sendFullState(client *Client) {
	state := ws.buildStateMessage()
	data, err := json.Marshal(state)
	if err != nil {
		log.Printf("Error marshaling state: %v", err)
		return
	}

	select {
	case client.send <- data:
	default:
		log.Printf("Client send buffer full, dropping state update")
	}
}

func (ws *WebSocketServer) broadcastLoop() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ws.stopChan:
			return
		case <-ticker.C:
			ws.broadcastFullState()
		}
	}
}

func (ws *WebSocketServer) broadcastFullState() {
	state := ws.buildStateMessage()
	data, err := json.Marshal(state)
	if err != nil {
		log.Printf("Error marshaling state: %v", err)
		return
	}

	ws.mu.RLock()
	clients := make([]*Client, 0, len(ws.clients))
	for client := range ws.clients {
		clients = append(clients, client)
	}
	ws.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.send <- data:
		default:
		}
	}
}

func (ws *WebSocketServer) buildStateMessage() Message {
	ships := ws.simulator.GetAllShips()
	shipData := make(map[string]interface{})

	for id, sh := range ships {
		shipData[id] = map[string]interface{}{
			"id":       sh.ID,
			"name":     sh.Name,
			"class_id": sh.ClassID,
			"position": map[string]float64{
				"x": sh.Position.X,
				"y": sh.Position.Y,
				"z": sh.Position.Z,
			},
			"velocity": map[string]float64{
				"x": sh.Velocity.X,
				"y": sh.Velocity.Y,
				"z": sh.Velocity.Z,
			},
			"rotation": map[string]float64{
				"w": sh.Rotation.W,
				"x": sh.Rotation.X,
				"y": sh.Rotation.Y,
				"z": sh.Rotation.Z,
			},
			"systems": ws.buildSystemsData(sh),
		}
	}

	return Message{
		Type: "state_update",
		Payload: map[string]interface{}{
			"time":  ws.simulator.CurrentTime,
			"ships": shipData,
		},
	}
}

func (ws *WebSocketServer) buildSystemsData(sh *ship.Ship) map[string]interface{} {
	engines := make(map[string]interface{})
	for id, eng := range sh.Engines {
		engines[id] = map[string]interface{}{
			"health":  eng.Health,
			"enabled": eng.Enabled,
			"on_fire": eng.OnFire,
		}
	}

	weapons := make(map[string]interface{})
	for id, wpn := range sh.Weapons {
		weapons[id] = map[string]interface{}{
			"health":   wpn.Health,
			"enabled":  wpn.Enabled,
			"cooldown": wpn.Cooldown,
			"armed":    wpn.Armed,
			"loaded":   wpn.Loaded,
			"locked":   wpn.Locked,
			"ammo":     wpn.AmmoCount,
			"on_fire":  wpn.OnFire,
		}
	}

	shields := make(map[string]interface{})
	for id, em := range sh.Shields.Emitters {
		shields[id] = map[string]interface{}{
			"strength": em.Strength,
			"health":   em.Health,
			"facing":   em.Facing,
			"on_fire":  em.OnFire,
		}
	}

	hull := make(map[string]interface{})
	for id, sec := range sh.Hull.Sections {
		hull[id] = map[string]interface{}{
			"armor":    sec.Armor,
			"health":   sec.Health,
			"breached": sec.Breached,
			"on_fire":  sec.OnFire,
		}
	}

	return map[string]interface{}{
		"engines": engines,
		"weapons": weapons,
		"shields": shields,
		"hull":    hull,
		"power": map[string]interface{}{
			"current":     sh.Power.CurrentCapacity,
			"max":         sh.Power.MaxCapacity,
			"generation":  sh.Power.Generation,
			"consumption": sh.Power.Consumption,
		},
	}
}

func (ws *WebSocketServer) heartbeatLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ws.stopChan:
			return
		case <-ticker.C:
			ws.checkHeartbeats()
		}
	}
}

func (ws *WebSocketServer) checkHeartbeats() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	timeout := 30 * time.Second
	now := time.Now()

	for client := range ws.clients {
		if now.Sub(client.lastHeartbeat) > timeout {
			log.Printf("Client timeout: %s", client.conn.RemoteAddr())
			client.conn.Close()
			delete(ws.clients, client)
		}
	}
}
