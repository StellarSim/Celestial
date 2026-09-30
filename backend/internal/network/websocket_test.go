package network

import (
	"celestial/internal/gm"
	"celestial/internal/mission"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type testServer struct {
	http    *httptest.Server
	ws      *WebSocketServer
	sim     interface{ Tick() }
	stopped chan struct{}
}

func newTestWSServer(t *testing.T, gmCtrl *gm.Controller) (*WebSocketServer, *httptest.Server) {
	t.Helper()
	sim := newTestSim()
	ws := NewWebSocketServer(0, sim, gmCtrl)
	if gmCtrl != nil {
		ws.gmController = gmCtrl
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.handleWebSocket)
	server := httptest.NewServer(mux)

	go ws.broadcastLoop()
	t.Cleanup(func() {
		ws.Stop()
		server.Close()
	})
	return ws, server
}

type testClient struct {
	conn     *websocket.Conn
	messages chan map[string]interface{}
}

// dialTest connects and starts a single background reader. gorilla forbids
// reading again after a failed read, so all reads go through the channel.
func dialTest(t *testing.T, server *httptest.Server) *testClient {
	t.Helper()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	t.Cleanup(func() { conn.Close() })

	client := &testClient{conn: conn, messages: make(chan map[string]interface{}, 512)}
	go func() {
		defer close(client.messages)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			client.messages <- msg
		}
	}()
	return client
}

// readUntil returns the first queued message satisfying match.
func (c *testClient) readUntil(t *testing.T, match func(map[string]interface{}) bool) map[string]interface{} {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case msg, ok := <-c.messages:
			if !ok {
				t.Fatal("connection closed while waiting for a message")
			}
			if match(msg) {
				return msg
			}
		case <-deadline:
			t.Fatal("timed out waiting for matching message")
		}
	}
}

func isType(want string) func(map[string]interface{}) bool {
	return func(m map[string]interface{}) bool { return m["type"] == want }
}

func send(t *testing.T, conn *websocket.Conn, msg map[string]interface{}) {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestRegisterRepliesRegistered(t *testing.T) {
	_, server := newTestWSServer(t, nil)
	client := dialTest(t, server)

	send(t, client.conn, map[string]interface{}{
		"type": "register", "role": "engineer", "client_id": "engineer_station_1",
	})

	msg := client.readUntil(t, func(m map[string]interface{}) bool {
		return m["type"] == "feedback" && m["status"] == "registered"
	})
	if msg["status"] != "registered" {
		t.Errorf("Expected registered status, got %v", msg["status"])
	}
}

func TestRegisterSendsStateUpdate(t *testing.T) {
	_, server := newTestWSServer(t, nil)
	client := dialTest(t, server)

	send(t, client.conn, map[string]interface{}{"type": "register", "role": "captain", "client_id": "c1"})

	msg := client.readUntil(t, isType("state_update"))
	for _, key := range []string{"time", "paused", "alert_level", "ships", "projectiles"} {
		if _, ok := msg[key]; !ok {
			t.Errorf("state_update missing top-level key %q", key)
		}
	}

	ships, ok := msg["ships"].([]interface{})
	if !ok {
		t.Fatalf("ships must be an array, got %T", msg["ships"])
	}
	if len(ships) != 1 {
		t.Fatalf("Expected 1 ship, got %d", len(ships))
	}

	ship, ok := ships[0].(map[string]interface{})
	if !ok {
		t.Fatal("ship entry must be an object")
	}
	for _, key := range []string{
		"id", "name", "faction", "is_player", "class",
		"position", "velocity", "rotation",
		"hull_integrity", "max_hull", "shields", "max_shields", "shields_enabled",
		"shield_facings", "power_available", "power_total", "power_breakers",
		"engines", "weapons", "damage_sections", "life_support", "crew",
		"sensors", "communications", "docked", "alert_level",
	} {
		if _, ok := ship[key]; !ok {
			t.Errorf("ship state missing key %q", key)
		}
	}

	if _, hasClassID := ship["class_id"]; hasClassID {
		t.Error("Ship state must use 'class', not 'class_id'")
	}
	if _, hasSystems := ship["systems"]; hasSystems {
		t.Error("Ship state must not nest a 'systems' object")
	}

	engines, ok := ship["engines"].([]interface{})
	if !ok || len(engines) == 0 {
		t.Fatalf("engines must be a non-empty array, got %T", ship["engines"])
	}
	engine := engines[0].(map[string]interface{})
	for _, key := range []string{"engine_id", "kind", "health", "enabled", "thrust"} {
		if _, ok := engine[key]; !ok {
			t.Errorf("engine entry missing key %q", key)
		}
	}

	weapons := ship["weapons"].(map[string]interface{})
	if _, ok := weapons["torpedo_bays"]; !ok {
		t.Error("weapons must carry torpedo_bays")
	}
	if _, ok := weapons["phaser_arrays"]; !ok {
		t.Error("weapons must carry phaser_arrays")
	}

	breakers := ship["power_breakers"].(map[string]interface{})
	for _, name := range []string{
		"reactor", "engines", "shields", "weapons",
		"sensors", "comms", "life_support", "navigation",
	} {
		if _, ok := breakers[name]; !ok {
			t.Errorf("power_breakers missing %q", name)
		}
	}

	sections := ship["damage_sections"].(map[string]interface{})
	for _, name := range []string{"forward", "aft", "port", "starboard"} {
		if _, ok := sections[name]; !ok {
			t.Errorf("damage_sections missing canonical section %q", name)
		}
	}
	if _, hasBow := sections["bow"]; hasBow {
		t.Error("damage_sections must not use 'bow'")
	}
}

func TestActionProducesFeedback(t *testing.T) {
	_, server := newTestWSServer(t, nil)
	client := dialTest(t, server)

	send(t, client.conn, map[string]interface{}{"type": "register", "role": "flight", "client_id": "f1"})
	client.readUntil(t, func(m map[string]interface{}) bool {
		return m["type"] == "feedback" && m["status"] == "registered"
	})

	send(t, client.conn, map[string]interface{}{
		"type": "action", "role": "flight", "system": "flight", "action": "set_throttle",
		"value": map[string]interface{}{"throttle": 0.5},
	})

	msg := client.readUntil(t, func(m map[string]interface{}) bool {
		return m["type"] == "feedback" && m["status"] == "success"
	})
	if msg["action"] != "set_throttle" {
		t.Errorf("Feedback should name the action, got %v", msg["action"])
	}
}

func TestUnknownActionReturnsError(t *testing.T) {
	_, server := newTestWSServer(t, nil)
	client := dialTest(t, server)

	send(t, client.conn, map[string]interface{}{
		"type": "action", "role": "flight", "system": "flight", "action": "self_destruct",
	})

	msg := client.readUntil(t, isType("error"))
	if msg["message"] == "" {
		t.Error("Error message should not be empty")
	}
}

func TestNonGMCommandIsRejected(t *testing.T) {
	_, server := newTestWSServer(t, nil)
	client := dialTest(t, server)

	send(t, client.conn, map[string]interface{}{"type": "register", "role": "engineer", "client_id": "e1"})
	client.readUntil(t, func(m map[string]interface{}) bool {
		return m["type"] == "feedback" && m["status"] == "registered"
	})

	send(t, client.conn, map[string]interface{}{"type": "gm_command", "command": "pause"})

	msg := client.readUntil(t, isType("error"))
	if !strings.Contains(strings.ToLower(toString(msg["message"])), "gm") {
		t.Errorf("Error should explain the gm role requirement, got %v", msg["message"])
	}
}

func TestUnknownMessageTypeErrors(t *testing.T) {
	_, server := newTestWSServer(t, nil)
	client := dialTest(t, server)

	send(t, client.conn, map[string]interface{}{"type": "nonsense"})
	client.readUntil(t, isType("error"))
}

func TestGMCommands(t *testing.T) {
	sim := newTestSim()
	sim.SpawnShip("player", "cruiser", "Endeavour", true, ship0())
	missionEngine := mission.NewEngine(sim)
	gmCtrl := gm.NewController(sim, missionEngine)
	t.Cleanup(gmCtrl.Stop)

	ws := NewWebSocketServer(0, sim, gmCtrl)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.handleWebSocket)
	server := httptest.NewServer(mux)
	t.Cleanup(func() {
		ws.Stop()
		server.Close()
	})

	client := dialTest(t, server)
	send(t, client.conn, map[string]interface{}{"type": "register", "role": "gm", "client_id": "gm1"})
	client.readUntil(t, func(m map[string]interface{}) bool {
		return m["type"] == "feedback" && m["status"] == "registered"
	})

	send(t, client.conn, map[string]interface{}{"type": "gm_command", "command": "pause"})
	client.readUntil(t, func(m map[string]interface{}) bool {
		return m["type"] == "feedback" && m["status"] == "success"
	})
	if !sim.IsPaused() {
		t.Error("pause command should pause the simulator")
	}

	send(t, client.conn, map[string]interface{}{"type": "gm_command", "command": "resume"})
	client.readUntil(t, func(m map[string]interface{}) bool {
		return m["type"] == "feedback" && m["status"] == "success"
	})
	if sim.IsPaused() {
		t.Error("resume command should resume the simulator")
	}

	send(t, client.conn, map[string]interface{}{"type": "gm_command", "command": "create_snapshot"})
	client.readUntil(t, func(m map[string]interface{}) bool {
		return m["type"] == "feedback" && m["status"] == "success"
	})

	// restore_snapshot uses snapshot_index per the protocol.
	send(t, client.conn, map[string]interface{}{
		"type": "gm_command", "command": "restore_snapshot", "snapshot_index": 0,
	})
	client.readUntil(t, func(m map[string]interface{}) bool {
		return m["type"] == "feedback" && m["status"] == "success"
	})

	send(t, client.conn, map[string]interface{}{
		"type": "gm_command", "command": "spawn_ship",
		"class_id": "cruiser", "ship_id": "raider", "name": "Raider",
		"position": map[string]interface{}{"x": 100.0, "y": 0.0, "z": 500.0},
	})
	client.readUntil(t, func(m map[string]interface{}) bool {
		return m["type"] == "feedback" && m["status"] == "success"
	})
	if sim.GetShip("raider") == nil {
		t.Error("spawn_ship should create the ship")
	}

	// Missing required params must not panic.
	send(t, client.conn, map[string]interface{}{"type": "gm_command", "command": "spawn_ship"})
	client.readUntil(t, isType("error"))

	send(t, client.conn, map[string]interface{}{"type": "gm_command", "command": "set_alert", "level": "red"})
	client.readUntil(t, func(m map[string]interface{}) bool {
		return m["type"] == "feedback" && m["status"] == "success"
	})
	if sim.GetAlertLevel() != "red" {
		t.Errorf("set_alert should change the alert level, got %s", sim.GetAlertLevel())
	}

	send(t, client.conn, map[string]interface{}{"type": "gm_command", "command": "unknown_command"})
	client.readUntil(t, isType("error"))
}

func TestMissionEventBroadcast(t *testing.T) {
	sim := newTestSim()
	missionEngine := mission.NewEngine(sim)
	gmCtrl := gm.NewController(sim, missionEngine)
	t.Cleanup(gmCtrl.Stop)

	ws := NewWebSocketServer(0, sim, gmCtrl)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.handleWebSocket)
	server := httptest.NewServer(mux)
	t.Cleanup(func() {
		ws.Stop()
		server.Close()
	})

	client := dialTest(t, server)
	ws.BroadcastMissionEvent("objective_complete", map[string]interface{}{"objective_id": "patrol"})

	msg := client.readUntil(t, isType("mission_event"))
	if msg["event"] != "objective_complete" {
		t.Errorf("Expected objective_complete, got %v", msg["event"])
	}
	data, ok := msg["data"].(map[string]interface{})
	if !ok || data["objective_id"] != "patrol" {
		t.Errorf("mission_event should carry data, got %v", msg["data"])
	}
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
