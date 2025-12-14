package panel

import (
	"celestial/internal/ship"
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

func (psm *PanelStateManager) UpdateFromShip(panelID string, sh *ship.Ship, currentTime float64) *PanelState {
	psm.mu.Lock()
	defer psm.mu.Unlock()

	state := &PanelState{
		PanelID:    panelID,
		Indicators: make(map[string]Indicator),
		Displays:   make(map[string]Display),
		Timestamp:  currentTime,
	}

	switch {
	case panelID == "engineer_power_main":
		psm.updateEngineerPowerPanel(state, sh)
	case panelID == "engineer_damage_main":
		psm.updateEngineerDamagePanel(state, sh)
	case panelID == "engineer_systems":
		psm.updateEngineerSystemsPanel(state, sh)
	case panelID == "flight_main":
		psm.updateFlightMainPanel(state, sh)
	case panelID == "flight_navigation":
		psm.updateFlightNavigationPanel(state, sh)
	case panelID == "weapons_torpedos_1":
		psm.updateWeaponsTorpedosPanel1(state, sh)
	case panelID == "weapons_torpedos_2":
		psm.updateWeaponsTorpedosPanel2(state, sh)
	case panelID == "weapons_phasers":
		psm.updateWeaponsPhasersPanel(state, sh)
	case panelID == "captain_command":
		psm.updateCaptainCommandPanel(state, sh)
	case panelID == "captain_status":
		psm.updateCaptainStatusPanel(state, sh)
	case panelID == "comms_main":
		psm.updateCommsMainPanel(state, sh)
	case panelID == "operations_power":
		psm.updateOperationsPowerPanel(state, sh)
	case panelID == "operations_resources":
		psm.updateOperationsResourcesPanel(state, sh)
	case panelID == "relay_sensors":
		psm.updateRelaySensorsPanel(state, sh)
	case panelID == "relay_scanning":
		psm.updateRelayScanningPanel(state, sh)
	case panelID == "first_officer_main":
		psm.updateFirstOfficerMainPanel(state, sh)
	}

	psm.states[panelID] = state
	return state
}

func (psm *PanelStateManager) updateEngineerPowerPanel(state *PanelState, sh *ship.Ship) {
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

	for id, breaker := range sh.Power.Breakers {
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

func (psm *PanelStateManager) updateEngineerDamagePanel(state *PanelState, sh *ship.Ship) {
	for id, section := range sh.Hull.Sections {
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

func (psm *PanelStateManager) updateEngineerSystemsPanel(state *PanelState, sh *ship.Ship) {
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

func (psm *PanelStateManager) updateFlightMainPanel(state *PanelState, sh *ship.Ship) {
	state.Displays["velocity_x"] = Display{
		Type:   "numeric",
		Value:  sh.Velocity.X,
		Unit:   "m/s",
		Format: "%.1f",
	}

	state.Displays["velocity_y"] = Display{
		Type:   "numeric",
		Value:  sh.Velocity.Y,
		Unit:   "m/s",
		Format: "%.1f",
	}

	state.Displays["velocity_z"] = Display{
		Type:   "numeric",
		Value:  sh.Velocity.Z,
		Unit:   "m/s",
		Format: "%.1f",
	}

	speed := (sh.Velocity.X*sh.Velocity.X + sh.Velocity.Y*sh.Velocity.Y + sh.Velocity.Z*sh.Velocity.Z)
	if speed > 0 {
		speed = speed * 0.5
	}

	state.Displays["speed"] = Display{
		Type:   "numeric",
		Value:  speed,
		Unit:   "m/s",
		Format: "%.0f",
	}

	state.Indicators["docked"] = Indicator{
		Type:  "led",
		Value: sh.Docked,
		Color: "blue",
		Blink: false,
	}
}

func (psm *PanelStateManager) updateFlightNavigationPanel(state *PanelState, sh *ship.Ship) {
	state.Displays["position_x"] = Display{
		Type:   "numeric",
		Value:  sh.Position.X,
		Unit:   "km",
		Format: "%.0f",
	}

	state.Displays["position_y"] = Display{
		Type:   "numeric",
		Value:  sh.Position.Y,
		Unit:   "km",
		Format: "%.0f",
	}

	state.Displays["position_z"] = Display{
		Type:   "numeric",
		Value:  sh.Position.Z,
		Unit:   "km",
		Format: "%.0f",
	}

	state.Displays["heading"] = Display{
		Type:   "numeric",
		Value:  0.0,
		Unit:   "°",
		Format: "%.1f",
	}
}

func (psm *PanelStateManager) updateWeaponsTorpedosPanel1(state *PanelState, sh *ship.Ship) {
	psm.updateTorpedoBay(state, sh, "torpedo_bay_1")
	psm.updateTorpedoBay(state, sh, "torpedo_bay_2")
}

func (psm *PanelStateManager) updateWeaponsTorpedosPanel2(state *PanelState, sh *ship.Ship) {
	psm.updateTorpedoBay(state, sh, "torpedo_bay_3")
	psm.updateTorpedoBay(state, sh, "torpedo_bay_4")
}

func (psm *PanelStateManager) updateTorpedoBay(state *PanelState, sh *ship.Ship, bayID string) {
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

	state.Indicators[bayID+"_locked"] = Indicator{
		Type:  "led",
		Value: weapon.Locked,
		Color: "red",
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

func (psm *PanelStateManager) updateWeaponsPhasersPanel(state *PanelState, sh *ship.Ship) {
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

func (psm *PanelStateManager) updateCaptainCommandPanel(state *PanelState, sh *ship.Ship) {
	state.Indicators["red_alert"] = Indicator{
		Type:  "led",
		Value: false,
		Color: "red",
		Blink: false,
	}

	state.Indicators["yellow_alert"] = Indicator{
		Type:  "led",
		Value: true,
		Color: "yellow",
		Blink: false,
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

func (psm *PanelStateManager) updateCaptainStatusPanel(state *PanelState, sh *ship.Ship) {
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

func (psm *PanelStateManager) updateCommsMainPanel(state *PanelState, sh *ship.Ship) {
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
			Value: comms.Enabled && healthPercent > 0,
			Color: color,
			Blink: false,
		}
	}
}

func (psm *PanelStateManager) updateOperationsPowerPanel(state *PanelState, sh *ship.Ship) {
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

func (psm *PanelStateManager) updateOperationsResourcesPanel(state *PanelState, sh *ship.Ship) {
	for id, bay := range sh.LaunchBays {
		state.Displays["bay_"+id+"_count"] = Display{
			Type:   "numeric",
			Value:  bay.Current,
			Unit:   "/" + string(rune(bay.Capacity)),
			Format: "%d",
		}
	}
}

func (psm *PanelStateManager) updateRelaySensorsPanel(state *PanelState, sh *ship.Ship) {
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
			Value: sensors.Enabled && healthPercent > 0,
			Color: color,
			Blink: false,
		}
	}
}

func (psm *PanelStateManager) updateRelayScanningPanel(state *PanelState, sh *ship.Ship) {
	state.Indicators["scan_active"] = Indicator{
		Type:  "led",
		Value: false,
		Color: "blue",
		Blink: false,
	}
}

func (psm *PanelStateManager) updateFirstOfficerMainPanel(state *PanelState, sh *ship.Ship) {
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
