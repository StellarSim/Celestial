package simulation

import (
	"celestial/internal/config"
	"celestial/internal/ship"
	"testing"
	"time"
)

func TestSimulatorCreation(t *testing.T) {
	classes := make(map[string]*config.ShipClass)
	sim := NewSimulator(60, classes)

	if sim.tickRate != 60 {
		t.Errorf("Expected tick rate 60, got %d", sim.tickRate)
	}

	if sim.Ships == nil {
		t.Error("Ships map should be initialized")
	}
}

func TestSpawnShip(t *testing.T) {
	classes := map[string]*config.ShipClass{
		"test_ship": {
			ID:           "test_ship",
			Name:         "Test Ship",
			Mass:         100000,
			MaxSpeed:     200,
			Acceleration: 50,
			TurnRate:     1.0,
			Engines:      []config.EngineConfig{},
			Weapons:      []config.WeaponConfig{},
			Shields:      config.ShieldConfig{Emitters: []config.EmitterConfig{}},
			Hull:         config.HullConfig{Sections: []config.HullSectionConfig{}},
			Subsystems:   []config.SubsystemConfig{},
			LaunchBays:   []config.LaunchBayConfig{},
		},
	}

	sim := NewSimulator(60, classes)

	pos := ship.Vector3{X: 100, Y: 200, Z: 300}
	err := sim.SpawnShip("ship_1", "test_ship", "Test Ship", false, pos)

	if err != nil {
		t.Errorf("Failed to spawn ship: %v", err)
	}

	sh := sim.GetShip("ship_1")
	if sh == nil {
		t.Error("Ship should exist after spawning")
	}

	if sh.Position.X != 100 || sh.Position.Y != 200 || sh.Position.Z != 300 {
		t.Error("Ship position not set correctly")
	}
}

func TestRemoveShip(t *testing.T) {
	classes := map[string]*config.ShipClass{
		"test_ship": {
			ID:           "test_ship",
			Name:         "Test Ship",
			Mass:         100000,
			MaxSpeed:     200,
			Acceleration: 50,
			TurnRate:     1.0,
			Engines:      []config.EngineConfig{},
			Weapons:      []config.WeaponConfig{},
			Shields:      config.ShieldConfig{Emitters: []config.EmitterConfig{}},
			Hull:         config.HullConfig{Sections: []config.HullSectionConfig{}},
			Subsystems:   []config.SubsystemConfig{},
			LaunchBays:   []config.LaunchBayConfig{},
		},
	}

	sim := NewSimulator(60, classes)
	pos := ship.Vector3{X: 0, Y: 0, Z: 0}
	sim.SpawnShip("ship_1", "test_ship", "Test Ship", false, pos)

	sim.RemoveShip("ship_1")

	sh := sim.GetShip("ship_1")
	if sh != nil {
		t.Error("Ship should not exist after removal")
	}
}

func TestSnapshotCreation(t *testing.T) {
	classes := make(map[string]*config.ShipClass)
	sim := NewSimulator(60, classes)

	initialCount := len(sim.Snapshots)
	sim.CreateSnapshot()

	if len(sim.Snapshots) != initialCount+1 {
		t.Error("Snapshot count should increase")
	}
}

func TestSnapshotRestore(t *testing.T) {
	classes := map[string]*config.ShipClass{
		"test_ship": {
			ID:           "test_ship",
			Name:         "Test Ship",
			Mass:         100000,
			MaxSpeed:     200,
			Acceleration: 50,
			TurnRate:     1.0,
			Engines:      []config.EngineConfig{},
			Weapons:      []config.WeaponConfig{},
			Shields:      config.ShieldConfig{Emitters: []config.EmitterConfig{}},
			Hull:         config.HullConfig{Sections: []config.HullSectionConfig{}},
			Subsystems:   []config.SubsystemConfig{},
			LaunchBays:   []config.LaunchBayConfig{},
		},
	}

	sim := NewSimulator(60, classes)

	pos := ship.Vector3{X: 100, Y: 0, Z: 0}
	sim.SpawnShip("ship_1", "test_ship", "Test Ship", false, pos)
	sim.CreateSnapshot()

	sim.RemoveShip("ship_1")
	if sim.GetShip("ship_1") != nil {
		t.Error("Ship should be removed")
	}

	sim.RestoreSnapshot(0)
	if sim.GetShip("ship_1") == nil {
		t.Error("Ship should be restored from snapshot")
	}
}

func TestPauseResume(t *testing.T) {
	classes := make(map[string]*config.ShipClass)
	sim := NewSimulator(60, classes)

	go sim.Start()
	time.Sleep(50 * time.Millisecond)

	sim.Pause()
	time.Sleep(50 * time.Millisecond)

	sim.Resume()
	time.Sleep(50 * time.Millisecond)

	sim.Stop()
	time.Sleep(50 * time.Millisecond)
}
