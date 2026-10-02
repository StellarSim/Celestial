package input

import (
	"celestial/internal/ship"
	"celestial/internal/simulation"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Action is a catalog entry as it arrives on the wire. `Value` carries the
// target object, e.g. {"section":"forward"} or {"bay_id":2}. Handlers must read
// the target from Value, never from System.
type Action struct {
	Role   string
	System string
	Action string
	Value  interface{}
}

type ActionHandler func(action *Action) error

// ActionRouter is the single action catalog: {role, system, action} -> handler.
// WebSocket screens, ESP32 panels (via configs/panels.yaml) and Lua all route
// through here.
type ActionRouter struct {
	simulator *simulation.Simulator
	handlers  map[string]ActionHandler

	mu    sync.Mutex
	teams []repairTeam
	// teamTimers paces deployed repair teams so repairs are continuous.
	teamTimers []float64

	orders     []string
	logEntries []string
	commsLog   []string

	waypoints []waypoint
	autopilot bool
	probes    int

	frequency       float64
	hailing         bool
	hailTarget      string
	shieldFrequency float64

	scan scanState

	transporterActive    bool
	transporterEmergency bool

	selfDestructAt float64
}

func NewActionRouter(sim *simulation.Simulator) *ActionRouter {
	router := &ActionRouter{
		simulator: sim,
		handlers:  make(map[string]ActionHandler),
		probes:    5,
		scan:      scanState{Mode: "passive", Duration: 6.0},
	}
	router.teams = []repairTeam{
		{Name: "Alpha Team", Status: "standing_by"},
		{Name: "Beta Team", Status: "standing_by"},
		{Name: "Gamma Team", Status: "standing_by"},
	}
	router.teamTimers = make([]float64, len(router.teams))
	router.register()
	return router
}

func (ar *ActionRouter) register() {
	ar.handle("engineer", "power", "toggle_breaker", ar.handleToggleBreaker)
	ar.handle("engineer", "power", "route_power", ar.handleRoutePower)
	ar.handle("engineer", "damage", "repair", ar.handleRepair)
	ar.handle("engineer", "damage", "extinguish", ar.handleExtinguish)
	ar.handle("engineer", "damage", "seal_breach", ar.handleSealBreach)
	ar.handle("engineer", "repair", "deploy_team", ar.handleDeployRepairTeam)
	ar.handle("engineer", "repair", "recall_team", ar.handleRecallRepairTeam)
	ar.handle("engineer", "repair", "repair_team", ar.handleRepairTeamSection)

	ar.handle("flight", "flight", "set_throttle", ar.handleSetThrottle)
	ar.handle("flight", "flight", "set_turn", ar.handleSetTurn)
	ar.handle("flight", "navigation", "set_waypoint", ar.handleSetWaypoint)
	ar.handle("flight", "navigation", "clear_waypoint", ar.handleClearWaypoint)
	ar.handle("flight", "navigation", "engage", ar.handleEngageAutopilot)
	ar.handle("flight", "navigation", "disengage", ar.handleDisengageAutopilot)
	ar.handle("flight", "docking", "release", ar.handleReleaseDocking)

	ar.handle("weapons", "weapons", "set_target", ar.handleSetTarget)
	ar.handle("weapons", "weapons", "clear_target", ar.handleClearTarget)
	ar.handle("weapons", "torpedo", "arm", ar.handleTorpedoArm)
	ar.handle("weapons", "torpedo", "load", ar.handleTorpedoLoad)
	ar.handle("weapons", "torpedo", "fire", ar.handleTorpedoFire)
	ar.handle("weapons", "phaser", "set_enabled", ar.handleSetPhaserEnabled)
	ar.handle("weapons", "phaser", "fire", ar.handlePhaserFire)

	ar.handle("captain", "alert", "set_level", ar.handleSetAlert)
	ar.handle("captain", "command", "issue_order", ar.handleIssueOrder)
	ar.handle("captain", "command", "clear_orders", ar.handleClearOrders)
	ar.handle("captain", "comms", "hail", ar.handleHail)
	ar.handle("captain", "comms", "broadcast", ar.handleBroadcast)

	// The bridge alert bar is reachable from every station.
	for _, role := range []string{
		"engineer", "flight", "weapons", "communications",
		"operations", "relay", "first_officer",
	} {
		ar.handle(role, "alert", "set_level", ar.handleSetAlert)
	}
	ar.handle("captain", "self_destruct", "arm", ar.handleSelfDestruct)
	ar.handle("captain", "self_destruct", "abort", ar.handleSelfDestructAbort)

	ar.handle("communications", "comms", "hail", ar.handleHail)
	ar.handle("communications", "comms", "send_message", ar.handleSendMessage)
	ar.handle("communications", "comms", "broadcast", ar.handleBroadcast)
	ar.handle("communications", "comms", "set_frequency", ar.handleSetFrequency)
	ar.handle("communications", "comms", "initiate_scan", ar.handleInitiateScan)
	ar.handle("communications", "comms", "deep_scan", ar.handleDeepScan)

	ar.handle("operations", "sensors", "set_mode", ar.handleSetSensorMode)
	ar.handle("operations", "sensors", "initiate_scan", ar.handleInitiateScan)
	ar.handle("operations", "sensors", "deep_scan", ar.handleDeepScan)
	ar.handle("operations", "shields", "raise", ar.handleRaiseShields)
	ar.handle("operations", "shields", "lower", ar.handleLowerShields)
	ar.handle("operations", "shields", "set_frequency", ar.handleShieldFrequency)
	ar.handle("operations", "shields", "rotate_frequency", ar.handleRotateShieldFrequency)
	ar.handle("operations", "transporter", "beam_up", ar.handleTransporterBeamUp)
	ar.handle("operations", "transporter", "beam_down", ar.handleTransporterBeamDown)
	ar.handle("operations", "transporter", "emergency", ar.handleTransporterEmergency)

	ar.handle("relay", "navigation", "set_waypoint", ar.handleSetWaypoint)
	ar.handle("relay", "navigation", "clear_waypoint", ar.handleClearWaypoint)
	ar.handle("relay", "sensors", "set_mode", ar.handleSetSensorMode)
	ar.handle("relay", "sensors", "mark_target", ar.handleMarkTarget)
	ar.handle("relay", "sensors", "initiate_scan", ar.handleInitiateScan)
	ar.handle("relay", "sensors", "deep_scan", ar.handleDeepScan)
	ar.handle("relay", "sensors", "launch_probe", ar.handleLaunchProbe)

	ar.handle("first_officer", "crew", "deploy_team", ar.handleDeployRepairTeam)
	ar.handle("first_officer", "crew", "recall_team", ar.handleRecallRepairTeam)
	ar.handle("first_officer", "crew", "assign_order", ar.handleAssignOrder)
	ar.handle("first_officer", "log", "add_entry", ar.handleAddLogEntry)
}

func (ar *ActionRouter) handle(role, system, action string, fn ActionHandler) {
	ar.handlers[catalogKey(role, system, action)] = fn
}

func catalogKey(role, system, action string) string {
	return role + "." + system + "." + action
}

// CatalogKeys returns every registered {role, system, action} key, sorted.
func (ar *ActionRouter) CatalogKeys() []string {
	keys := make([]string, 0, len(ar.handlers))
	for k := range ar.handlers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (ar *ActionRouter) RouteAction(action *Action) error {
	if action == nil {
		return fmt.Errorf("nil action")
	}
	if action.System == "" || action.Action == "" {
		return fmt.Errorf("action requires system and action")
	}

	role := action.Role
	if role == "" {
		return fmt.Errorf("action requires role")
	}

	key := catalogKey(role, action.System, action.Action)
	handler, ok := ar.handlers[key]
	if !ok {
		return fmt.Errorf("unknown action: %s", key)
	}
	return handler(action)
}

// ---------------------------------------------------------------------------
// Value helpers: handlers read targets from Value only.
// ---------------------------------------------------------------------------

func valueDict(action *Action) map[string]interface{} {
	switch v := action.Value.(type) {
	case map[string]interface{}:
		return v
	case nil:
		return map[string]interface{}{}
	default:
		return map[string]interface{}{"value": action.Value}
	}
}

func dictString(d map[string]interface{}, key string) string {
	if v, ok := d[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func dictFloat(d map[string]interface{}, key string) (float64, bool) {
	v, ok := d[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case bool:
		if n {
			return 1, true
		}
		return 0, true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func dictInt(d map[string]interface{}, key string) (int, bool) {
	f, ok := dictFloat(d, key)
	if !ok {
		return 0, false
	}
	return int(f), true
}

func dictBool(d map[string]interface{}, key string) (bool, bool) {
	v, ok := d[key]
	if !ok {
		return false, false
	}
	switch b := v.(type) {
	case bool:
		return b, true
	case float64:
		return b != 0, true
	case int:
		return b != 0, true
	case string:
		return b == "true" || b == "on" || b == "1", true
	default:
		return false, false
	}
}

func (ar *ActionRouter) getPlayerShip() *ship.Ship {
	for _, sh := range ar.simulator.GetAllShips() {
		if sh.IsPlayer {
			return sh
		}
	}
	return nil
}

func (ar *ActionRouter) playerShip(action *Action) (*ship.Ship, error) {
	d := valueDict(action)
	if id := dictString(d, "ship_id"); id != "" {
		sh := ar.simulator.GetShip(id)
		if sh == nil {
			return nil, fmt.Errorf("ship not found: %s", id)
		}
		return sh, nil
	}
	sh := ar.getPlayerShip()
	if sh == nil {
		return nil, fmt.Errorf("no player ship")
	}
	return sh, nil
}

// findTorpedoBay resolves a bay target from value: {"bay_id":N} or {"bay":"torpedo_bay_2"}.
func findTorpedoBay(sh *ship.Ship, d map[string]interface{}) (*ship.Weapon, error) {
	weapons := sh.WeaponsSnapshot()

	if bayID, ok := dictInt(d, "bay_id"); ok {
		key := fmt.Sprintf("torpedo_bay_%d", bayID)
		if w, ok := weapons[key]; ok && w.Type == "torpedo" {
			return &w, nil
		}
		return nil, fmt.Errorf("torpedo bay not found: %d", bayID)
	}

	if bay := dictString(d, "bay"); bay != "" {
		if w, ok := weapons[bay]; ok && w.Type == "torpedo" {
			return &w, nil
		}
		return nil, fmt.Errorf("torpedo bay not found: %s", bay)
	}

	// Default to the first torpedo bay.
	keys := make([]string, 0, len(weapons))
	for k, w := range weapons {
		if w.Type == "torpedo" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no torpedo bays")
	}
	sort.Strings(keys)
	w := weapons[keys[0]]
	return &w, nil
}

func findPhaserArray(sh *ship.Ship, d map[string]interface{}) (*ship.Weapon, error) {
	weapons := sh.WeaponsSnapshot()

	if arrayID := dictString(d, "array_id"); arrayID != "" {
		if w, ok := weapons[arrayID]; ok && w.Type != "torpedo" {
			return &w, nil
		}
		return nil, fmt.Errorf("phaser array not found: %s", arrayID)
	}

	keys := make([]string, 0, len(weapons))
	for k, w := range weapons {
		if w.Type != "torpedo" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no phaser arrays")
	}
	sort.Strings(keys)
	w := weapons[keys[0]]
	return &w, nil
}

func normalizeSection(s string) (string, error) {
	switch strings.ToLower(s) {
	case "forward", "bow", "fore":
		return ship.SectionForward, nil
	case "aft", "stern":
		return ship.SectionAft, nil
	case "port", "left":
		return ship.SectionPort, nil
	case "starboard", "right":
		return ship.SectionStarboard, nil
	default:
		return "", fmt.Errorf("unknown hull section: %s", s)
	}
}
