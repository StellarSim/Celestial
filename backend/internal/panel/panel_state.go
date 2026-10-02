package panel

import (
	"celestial/internal/ship"
	"fmt"
	"math"
	"sync"
)

type PanelState struct {
	PanelID    string               `json:"panel_id"`
	Indicators map[string]Indicator `json:"indicators"`
	Displays   map[string]Display   `json:"displays"`
	Timestamp  float64              `json:"timestamp"`
}

type Indicator struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
	Color string      `json:"color"`
	Blink bool        `json:"blink"`
}

type Display struct {
	Type   string      `json:"type"`
	Value  interface{} `json:"value"`
	Unit   string      `json:"unit"`
	Format string      `json:"format"`
}

type PanelStateManager struct {
	mu     sync.RWMutex
	states map[string]*PanelState
}

func NewPanelStateManager() *PanelStateManager {
	return &PanelStateManager{
		states: make(map[string]*PanelState),
	}
}

// updaters maps the canonical panel IDs from backend/configs/panels.yaml to serializers.
var updaters = map[string]func(*PanelState, *ship.Ship, StationState){
	"engineer_power_main":  updateEngineerPowerPanel,
	"engineer_damage_main": updateEngineerDamagePanel,
	"engineer_systems":     updateEngineerSystemsPanel,
	"flight_main":          updateFlightMainPanel,
	"flight_navigation":    updateFlightNavigationPanel,
	"weapons_torpedos_1":   updateWeaponsTorpedosPanel1,
	"weapons_torpedos_2":   updateWeaponsTorpedosPanel2,
	"weapons_phasers":      updateWeaponsPhasersPanel,
	"captain_command":      updateCaptainCommandPanel,
	"captain_status":       updateCaptainStatusPanel,
	"comms_main":           updateCommsMainPanel,
	"operations_power":     updateOperationsPowerPanel,
	"operations_resources": updateOperationsResourcesPanel,
	"relay_sensors":        updateRelaySensorsPanel,
	"relay_scanning":       updateRelayScanningPanel,
	"first_officer_main":   updateFirstOfficerMainPanel,
}

func updateEngineerPowerPanel(state *PanelState, sh *ship.Ship, station StationState) {
	powerPercent := (sh.Power.CurrentCapacity / sh.Power.MaxCapacity) * 100

	state.Displays["power_level"] = Display{
		Type:   "numeric",
		Value:  powerPercent,
		Unit:   "%",
		Format: "%.1f",
	}

	state.Displays["power_generation"] = Display{
		Type:   "numeric",
		Value:  sh.Power.Generation,
		Unit:   "MW",
		Format: "%.0f",
	}

	state.Displays["power_consumption"] = Display{
		Type:   "numeric",
		Value:  sh.Power.Consumption,
		Unit:   "MW",
		Format: "%.0f",
	}

	color := "green"
	if powerPercent < 25 {
		color = "red"
	} else if powerPercent < 50 {
		color = "yellow"
	}

	state.Indicators["power_status"] = Indicator{
		Type:  "led",
		Value: true,
		Color: color,
		Blink: powerPercent < 15,
	}

	for _, id := range ship.BreakerNames {
		breaker, ok := sh.Power.Breakers[id]
		if !ok {
			continue
		}
		state.Indicators["breaker_"+id] = Indicator{
			Type:  "led",
			Value: breaker.Enabled,
			Color: "green",
			Blink: false,
		}

		state.Displays["breaker_load_"+id] = Display{
			Type:   "numeric",
			Value:  breaker.Load,
			Unit:   "MW",
			Format: "%.1f",
		}
	}
}

func updateEngineerDamagePanel(state *PanelState, sh *ship.Ship, station StationState) {
	// Iterate the canonical sections in a stable order so the panel layout does
	// not shuffle between updates.
	for _, id := range []string{
		ship.SectionForward, ship.SectionAft, ship.SectionPort, ship.SectionStarboard,
	} {
		section, ok := sh.Hull.Sections[id]
		if !ok {
			continue
		}
		healthPercent := (section.Health / section.MaxHealth) * 100

		color := "green"
		if healthPercent < 25 {
			color = "red"
		} else if healthPercent < 50 {
			color = "yellow"
		}

		state.Indicators["hull_"+id] = Indicator{
			Type:  "led",
			Value: true,
			Color: color,
			Blink: section.OnFire || section.Breached,
		}

		state.Displays["hull_health_"+id] = Display{
			Type:   "numeric",
			Value:  healthPercent,
			Unit:   "%",
			Format: "%.0f",
		}

		if section.OnFire {
			state.Indicators["fire_"+id] = Indicator{
				Type:  "led",
				Value: true,
				Color: "red",
				Blink: true,
			}
		}

		if section.Breached {
			state.Indicators["breach_"+id] = Indicator{
				Type:  "led",
				Value: true,
				Color: "red",
				Blink: true,
			}
		}
	}

	for id, comp := range sh.LifeSupport.Compartments {
		state.Displays["pressure_"+id] = Display{
			Type:   "numeric",
			Value:  comp.Pressure,
			Unit:   "kPa",
			Format: "%.1f",
		}

		state.Displays["oxygen_"+id] = Display{
			Type:   "numeric",
			Value:  comp.Oxygen,
			Unit:   "%",
			Format: "%.1f",
		}
	}
}

func updateEngineerSystemsPanel(state *PanelState, sh *ship.Ship, station StationState) {
	for id, engine := range sh.Engines {
		healthPercent := (engine.Health / engine.MaxHealth) * 100

		color := "green"
		if healthPercent < 25 {
			color = "red"
		} else if healthPercent < 50 {
			color = "yellow"
		}

		state.Indicators["engine_"+id] = Indicator{
			Type:  "led",
			Value: engine.Enabled,
			Color: color,
			Blink: engine.OnFire,
		}

		state.Displays["engine_health_"+id] = Display{
			Type:   "numeric",
			Value:  healthPercent,
			Unit:   "%",
			Format: "%.0f",
		}

		state.Displays["engine_thrust_"+id] = Display{
			Type:   "numeric",
			Value:  engine.Thrust * (healthPercent / 100),
			Unit:   "kN",
			Format: "%.0f",
		}
	}
}

func updateFlightMainPanel(state *PanelState, sh *ship.Ship, station StationState) {
	vel := sh.Velocity

	state.Displays["velocity_x"] = Display{
		Type:   "numeric",
		Value:  vel.X,
		Unit:   "m/s",
		Format: "%.1f",
	}

	state.Displays["velocity_y"] = Display{
		Type:   "numeric",
		Value:  vel.Y,
		Unit:   "m/s",
		Format: "%.1f",
	}

	state.Displays["velocity_z"] = Display{
		Type:   "numeric",
		Value:  vel.Z,
		Unit:   "m/s",
		Format: "%.1f",
	}

	speed := math.Sqrt(vel.X*vel.X + vel.Y*vel.Y + vel.Z*vel.Z)
	state.Displays["speed"] = Display{
		Type:   "numeric",
		Value:  speed,
		Unit:   "m/s",
		Format: "%.0f",
	}

	throttle := sh.Throttle
	state.Displays["throttle"] = Display{
		Type:   "numeric",
		Value:  throttle,
		Unit:   "%",
		Format: "%.0f",
	}

	state.Indicators["docked"] = Indicator{
		Type:  "led",
		Value: sh.Docked,
		Color: "blue",
		Blink: false,
	}
}

func updateFlightNavigationPanel(state *PanelState, sh *ship.Ship, station StationState) {
	pos := sh.Position

	state.Displays["position_x"] = Display{
		Type:   "numeric",
		Value:  pos.X,
		Unit:   "km",
		Format: "%.0f",
	}

	state.Displays["position_y"] = Display{
		Type:   "numeric",
		Value:  pos.Y,
		Unit:   "km",
		Format: "%.0f",
	}

	state.Displays["position_z"] = Display{
		Type:   "numeric",
		Value:  pos.Z,
		Unit:   "km",
		Format: "%.0f",
	}

	state.Displays["heading"] = Display{
		Type:   "numeric",
		Value:  headingDegrees(sh),
		Unit:   "°",
		Format: "%.1f",
	}
}

// headingDegrees returns the ship's compass heading in degrees.
func headingDegrees(sh *ship.Ship) float64 {
	fwd := sh.Forward()
	deg := math.Atan2(fwd.X, -fwd.Z) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	return deg
}

func updateWeaponsTorpedosPanel1(state *PanelState, sh *ship.Ship, station StationState) {
	updateTorpedoBay(state, sh, "torpedo_bay_1")
	updateTorpedoBay(state, sh, "torpedo_bay_2")
}

func updateWeaponsTorpedosPanel2(state *PanelState, sh *ship.Ship, station StationState) {
	updateTorpedoBay(state, sh, "torpedo_bay_3")
	updateTorpedoBay(state, sh, "torpedo_bay_4")
}

func updateTorpedoBay(state *PanelState, sh *ship.Ship, bayID string) {
	weapon, ok := sh.Weapons[bayID]
	if !ok {
		return
	}

	state.Indicators[bayID+"_armed"] = Indicator{
		Type:  "led",
		Value: weapon.Armed,
		Color: "yellow",
		Blink: false,
	}

	state.Indicators[bayID+"_loaded"] = Indicator{
		Type:  "led",
		Value: weapon.Loaded,
		Color: "green",
		Blink: false,
	}

	state.Displays[bayID+"_ammo"] = Display{
		Type:   "numeric",
		Value:  weapon.AmmoCount,
		Unit:   "",
		Format: "%d",
	}

	state.Displays[bayID+"_cooldown"] = Display{
		Type:   "numeric",
		Value:  weapon.Cooldown,
		Unit:   "s",
		Format: "%.1f",
	}

	healthPercent := (weapon.Health / weapon.MaxHealth) * 100
	state.Displays[bayID+"_health"] = Display{
		Type:   "numeric",
		Value:  healthPercent,
		Unit:   "%",
		Format: "%.0f",
	}
}

func updateWeaponsPhasersPanel(state *PanelState, sh *ship.Ship, station StationState) {
	for id, weapon := range sh.Weapons {
		if weapon.Type != "phaser" {
			continue
		}

		healthPercent := (weapon.Health / weapon.MaxHealth) * 100
		color := "green"
		if healthPercent < 25 {
			color = "red"
		} else if healthPercent < 50 {
			color = "yellow"
		}

		state.Indicators["phaser_"+id] = Indicator{
			Type:  "led",
			Value: weapon.Enabled && weapon.Health > 0,
			Color: color,
			Blink: weapon.Cooldown > 0,
		}

		state.Displays["phaser_health_"+id] = Display{
			Type:   "numeric",
			Value:  healthPercent,
			Unit:   "%",
			Format: "%.0f",
		}

		state.Displays["phaser_cooldown_"+id] = Display{
			Type:   "numeric",
			Value:  weapon.Cooldown,
			Unit:   "s",
			Format: "%.1f",
		}
	}

	state.Displays["target_id"] = Display{
		Type:   "text",
		Value:  sh.TargetID,
		Unit:   "",
		Format: "%s",
	}
}

func updateCaptainCommandPanel(state *PanelState, sh *ship.Ship, station StationState) {
	level := sh.AlertLevel
	if level == "" {
		level = "normal"
	}
	state.Indicators["alert_normal"] = Indicator{
		Type:  "led",
		Value: level == "normal",
		Color: "green",
		Blink: false,
	}
	state.Indicators["alert_yellow"] = Indicator{
		Type:  "led",
		Value: level == "yellow",
		Color: "yellow",
		Blink: level == "yellow",
	}
	state.Indicators["alert_red"] = Indicator{
		Type:  "led",
		Value: level == "red",
		Color: "red",
		Blink: level == "red",
	}

	for role, crew := range sh.Crew {
		healthPercent := crew.Health
		color := "green"
		if healthPercent < 25 {
			color = "red"
		} else if healthPercent < 50 {
			color = "yellow"
		}

		state.Indicators["crew_"+role] = Indicator{
			Type:  "led",
			Value: healthPercent > 0,
			Color: color,
			Blink: crew.Status != "healthy",
		}
	}
}

func updateCaptainStatusPanel(state *PanelState, sh *ship.Ship, station StationState) {
	totalHull := 0.0
	maxHull := 0.0
	for _, section := range sh.Hull.Sections {
		totalHull += section.Health
		maxHull += section.MaxHealth
	}
	hullPercent := 0.0
	if maxHull > 0 {
		hullPercent = (totalHull / maxHull) * 100
	}

	state.Displays["hull_integrity"] = Display{
		Type:   "numeric",
		Value:  hullPercent,
		Unit:   "%",
		Format: "%.0f",
	}

	totalShields := 0.0
	maxShields := 0.0
	for _, emitter := range sh.Shields.Emitters {
		totalShields += emitter.Strength
		maxShields += emitter.MaxStrength
	}
	shieldPercent := 0.0
	if maxShields > 0 {
		shieldPercent = (totalShields / maxShields) * 100
	}

	state.Displays["shield_strength"] = Display{
		Type:   "numeric",
		Value:  shieldPercent,
		Unit:   "%",
		Format: "%.0f",
	}

	powerPercent := (sh.Power.CurrentCapacity / sh.Power.MaxCapacity) * 100
	state.Displays["power_level"] = Display{
		Type:   "numeric",
		Value:  powerPercent,
		Unit:   "%",
		Format: "%.0f",
	}
}

func updateCommsMainPanel(state *PanelState, sh *ship.Ship, station StationState) {
	comms, ok := sh.Subsystems["comms"]
	if ok {
		healthPercent := (comms.Health / comms.MaxHealth) * 100
		state.Displays["comms_health"] = Display{
			Type:   "numeric",
			Value:  healthPercent,
			Unit:   "%",
			Format: "%.0f",
		}

		color := "green"
		if healthPercent < 25 {
			color = "red"
		} else if healthPercent < 50 {
			color = "yellow"
		}

		state.Indicators["comms_online"] = Indicator{
			Type:  "led",
			Value: comms.Enabled && sh.BreakerOn("comms") && healthPercent > 0,
			Color: color,
			Blink: false,
		}
	}

	state.Displays["frequency"] = Display{
		Type:   "numeric",
		Value:  station.Frequency,
		Unit:   "MHz",
		Format: "%.1f",
	}

	state.Indicators["hailing"] = Indicator{
		Type:  "led",
		Value: station.Hailing,
		Color: "green",
		Blink: station.Hailing,
	}

	if station.HailTarget != "" {
		state.Displays["hail_target"] = Display{
			Type:   "text",
			Value:  station.HailTarget,
			Unit:   "",
			Format: "%s",
		}
	}
}

func updateOperationsPowerPanel(state *PanelState, sh *ship.Ship, station StationState) {
	state.Indicators["shields_enabled"] = Indicator{
		Type:  "led",
		Value: sh.Shields.Enabled,
		Color: "blue",
		Blink: false,
	}

	for id, emitter := range sh.Shields.Emitters {
		strengthPercent := (emitter.Strength / emitter.MaxStrength) * 100
		state.Displays["shield_"+id] = Display{
			Type:   "numeric",
			Value:  strengthPercent,
			Unit:   "%",
			Format: "%.0f",
		}

		color := "green"
		if strengthPercent < 25 {
			color = "red"
		} else if strengthPercent < 50 {
			color = "yellow"
		}

		state.Indicators["shield_"+id+"_status"] = Indicator{
			Type:  "led",
			Value: true,
			Color: color,
			Blink: false,
		}
	}
}

func updateOperationsResourcesPanel(state *PanelState, sh *ship.Ship, station StationState) {
	for id, bay := range sh.LaunchBays {
		state.Displays["bay_"+id+"_count"] = Display{
			Type:   "numeric",
			Value:  bay.Current,
			Unit:   fmt.Sprintf("/%d", bay.Capacity),
			Format: "%d",
		}
	}

	state.Indicators["transporter_active"] = Indicator{
		Type:  "led",
		Value: station.TransporterActive,
		Color: "blue",
		Blink: station.TransporterEmergency,
	}

	state.Displays["transporter_status"] = Display{
		Type:   "text",
		Value:  transporterLabel(station),
		Unit:   "",
		Format: "%s",
	}
}

func transporterLabel(station StationState) string {
	if station.TransporterEmergency {
		return "EMERGENCY"
	}
	if station.TransporterActive {
		return "BEAM ACTIVE"
	}
	return "READY"
}

func updateRelaySensorsPanel(state *PanelState, sh *ship.Ship, station StationState) {
	sensors, ok := sh.Subsystems["sensors"]
	if ok {
		healthPercent := (sensors.Health / sensors.MaxHealth) * 100
		state.Displays["sensors_health"] = Display{
			Type:   "numeric",
			Value:  healthPercent,
			Unit:   "%",
			Format: "%.0f",
		}

		color := "green"
		if healthPercent < 25 {
			color = "red"
		} else if healthPercent < 50 {
			color = "yellow"
		}

		state.Indicators["sensors_online"] = Indicator{
			Type:  "led",
			Value: sensors.Enabled && sh.BreakerOn("sensors") && healthPercent > 0,
			Color: color,
			Blink: false,
		}
	}

	state.Indicators["scan_active"] = Indicator{
		Type:  "led",
		Value: station.ScanActive,
		Color: "blue",
		Blink: station.ScanActive,
	}

	state.Displays["scan_progress"] = Display{
		Type:   "numeric",
		Value:  station.ScanProgress * 100,
		Unit:   "%",
		Format: "%.0f",
	}

	if station.ScanTarget != "" {
		state.Displays["scan_target"] = Display{
			Type:   "text",
			Value:  station.ScanTarget,
			Unit:   "",
			Format: "%s",
		}
	}
}

// StationState carries the server-owned values a panel renders that are not
// part of the ship snapshot itself (scan progress, transporter, comms).
type StationState struct {
	ScanActive   bool
	ScanTarget   string
	ScanProgress float64
	ScanMode     string

	Hailing    bool
	HailTarget string
	Frequency  float64

	TransporterActive    bool
	TransporterEmergency bool
}

// UpdateFromShip builds panel state from a ship plus server-owned station state.
func (psm *PanelStateManager) UpdateFromShip(panelID string, sh *ship.Ship, currentTime float64, station StationState) *PanelState {
	state := &PanelState{
		PanelID:    panelID,
		Indicators: make(map[string]Indicator),
		Displays:   make(map[string]Display),
		Timestamp:  currentTime,
	}

	// Serialize without the manager lock: updaters only read the ship snapshot
	// and the station state, so panels do not serialize against each other.
	if updater, ok := updaters[panelID]; ok {
		updater(state, sh, station)
	}

	psm.mu.Lock()
	psm.states[panelID] = state
	psm.mu.Unlock()

	return state
}

// UpdateFromShip builds panel state from a ship plus server-owned station state.
func updateRelayScanningPanel(state *PanelState, sh *ship.Ship, station StationState) {
	state.Indicators["scan_active"] = Indicator{
		Type:  "led",
		Value: station.ScanActive,
		Color: "blue",
		Blink: station.ScanActive,
	}

	state.Displays["scan_progress"] = Display{
		Type:   "numeric",
		Value:  station.ScanProgress * 100,
		Unit:   "%",
		Format: "%.0f",
	}

	state.Displays["scan_mode"] = Display{
		Type:   "text",
		Value:  station.ScanMode,
		Unit:   "",
		Format: "%s",
	}

	if station.ScanTarget != "" {
		state.Displays["scan_target"] = Display{
			Type:   "text",
			Value:  station.ScanTarget,
			Unit:   "",
			Format: "%s",
		}
	}
}

func updateFirstOfficerMainPanel(state *PanelState, sh *ship.Ship, station StationState) {
	for id, subsystem := range sh.Subsystems {
		healthPercent := (subsystem.Health / subsystem.MaxHealth) * 100

		color := "green"
		if healthPercent < 25 {
			color = "red"
		} else if healthPercent < 50 {
			color = "yellow"
		}

		state.Indicators["system_"+id] = Indicator{
			Type:  "led",
			Value: subsystem.Enabled,
			Color: color,
			Blink: subsystem.OnFire,
		}

		state.Displays["system_health_"+id] = Display{
			Type:   "numeric",
			Value:  healthPercent,
			Unit:   "%",
			Format: "%.0f",
		}
	}
}

func (psm *PanelStateManager) GetState(panelID string) *PanelState {
	psm.mu.RLock()
	defer psm.mu.RUnlock()

	return psm.states[panelID]
}

func (psm *PanelStateManager) GetAllStates() map[string]*PanelState {
	psm.mu.RLock()
	defer psm.mu.RUnlock()

	states := make(map[string]*PanelState)
	for k, v := range psm.states {
		states[k] = v
	}
	return states
}
