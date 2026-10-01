package network

import (
	"celestial/internal/gm"
	"celestial/internal/input"
	"celestial/internal/ship"
	"celestial/internal/simulation"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketServer struct {
	port         int
	simulator    *simulation.Simulator
	gmController *gm.Controller
	actionRouter *input.ActionRouter
	clients      map[*Client]bool
	mu           sync.RWMutex
	upgrader     websocket.Upgrader
	stopChan     chan struct{}
	server       *http.Server
}

type Client struct {
	conn          *websocket.Conn
	role          string
	clientID      string
	send          chan []byte
	lastHeartbeat time.Time
}

func NewWebSocketServer(port int, sim *simulation.Simulator, gmCtrl *gm.Controller) *WebSocketServer {
	return &WebSocketServer{
		port:         port,
		simulator:    sim,
		gmController: gmCtrl,
		actionRouter: input.NewActionRouter(sim),
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
	mux.HandleFunc("/", ws.handleWebSocket)

	ws.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", ws.port),
		Handler: mux,
	}

	go ws.broadcastLoop()
	go ws.heartbeatLoop()

	log.Printf("WebSocket server starting on port %d", ws.port)
	if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("WebSocket server error: %v", err)
	}
}

// ActionRouter exposes the shared catalog so the sim tick can advance
// server-owned timers (scan progress, self destruct, repair teams).
func (ws *WebSocketServer) ActionRouter() *input.ActionRouter {
	return ws.actionRouter
}

func (ws *WebSocketServer) Stop() {
	select {
	case <-ws.stopChan:
		return
	default:
		close(ws.stopChan)
	}
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

		var raw map[string]interface{}
		if err := json.Unmarshal(message, &raw); err != nil {
			log.Printf("Error parsing message: %v", err)
			ws.sendError(client, "invalid JSON")
			continue
		}

		msgType, _ := raw["type"].(string)
		if msgType == "" {
			ws.sendError(client, "missing type field")
			continue
		}

		ws.handleMessage(client, msgType, raw)
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
		case <-ws.stopChan:
			return
		}
	}
}

func (ws *WebSocketServer) handleMessage(client *Client, msgType string, raw map[string]interface{}) {
	// A malformed client message must never take the server down.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Client %s sent a message that could not be handled: %v", client.role, r)
			ws.sendError(client, "message could not be handled")
		}
	}()
	switch msgType {
	case "register":
		role, _ := raw["role"].(string)
		clientID, _ := raw["client_id"].(string)
		if role == "" {
			role, _ = raw["client_type"].(string)
		}
		if clientID == "" {
			clientID, _ = raw["station_role"].(string)
		}
		client.role = role
		client.clientID = clientID
		log.Printf("Client registered as role=%s id=%s", role, clientID)
		ws.sendFeedback(client, "registered", "", "")
		ws.sendFullState(client)

	case "action":
		ws.handleAction(client, raw)

	case "gm_command":
		ws.handleGMCommand(client, raw)

	case "request_state":
		ws.sendFullState(client)

	case "heartbeat":
		client.lastHeartbeat = time.Now()
		ws.sendRaw(client, map[string]interface{}{"type": "heartbeat"})

	default:
		ws.sendError(client, fmt.Sprintf("unknown message type: %s", msgType))
	}
}

func (ws *WebSocketServer) handleAction(client *Client, raw map[string]interface{}) {
	system, _ := raw["system"].(string)
	action, _ := raw["action"].(string)
	role, _ := raw["role"].(string)
	if role == "" {
		role = client.role
	}
	value := raw["value"]

	if system == "" || action == "" {
		ws.sendError(client, "action requires system and action")
		return
	}

	act := &input.Action{
		Role:   role,
		System: system,
		Action: action,
		Value:  value,
	}
	if err := ws.actionRouter.RouteAction(act); err != nil {
		ws.sendError(client, err.Error())
		return
	}
	ws.sendFeedback(client, "success", action, "")
}

func (ws *WebSocketServer) handleGMCommand(client *Client, raw map[string]interface{}) {
	if client.role != "" && client.role != "gm" {
		ws.sendError(client, "gm_command requires gm role")
		return
	}
	command, _ := raw["command"].(string)
	if command == "" {
		ws.sendError(client, "gm_command requires command")
		return
	}

	switch command {
	case "pause":
		ws.simulator.Pause()
		ws.sendFeedback(client, "success", command, "")
	case "resume":
		ws.simulator.Resume()
		ws.sendFeedback(client, "success", command, "")
	case "create_snapshot":
		ws.simulator.CreateSnapshot()
		ws.sendFeedback(client, "success", command, "")
	case "restore_snapshot":
		idx := getIntParam(raw, "snapshot_index", -1)
		if idx < 0 {
			idx = getIntParam(raw, "index", -1)
		}
		if idx < 0 {
			ws.sendError(client, "restore_snapshot requires snapshot_index")
			return
		}
		if err := ws.simulator.RestoreSnapshot(idx); err != nil {
			ws.sendError(client, err.Error())
			return
		}
		ws.sendFeedback(client, "success", command, "")
		ws.broadcastFullState()
	case "spawn_ship":
		if err := ws.handleSpawnShip(raw); err != nil {
			ws.sendError(client, err.Error())
			return
		}
		ws.sendFeedback(client, "success", command, "")
	case "remove_ship", "destroy_ship":
		shipID, _ := raw["ship_id"].(string)
		if shipID == "" {
			ws.sendError(client, "remove_ship requires ship_id")
			return
		}
		ws.simulator.RemoveShip(shipID)
		ws.sendFeedback(client, "success", command, "")
	case "modify_ship":
		if err := ws.handleModifyShip(raw); err != nil {
			ws.sendError(client, err.Error())
			return
		}
		ws.sendFeedback(client, "success", command, "")
	case "set_alert":
		level, _ := raw["level"].(string)
		if level == "" {
			ws.sendError(client, "set_alert requires level")
			return
		}
		ws.simulator.SetAlertLevel(level)
		for _, sh := range ws.simulator.GetAllShips() {
			if sh.IsPlayer {
				sh.SetAlertLevel(level)
			}
		}
		ws.sendFeedback(client, "success", command, "")
	case "set_ai_difficulty":
		value := getFloatParam(raw, "value", -1)
		if value < 0 {
			ws.sendError(client, "set_ai_difficulty requires value")
			return
		}
		shipID, _ := raw["ship_id"].(string)
		if shipID != "" {
			ws.gmController.SetAIDifficulty(shipID, value)
		} else {
			for id := range ws.simulator.AIControllers {
				ws.gmController.SetAIDifficulty(id, value)
			}
		}
		ws.sendFeedback(client, "success", command, "")
	case "start_mission":
		missionID, _ := raw["mission"].(string)
		if missionID == "" {
			missionID, _ = raw["mission_id"].(string)
		}
		if missionID == "" {
			ws.sendError(client, "start_mission requires mission")
			return
		}
		if err := ws.gmController.StartMission(missionID); err != nil {
			ws.sendError(client, err.Error())
			return
		}
		ws.sendFeedback(client, "success", command, "")
	case "stop_mission", "mission_restart":
		ws.gmController.StopMission()
		ws.sendFeedback(client, "success", command, "")
	case "trigger_mission_event":
		event, _ := raw["event"].(string)
		if event == "" {
			ws.sendError(client, "trigger_mission_event requires event")
			return
		}
		data := map[string]interface{}{}
		if d, ok := raw["data"].(map[string]interface{}); ok {
			data = d
		}
		ws.gmController.TriggerEvent(event, data)
		ws.sendFeedback(client, "success", command, "")
	case "mission_win", "mission_lose":
		ws.BroadcastMissionEvent(command, map[string]interface{}{})
		ws.sendFeedback(client, "success", command, "")
	default:
		ws.sendError(client, fmt.Sprintf("unknown gm command: %s", command))
	}
}

func (ws *WebSocketServer) handleSpawnShip(raw map[string]interface{}) error {
	shipID, _ := raw["ship_id"].(string)
	classID, _ := raw["class_id"].(string)
	if classID == "" {
		classID, _ = raw["class"].(string)
	}
	name, _ := raw["name"].(string)
	isPlayer, _ := raw["is_player"].(bool)

	if shipID == "" {
		return fmt.Errorf("spawn_ship requires ship_id")
	}
	if classID == "" {
		return fmt.Errorf("spawn_ship requires class_id")
	}
	if name == "" {
		name = shipID
	}

	posMap, _ := raw["position"].(map[string]interface{})
	if posMap == nil {
		return fmt.Errorf("spawn_ship requires position")
	}
	x, ok := toFloat(posMap["x"])
	if !ok {
		return fmt.Errorf("spawn_ship position.x must be a number")
	}
	y, ok := toFloat(posMap["y"])
	if !ok {
		return fmt.Errorf("spawn_ship position.y must be a number")
	}
	z, ok := toFloat(posMap["z"])
	if !ok {
		return fmt.Errorf("spawn_ship position.z must be a number")
	}

	position := ship.Vector3{X: x, Y: y, Z: z}
	return ws.simulator.SpawnShip(shipID, classID, name, isPlayer, position)
}

func (ws *WebSocketServer) handleModifyShip(raw map[string]interface{}) error {
	shipID, _ := raw["ship_id"].(string)
	if shipID == "" {
		return fmt.Errorf("modify_ship requires ship_id")
	}
	system, _ := raw["system"].(string)
	value := raw["value"]
	if system == "" {
		return fmt.Errorf("modify_ship requires system")
	}
	sh := ws.simulator.GetShip(shipID)
	if sh == nil {
		return fmt.Errorf("ship not found: %s", shipID)
	}
	// Optional section target: top-level "section" or a {"section","amount"} dict.
	section := ""
	if s, _ := raw["section"].(string); s != "" {
		section = s
	}
	amount := 0.0
	hasAmount := false
	if m, ok := value.(map[string]interface{}); ok {
		if s, _ := m["section"].(string); s != "" {
			section = s
		}
		if f, ok := toFloat(m["amount"]); ok {
			amount, hasAmount = f, true
		} else if f, ok := toFloat(m["value"]); ok {
			amount, hasAmount = f, true
		}
	} else if f, ok := toFloat(value); ok {
		amount, hasAmount = f, true
	}
	if !hasAmount {
		return fmt.Errorf("modify_ship requires a numeric value")
	}
	return modifyShipHull(sh, system, section, amount)
}

// modifyShipHull applies GM hull damage/heal through the locked Ship mutators.
// Negative amounts damage, positive amounts repair. Both go through TakeDamage
// and RepairSection so no raw hull pointers escape the ship lock.
func modifyShipHull(sh *ship.Ship, system, section string, amount float64) error {
	switch system {
	case "hull", "damage", "health":
		if amount < 0 {
			loc := section
			if loc == "" {
				loc = "forward"
			}
			sh.TakeDamage(-amount, loc)
			return nil
		}
		if section != "" {
			sh.RepairSection(section, amount)
			return nil
		}
		for sec := range sh.HullSnapshot() {
			sh.RepairSection(sec, amount)
		}
		return nil
	default:
		return fmt.Errorf("modify_ship: unsupported system %q", system)
	}
}

func (ws *WebSocketServer) sendFeedback(client *Client, status, action, message string) {
	msg := map[string]interface{}{
		"type":   "feedback",
		"status": status,
	}
	if action != "" {
		msg["action"] = action
	}
	if message != "" {
		msg["message"] = message
	}
	ws.sendRaw(client, msg)
}

func (ws *WebSocketServer) sendError(client *Client, message string) {
	ws.sendRaw(client, map[string]interface{}{
		"type":    "error",
		"message": message,
	})
}

func (ws *WebSocketServer) sendRaw(client *Client, obj map[string]interface{}) {
	data, err := json.Marshal(obj)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}
	select {
	case client.send <- data:
	default:
		log.Printf("Client send buffer full, dropping message")
	}
}

func (ws *WebSocketServer) BroadcastMissionEvent(event string, data map[string]interface{}) {
	if data == nil {
		data = map[string]interface{}{}
	}
	msg := map[string]interface{}{
		"type":  "mission_event",
		"event": event,
		"data":  data,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
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
		case client.send <- payload:
		default:
		}
	}
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

var canonicalBreakers = []string{"reactor", "engines", "shields", "weapons", "sensors", "comms", "life_support", "navigation"}

func (ws *WebSocketServer) buildStateMessage() map[string]interface{} {
	ships := ws.simulator.GetAllShips()
	shipArr := make([]interface{}, 0, len(ships))
	alertLevel := ws.simulator.GetAlertLevel()
	scanActive, scanTarget, scanProgress, scanMode := ws.actionRouter.ScanState()
	hailing, hailTarget, frequency := ws.actionRouter.CommState()
	shieldFreq := ws.actionRouter.ShieldFrequency()
	transporterActive, transporterEmergency := ws.actionRouter.TransporterState()
	selfDestruct := ws.actionRouter.SelfDestructRemaining()
	for _, sh := range ships {
		shipArr = append(shipArr, ws.buildShipData(sh, alertLevel, stationState{
			scanActive: scanActive, scanTarget: scanTarget, scanProgress: scanProgress, scanMode: scanMode,
			hailing: hailing, hailTarget: hailTarget, frequency: frequency, shieldFrequency: shieldFreq,
			transporterActive: transporterActive, transporterEmergency: transporterEmergency,
			selfDestructRemaining: selfDestruct,
		}))
	}

	projs := ws.simulator.GetAllProjectiles()
	projArr := make([]interface{}, 0, len(projs))
	for _, p := range projs {
		projArr = append(projArr, map[string]interface{}{
			"id":        p.ID,
			"type":      p.Type,
			"position":  map[string]float64{"x": p.Position.X, "y": p.Position.Y, "z": p.Position.Z},
			"velocity":  map[string]float64{"x": p.Velocity.X, "y": p.Velocity.Y, "z": p.Velocity.Z},
			"owner_id":  p.SourceID,
			"target_id": p.TargetID,
		})
	}

	msg := map[string]interface{}{
		"type":               "state_update",
		"time":               ws.simulator.GetCurrentTime(),
		"paused":             ws.simulator.IsPaused(),
		"alert_level":        alertLevel,
		"ships":              shipArr,
		"projectiles":        projArr,
		"orders":             ws.actionRouter.Orders(),
		"waypoints":          ws.actionRouter.Waypoints(),
		"repair_teams":       ws.actionRouter.RepairTeams(),
		"probes":             ws.actionRouter.Probes(),
		"communications_log": ws.actionRouter.CommsLog(),
		"log_entries":        ws.actionRouter.LogEntries(),
		"autopilot":          ws.actionRouter.Autopilot(),
		"auto_fire":          ws.actionRouter.AutoFire(),
	}
	if ws.gmController != nil {
		if m := ws.gmController.GetActiveMission(); m != nil {
			objs := make([]interface{}, 0, len(m.Objectives))
			for _, o := range m.Objectives {
				objs = append(objs, map[string]interface{}{
					"id": o.ID, "description": o.Description, "complete": o.Completed,
				})
			}
			name := m.Name
			if name == "" {
				name = m.ID
			}
			msg["mission"] = map[string]interface{}{
				"id": m.ID, "name": name, "objectives": objs,
			}
		}
		msg["missions"] = ws.gmController.GetMissionIDs()
		msg["active_mission"] = ws.gmController.GetActiveMissionID()
	} else {
		msg["missions"] = []interface{}{}
		msg["active_mission"] = ""
	}
	snapshots := ws.simulator.SnapshotInfo()
	snapArr := make([]interface{}, 0, len(snapshots))
	for _, s := range snapshots {
		snapArr = append(snapArr, s)
	}
	msg["snapshots"] = snapArr
	msg["snapshot_count"] = len(snapshots)
	return msg
}

// stationState carries ship-agnostic values the server owns and every client
// renders. Values the client used to fake locally (scan progress, self destruct
// countdown, shield echo) are computed here.
type stationState struct {
	scanActive   bool
	scanTarget   string
	scanProgress float64
	scanMode     string

	hailing    bool
	hailTarget string
	frequency  float64

	shieldFrequency float64

	transporterActive    bool
	transporterEmergency bool

	selfDestructRemaining float64
}

func (ws *WebSocketServer) buildShipData(sh *ship.Ship, globalAlert string, st stationState) map[string]interface{} {
	// Clone once so the broadcast reads a consistent snapshot without racing
	// the sim tick. All field reads below are on the private copy.
	sh = sh.Clone()
	faction := sh.Faction
	if faction == "" {
		if sh.IsPlayer {
			faction = "player"
		} else {
			faction = "hostile"
		}
	}
	alert := sh.AlertLevel
	if alert == "" {
		alert = globalAlert
	}
	if alert == "" {
		alert = "normal"
	}

	hull, maxHull := sumHull(sh)
	shields, maxShields := sumShields(sh)
	facings := shieldFacings(sh)
	breakers := map[string]interface{}{}
	for _, b := range canonicalBreakers {
		if sh.Power != nil && sh.Power.Breakers != nil {
			if br, ok := sh.Power.Breakers[b]; ok {
				breakers[b] = br.Enabled
				continue
			}
		}
		breakers[b] = true
	}
	powerAvail, powerTotal := 0.0, 0.0
	if sh.Power != nil {
		powerAvail = sh.Power.CurrentCapacity
		powerTotal = sh.Power.MaxCapacity
	}

	engines := []interface{}{}
	if sh.Engines != nil {
		for _, e := range sh.Engines {
			engines = append(engines, map[string]interface{}{
				"engine_id": e.ID,
				"kind":      e.Type,
				"health":    e.Health,
				"enabled":   e.Enabled,
				"thrust":    map[string]float64{"x": 0, "y": 0, "z": e.Thrust},
			})
		}
	}

	torpedoBays := []interface{}{}
	phaserArrays := []interface{}{}
	if sh.Weapons != nil {
		for _, w := range sh.Weapons {
			if w.Type == "torpedo" {
				torpedoBays = append(torpedoBays, map[string]interface{}{
					"bay_id": wAmmoID(w.ID), "id": w.ID,
					"armed": w.Armed, "loaded": w.Loaded, "locked": w.Locked,
					"ammo": w.AmmoCount, "max_ammo": w.AmmoCapacity,
					"cooldown": w.Cooldown, "target_id": sh.TargetID,
				})
			} else {
				phaserArrays = append(phaserArrays, map[string]interface{}{
					"array_id": w.ID, "id": w.ID,
					"facing": map[string]float64{"x": 0, "y": 0, "z": 1},
					"health": w.Health, "cooldown": w.Cooldown, "power_level": 100.0,
				})
			}
		}
	}

	damageSections := map[string]interface{}{}
	if sh.Hull != nil {
		for id, sec := range sh.Hull.Sections {
			fires := 0
			if sec.OnFire {
				fires = 1
			}
			breaches := 0
			if sec.Breached {
				breaches = 1
			}
			damageSections[id] = map[string]interface{}{
				"health": sec.Health, "fires": fires, "breaches": breaches, "crew_trapped": 0,
			}
		}
	}

	oxygen, temp := 21.0, 20.0
	if sh.LifeSupport != nil && len(sh.LifeSupport.Compartments) > 0 {
		sumO, sumT, n := 0.0, 0.0, 0.0
		for _, c := range sh.LifeSupport.Compartments {
			sumO += c.Oxygen
			sumT += c.Temperature
			n++
		}
		if n > 0 {
			oxygen = sumO / n
			temp = sumT / n
		}
	}

	crew := map[string]interface{}{}
	for role, c := range sh.Crew {
		crew[role] = map[string]interface{}{
			"health": c.Health, "stress": 0.0, "available": c.Status == "healthy",
		}
	}

	sensorHealth, sensorEnabled := 100.0, true
	if sh.Subsystems != nil {
		if s, ok := sh.Subsystems["sensors"]; ok {
			sensorHealth = s.Health
			sensorEnabled = s.Enabled
		}
	}
	commsHealth, commsEnabled := 100.0, true
	if sh.Subsystems != nil {
		if s, ok := sh.Subsystems["comms"]; ok {
			commsHealth = s.Health
			commsEnabled = s.Enabled
		}
	}

	return map[string]interface{}{
		"id": sh.ID, "name": sh.Name, "faction": faction, "is_player": sh.IsPlayer,
		"class":          sh.ClassID,
		"position":       map[string]float64{"x": sh.Position.X, "y": sh.Position.Y, "z": sh.Position.Z},
		"velocity":       map[string]float64{"x": sh.Velocity.X, "y": sh.Velocity.Y, "z": sh.Velocity.Z},
		"rotation":       map[string]float64{"w": sh.Rotation.W, "x": sh.Rotation.X, "y": sh.Rotation.Y, "z": sh.Rotation.Z},
		"hull_integrity": hull, "max_hull": maxHull,
		"shields": shields, "max_shields": maxShields, "shields_enabled": shieldsEnabled(sh),
		"shield_facings":   facings,
		"shield_frequency": st.shieldFrequency,
		"power_available":  powerAvail, "power_total": powerTotal,
		"power_breakers":  breakers,
		"engines":         engines,
		"weapons":         map[string]interface{}{"torpedo_bays": torpedoBays, "phaser_arrays": phaserArrays},
		"damage_sections": damageSections,
		"life_support":    map[string]interface{}{"oxygen_level": oxygen, "temperature": temp, "gravity": 1.0, "enabled": sh.BreakerOn("life_support")},
		"crew":            crew,
		"sensors": map[string]interface{}{
			"health": sensorHealth, "enabled": sensorEnabled,
			"scan_active": st.scanActive, "scan_target": st.scanTarget,
			"scan_progress": st.scanProgress, "mode": st.scanMode,
		},
		"communications": map[string]interface{}{
			"health": commsHealth, "enabled": commsEnabled,
			"hailing": st.hailing, "hail_target": st.hailTarget, "frequency": st.frequency,
		},
		"transporter":             map[string]interface{}{"active": st.transporterActive, "emergency": st.transporterEmergency},
		"self_destruct_remaining": st.selfDestructRemaining,
		"docked":                  sh.Docked,
		"target_id":               sh.TargetID,
		"alert_level":             alert,
	}
}

func sumHull(sh *ship.Ship) (float64, float64) {
	total, max := 0.0, 0.0
	if sh.Hull != nil {
		for _, s := range sh.Hull.Sections {
			total += s.Health
			max += s.MaxHealth
		}
	}
	return total, max
}

func sumShields(sh *ship.Ship) (float64, float64) {
	total, max := 0.0, 0.0
	if sh.Shields != nil {
		for _, e := range sh.Shields.Emitters {
			total += e.Strength
			max += e.MaxStrength
		}
	}
	return total, max
}

func shieldsEnabled(sh *ship.Ship) bool {
	if sh.Shields == nil {
		return false
	}
	return sh.Shields.Enabled
}

func shieldFacings(sh *ship.Ship) map[string]interface{} {
	out := map[string]interface{}{"fore": 0.0, "aft": 0.0, "port": 0.0, "starboard": 0.0}
	if sh.Shields != nil {
		for _, e := range sh.Shields.Emitters {
			f := strings.ToLower(e.Facing)
			if f == "forward" {
				f = "fore"
			}
			if _, ok := out[f]; ok {
				out[f] = e.Strength
			}
		}
	}
	return out
}

func wAmmoID(id string) int {
	parts := strings.Split(id, "_")
	if len(parts) == 0 {
		return 0
	}
	n, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0
	}
	return n
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func getIntParam(raw map[string]interface{}, key string, def int) int {
	v, ok := raw[key]
	if !ok {
		return def
	}
	if f, ok := toFloat(v); ok {
		return int(f)
	}
	return def
}

func getFloatParam(raw map[string]interface{}, key string, def float64) float64 {
	v, ok := raw[key]
	if !ok {
		return def
	}
	if f, ok := toFloat(v); ok {
		return f
	}
	return def
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
			ws.sendHeartbeat()
		}
	}
}

func (ws *WebSocketServer) sendHeartbeat() {
	payload, _ := json.Marshal(map[string]interface{}{"type": "heartbeat"})
	ws.mu.RLock()
	clients := make([]*Client, 0, len(ws.clients))
	for client := range ws.clients {
		clients = append(clients, client)
	}
	ws.mu.RUnlock()
	for _, client := range clients {
		select {
		case client.send <- payload:
		default:
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
