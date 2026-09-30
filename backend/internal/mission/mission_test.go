package mission

import (
	"celestial/internal/config"
	"celestial/internal/ship"
	"celestial/internal/simulation"
	"os"
	"path/filepath"
	"testing"
)

func testClasses() map[string]*config.ShipClass {
	return map[string]*config.ShipClass{
		"player_cruiser": {
			ID: "player_cruiser", Name: "Cruiser",
			Mass: 500000, MaxSpeed: 250, Acceleration: 50, TurnRate: 0.8,
			Engines: []config.EngineConfig{
				{ID: "main_1", Type: "main", Thrust: 100000, Health: 100, PowerDraw: 150},
			},
			Weapons: []config.WeaponConfig{},
			Shields: config.ShieldConfig{RechargeRate: 10, PowerDraw: 100,
				Emitters: []config.EmitterConfig{
					{ID: "forward", Facing: "forward", Strength: 500, Health: 100},
				}},
			Hull: config.HullConfig{Sections: []config.HullSectionConfig{
				{ID: "forward", Armor: 200, Health: 500},
				{ID: "aft", Armor: 200, Health: 500},
			}},
			Subsystems: []config.SubsystemConfig{},
			LaunchBays: []config.LaunchBayConfig{},
		},
		"enemy_frigate": {
			ID: "enemy_frigate", Name: "Frigate",
			Mass: 200000, MaxSpeed: 300, Acceleration: 60, TurnRate: 1.0,
			Engines: []config.EngineConfig{
				{ID: "main_1", Type: "main", Thrust: 60000, Health: 100, PowerDraw: 100},
			},
			Weapons: []config.WeaponConfig{},
			Shields: config.ShieldConfig{RechargeRate: 10, PowerDraw: 50,
				Emitters: []config.EmitterConfig{
					{ID: "forward", Facing: "forward", Strength: 300, Health: 100},
				}},
			Hull: config.HullConfig{Sections: []config.HullSectionConfig{
				{ID: "forward", Armor: 120, Health: 300},
				{ID: "aft", Armor: 120, Health: 300},
			}},
			Subsystems: []config.SubsystemConfig{},
			LaunchBays: []config.LaunchBayConfig{},
		},
	}
}

func newTestEngine(t *testing.T) (*Engine, *simulation.Simulator) {
	t.Helper()
	sim := simulation.NewSimulator(60, testClasses())

	engine := NewEngine(sim)
	sim.OnEvent = func(name string, data map[string]interface{}) {
		engine.TriggerEvent(name, data)
	}
	t.Cleanup(func() { engine.StopMission() })
	return engine, sim
}

func loadShippedMissions(t *testing.T, engine *Engine) {
	t.Helper()
	dir := filepath.Join("..", "..", "missions")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("missions directory not available: %v", err)
	}
	if err := engine.LoadMissions(dir); err != nil {
		t.Fatalf("LoadMissions: %v", err)
	}
}

func TestLoadShippedMissions(t *testing.T) {
	engine, _ := newTestEngine(t)
	loadShippedMissions(t, engine)

	missions := engine.GetMissions()
	if len(missions) == 0 {
		t.Fatal("No missions were loaded")
	}
	for _, id := range []string{"border_patrol", "rescue_operation"} {
		if _, ok := missions[id]; !ok {
			t.Errorf("Missing shipped mission %q", id)
		}
	}
}

func TestStartMissionReadsNameAndObjectives(t *testing.T) {
	engine, sim := newTestEngine(t)
	loadShippedMissions(t, engine)

	if err := engine.StartMission("rescue_operation"); err != nil {
		t.Fatalf("StartMission: %v", err)
	}

	active := engine.GetActiveMission()
	if active == nil {
		t.Fatal("Mission should be active")
	}
	if active.Name != "Rescue Operation" {
		t.Errorf("Mission name should come from the Lua table, got %q", active.Name)
	}
	if active.Description == "" {
		t.Error("Mission description should come from the Lua table")
	}
	if len(active.Objectives) == 0 {
		t.Error("on_start should set objectives")
	}

	if sim.GetShip("player_1") == nil {
		t.Error("on_start should spawn the player ship")
	}
}

func TestStartUnknownMissionErrors(t *testing.T) {
	engine, _ := newTestEngine(t)
	loadShippedMissions(t, engine)

	if err := engine.StartMission("does_not_exist"); err == nil {
		t.Error("Starting an unknown mission should error")
	}
}

func TestObjectivesUpdateThroughLifecycle(t *testing.T) {
	engine, _ := newTestEngine(t)
	loadShippedMissions(t, engine)

	if err := engine.StartMission("rescue_operation"); err != nil {
		t.Fatalf("StartMission: %v", err)
	}

	if objectiveDescription(engine, "protect") == "" {
		t.Fatal("protect objective should exist after on_start")
	}

	// The mission reacts to what the crew did, not to a kill counter.
	engine.TriggerEvent("crew_action", map[string]interface{}{"action": "hail"})

	desc := objectiveDescription(engine, "protect")
	if desc == "" || desc == "Keep the Aurora alive" {
		t.Errorf("The hail should change the protect objective, got %q", desc)
	}

	engine.TriggerEvent("crew_action", map[string]interface{}{"action": "dock"})
	if !objectiveCompleted(engine.GetActiveMission(), "respond") {
		t.Error("Docking should complete the respond objective")
	}
}

func objectiveDescription(engine *Engine, id string) string {
	for _, o := range engine.GetActiveMission().Objectives {
		if o.ID == id {
			return o.Description
		}
	}
	return ""
}

func objectiveCompleted(m *Mission, id string) bool {
	for _, o := range m.Objectives {
		if o.ID == id {
			return o.Completed
		}
	}
	return false
}

func TestMissionEventsAreBroadcast(t *testing.T) {
	engine, _ := newTestEngine(t)
	loadShippedMissions(t, engine)

	var names []string
	engine.OnEvent = func(name string, data map[string]interface{}) {
		names = append(names, name)
	}

	if err := engine.StartMission("rescue_operation"); err != nil {
		t.Fatalf("StartMission: %v", err)
	}
	names = nil

	engine.TriggerEvent("crew_action", map[string]interface{}{"action": "dock"})

	found := false
	for _, n := range names {
		if n == "mission_win" {
			found = true
		}
	}
	if !found {
		t.Errorf("A mission ending should broadcast mission_win, got %v", names)
	}
}

func TestTriggerEventWithoutMissionIsSafe(t *testing.T) {
	engine, _ := newTestEngine(t)
	loadShippedMissions(t, engine)

	// No active mission: must not panic.
	engine.TriggerEvent("ship_destroyed", map[string]interface{}{"ship_id": "ghost"})
}

func TestShipDestroyedEventFromSim(t *testing.T) {
	engine, sim := newTestEngine(t)
	loadShippedMissions(t, engine)

	if err := engine.StartMission("rescue_operation"); err != nil {
		t.Fatalf("StartMission: %v", err)
	}

	var outcomes []string
	engine.OnEvent = func(name string, data map[string]interface{}) {
		if name == "mission_lose" {
			outcomes = append(outcomes, name)
		}
	}

	// Destroy the player ship outright.
	player := sim.GetShip("player_1")
	for _, section := range []string{ship.SectionForward, ship.SectionAft} {
		player.RepairSection(section, -10000)
	}
	sim.Tick()

	if len(outcomes) == 0 {
		t.Error("Destroying the player ship should end the mission")
	}
}

func TestStopMission(t *testing.T) {
	engine, _ := newTestEngine(t)
	loadShippedMissions(t, engine)

	if err := engine.StartMission("border_patrol"); err != nil {
		t.Fatalf("StartMission: %v", err)
	}
	engine.StopMission()
	if engine.GetActiveMission() != nil {
		t.Error("StopMission should clear the active mission")
	}
}

func TestRestartResetsObjectives(t *testing.T) {
	engine, _ := newTestEngine(t)
	loadShippedMissions(t, engine)

	if err := engine.StartMission("rescue_operation"); err != nil {
		t.Fatalf("StartMission: %v", err)
	}
	first := len(engine.GetActiveMission().Objectives)
	if first == 0 {
		t.Fatal("First run should set objectives")
	}

	// Complete one, then restart the same mission.
	engine.TriggerEvent("crew_action", map[string]interface{}{"action": "dock"})
	if !objectiveCompleted(engine.GetActiveMission(), "respond") {
		t.Fatal("Dock should complete the respond objective")
	}

	if err := engine.StartMission("rescue_operation"); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if got := len(engine.GetActiveMission().Objectives); got != first {
		t.Errorf("Restart should reset objectives: %d, want %d", got, first)
	}
	if objectiveCompleted(engine.GetActiveMission(), "respond") {
		t.Error("Restart should clear completed objectives")
	}
}
