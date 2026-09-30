package simulation

import (
	"celestial/internal/config"
	"celestial/internal/ship"
	"math"
	"testing"
	"time"
)

func testClasses() map[string]*config.ShipClass {
	return map[string]*config.ShipClass{
		"cruiser": {
			ID: "cruiser", Name: "Cruiser",
			Mass: 500000, MaxSpeed: 250, Acceleration: 50, TurnRate: 0.8,
			Engines: []config.EngineConfig{
				{ID: "main_1", Type: "main", Thrust: 100000, Health: 100, PowerDraw: 150},
				{ID: "main_2", Type: "main", Thrust: 100000, Health: 100, PowerDraw: 150},
			},
			Weapons: []config.WeaponConfig{
				{ID: "phaser_1", Type: "phaser", Damage: 25, Range: 2000, CooldownTime: 2, Health: 100, PowerDraw: 50},
				{ID: "torpedo_bay_1", Type: "torpedo", Damage: 100, Range: 5000, CooldownTime: 5, Health: 100, PowerDraw: 20, AmmoCapacity: 20},
			},
			Shields: config.ShieldConfig{
				RechargeRate: 10, PowerDraw: 100,
				Emitters: []config.EmitterConfig{
					{ID: "forward", Facing: "forward", Strength: 500, Health: 100},
					{ID: "aft", Facing: "aft", Strength: 500, Health: 100},
					{ID: "port", Facing: "port", Strength: 400, Health: 100},
					{ID: "starboard", Facing: "starboard", Strength: 400, Health: 100},
				},
			},
			Hull: config.HullConfig{Sections: []config.HullSectionConfig{
				{ID: "forward", Armor: 200, Health: 500},
				{ID: "aft", Armor: 200, Health: 500},
				{ID: "port", Armor: 150, Health: 400},
				{ID: "starboard", Armor: 150, Health: 400},
			}},
			Subsystems: []config.SubsystemConfig{
				{ID: "sensors", Type: "sensors", Health: 100, PowerDraw: 30},
			},
			LaunchBays: []config.LaunchBayConfig{},
		},
	}
}

func newTestSim() *Simulator {
	return NewSimulator(60, testClasses())
}

func TestSimulatorCreation(t *testing.T) {
	sim := NewSimulator(60, make(map[string]*config.ShipClass))

	if sim.tickRate != 60 {
		t.Errorf("Expected tick rate 60, got %d", sim.tickRate)
	}
	if sim.Ships == nil {
		t.Error("Ships map should be initialized")
	}
	if sim.GetAlertLevel() != "normal" {
		t.Errorf("Expected default alert level normal, got %s", sim.GetAlertLevel())
	}
}

func TestSpawnShip(t *testing.T) {
	sim := newTestSim()

	pos := ship.Vector3{X: 100, Y: 200, Z: 300}
	if err := sim.SpawnShip("ship_1", "cruiser", "Test Ship", false, pos); err != nil {
		t.Fatalf("Failed to spawn ship: %v", err)
	}

	sh := sim.GetShip("ship_1")
	if sh == nil {
		t.Fatal("Ship should exist after spawning")
	}
	if sh.Position.X != 100 || sh.Position.Y != 200 || sh.Position.Z != 300 {
		t.Errorf("Ship position not set correctly: %+v", sh.Position)
	}
	if sim.AIControllers["ship_1"] == nil {
		t.Error("Non-player ships should get an AI controller")
	}
}

func TestSpawnUnknownClassIsAnError(t *testing.T) {
	sim := newTestSim()

	if err := sim.SpawnShip("x", "nope", "Nope", false, ship.Vector3{}); err == nil {
		t.Error("Spawning an unknown class should return an error")
	}
}

func TestRemoveShip(t *testing.T) {
	sim := newTestSim()
	sim.SpawnShip("ship_1", "cruiser", "Test Ship", false, ship.Vector3{})

	sim.RemoveShip("ship_1")

	if sim.GetShip("ship_1") != nil {
		t.Error("Ship should not exist after removal")
	}
	if _, ok := sim.AIControllers["ship_1"]; ok {
		t.Error("AI controller should be removed with the ship")
	}
}

func TestThrottleMovesShip(t *testing.T) {
	sim := newTestSim()
	sim.SpawnShip("player", "cruiser", "Endeavour", true, ship.Vector3{})
	player := sim.GetShip("player")

	player.SetThrottle(1.0)
	start := player.GetPosition()
	for i := 0; i < 120; i++ {
		sim.Tick()
	}

	end := player.GetPosition()
	moved := math.Sqrt(
		(end.X-start.X)*(end.X-start.X) +
			(end.Y-start.Y)*(end.Y-start.Y) +
			(end.Z-start.Z)*(end.Z-start.Z))
	if moved < 1 {
		t.Errorf("Full throttle should move the ship, moved %.3f m", moved)
	}
	if math.Abs(end.X-start.X) > 0.001 || math.Abs(end.Y-start.Y) > 0.001 {
		t.Errorf("Motion should be along the ship's forward axis: %+v -> %+v", start, end)
	}
}

func TestDistanceIsLinear(t *testing.T) {
	a := ship.Vector3{X: 0, Y: 0, Z: 0}
	b := ship.Vector3{X: 3, Y: 4, Z: 0}

	if got := distance(a, b); math.Abs(got-5.0) > 0.0001 {
		t.Errorf("Expected 5, got %.4f", got)
	}
}

func TestProjectileHitsTargetInRange(t *testing.T) {
	sim := newTestSim()
	sim.SpawnShip("player", "cruiser", "Endeavour", true, ship.Vector3{})
	sim.SpawnShip("raider", "cruiser", "Raider", false, ship.Vector3{X: 400, Y: 0, Z: 0})

	target := sim.GetShip("raider")
	target.SetShieldsEnabled(false)
	target.SetBreaker("shields", false)

	before := target.HullSnapshot()["forward"].Health

	sim.SpawnProjectile("torp_1", "torpedo", "player", "raider",
		ship.Vector3{X: 380, Y: 0, Z: 0},
		ship.Vector3{X: 400, Y: 0, Z: 0},
		80)

	for i := 0; i < 10; i++ {
		sim.Tick()
	}

	after := target.HullSnapshot()["forward"].Health
	if after >= before {
		t.Errorf("Projectile within range should damage the target: %.2f -> %.2f", before, after)
	}
	if len(sim.GetAllProjectiles()) != 0 {
		t.Error("Projectile should be consumed on impact")
	}
}

func TestProjectileOutOfRangePassesThrough(t *testing.T) {
	sim := newTestSim()
	sim.SpawnShip("player", "cruiser", "Endeavour", true, ship.Vector3{})
	sim.SpawnShip("raider", "cruiser", "Raider", false, ship.Vector3{X: 5000, Y: 0, Z: 0})

	target := sim.GetShip("raider")
	target.SetShieldsEnabled(false)
	target.SetBreaker("shields", false)

	before := target.HullSnapshot()["forward"].Health

	sim.SpawnProjectile("torp_1", "torpedo", "player", "raider",
		ship.Vector3{X: -5000, Y: 0, Z: 0},
		ship.Vector3{X: 100, Y: 0, Z: 0},
		80)

	for i := 0; i < 30; i++ {
		sim.Tick()
	}

	after := target.HullSnapshot()["forward"].Health
	if after != before {
		t.Errorf("A projectile that never gets close should not damage the target: %.2f -> %.2f", before, after)
	}
}

func TestHitFromBehindDamagesAft(t *testing.T) {
	sim := newTestSim()
	sim.SpawnShip("player", "cruiser", "Endeavour", true, ship.Vector3{})
	sim.SpawnShip("raider", "cruiser", "Raider", false, ship.Vector3{X: 400, Y: 0, Z: 0})

	target := sim.GetShip("raider")
	target.SetShieldsEnabled(false)
	target.SetBreaker("shields", false)

	hull := target.HullSnapshot()
	aftBefore := hull[ship.SectionAft].Health
	forwardBefore := hull[ship.SectionForward].Health

	// The raider's forward is local -Z, so a projectile closing from +Z hits
	// its stern.
	sim.SpawnProjectile("torp_1", "torpedo", "player", "raider",
		ship.Vector3{X: 400, Y: 0, Z: 40},
		ship.Vector3{X: 0, Y: 0, Z: -400},
		80)

	for i := 0; i < 10; i++ {
		sim.Tick()
	}

	after := target.HullSnapshot()
	if after[ship.SectionAft].Health >= aftBefore {
		t.Errorf("A hit from astern should damage the aft section: %.2f -> %.2f",
			aftBefore, after[ship.SectionAft].Health)
	}
	if after[ship.SectionForward].Health != forwardBefore {
		t.Errorf("Forward section should be untouched by a hit astern")
	}
}

func TestCollisionSeparatesAndDamagesOnce(t *testing.T) {
	sim := newTestSim()
	sim.SpawnShip("a", "cruiser", "A", true, ship.Vector3{})
	sim.SpawnShip("b", "cruiser", "B", false, ship.Vector3{X: 20, Y: 0, Z: 0})

	a := sim.GetShip("a")
	b := sim.GetShip("b")
	a.SetShieldsEnabled(false)
	b.SetShieldsEnabled(false)
	a.SetBreaker("shields", false)
	b.SetBreaker("shields", false)

	for i := 0; i < 30; i++ {
		sim.Tick()
	}

	sep := math.Sqrt(
		(b.Position.X-a.Position.X)*(b.Position.X-a.Position.X) +
			(b.Position.Y-a.Position.Y)*(b.Position.Y-a.Position.Y) +
			(b.Position.Z-a.Position.Z)*(b.Position.Z-a.Position.Z))
	if sep < collisionRadius {
		t.Errorf("Colliding ships should be pushed apart, separation %.2f", sep)
	}

	for id, sh := range map[string]*ship.Ship{"a": a, "b": b} {
		damaged := false
		for _, s := range sh.HullSnapshot() {
			if s.Health < s.MaxHealth {
				damaged = true
				break
			}
		}
		if !damaged {
			t.Errorf("Ship %s took no collision damage", id)
		}
	}
}

func TestSnapshotRestoreRevertsState(t *testing.T) {
	sim := newTestSim()
	sim.SpawnShip("player", "cruiser", "Endeavour", true, ship.Vector3{})
	player := sim.GetShip("player")
	player.SetThrottle(0.75)

	sim.CreateSnapshot()
	sim.Tick()

	player.SetThrottle(0.0)
	player.RepairSection(ship.SectionForward, -400)
	damaged := player.HullSnapshot()[ship.SectionForward].Health

	if err := sim.RestoreSnapshot(0); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	restored := sim.GetShip("player")
	if restored.HullSnapshot()[ship.SectionForward].Health <= damaged {
		t.Error("Restoring a snapshot should bring hull damage back")
	}
	if restored.Throttle != 0.75 {
		t.Errorf("Restoring a snapshot should bring throttle back, got %.2f", restored.Throttle)
	}
}

func TestRestoreInvalidIndexErrors(t *testing.T) {
	sim := newTestSim()
	if err := sim.RestoreSnapshot(5); err == nil {
		t.Error("Restoring a nonexistent snapshot should return an error")
	}
	sim.CreateSnapshot()
	if err := sim.RestoreSnapshot(-1); err == nil {
		t.Error("Restoring a negative index should return an error")
	}
}

func TestSnapshotDoesNotAliasLiveState(t *testing.T) {
	sim := newTestSim()
	sim.SpawnShip("player", "cruiser", "Endeavour", true, ship.Vector3{})
	player := sim.GetShip("player")

	sim.CreateSnapshot()

	// Mutating the live ship must not reach into the captured snapshot.
	player.RepairSection(ship.SectionForward, -100)
	player.SetThrottle(1.0)

	sim.mu.RLock()
	captured := sim.Snapshots[0].Ships["player"]
	sim.mu.RUnlock()

	if captured.HullSnapshot()[ship.SectionForward].Health != 500.0 {
		t.Errorf("Snapshot hull should be untouched, got %.2f",
			captured.HullSnapshot()[ship.SectionForward].Health)
	}
	if captured.Throttle != 0 {
		t.Errorf("Snapshot throttle should be untouched, got %.2f", captured.Throttle)
	}
}

func TestPauseResume(t *testing.T) {
	sim := newTestSim()

	go sim.Start()
	time.Sleep(50 * time.Millisecond)

	sim.Pause()
	time.Sleep(50 * time.Millisecond)
	if !sim.IsPaused() {
		t.Error("Simulator should report paused")
	}

	sim.Resume()
	time.Sleep(50 * time.Millisecond)
	if sim.IsPaused() {
		t.Error("Simulator should report running after resume")
	}

	sim.Stop()
	time.Sleep(50 * time.Millisecond)
}

func TestAITorpedoSpawnsProjectile(t *testing.T) {
	sim := newTestSim()
	sim.SpawnShip("player", "cruiser", "Endeavour", true, ship.Vector3{})
	sim.SpawnShip("raider", "cruiser", "Raider", false, ship.Vector3{X: 800, Y: 0, Z: 0})

	raider := sim.GetShip("raider")
	raider.MutateWeapon("torpedo_bay_1", func(w *ship.Weapon) error {
		w.Armed = true
		w.Loaded = true
		w.Locked = true
		return nil
	})

	controller := sim.AIControllers["raider"]
	controller.TargetID = "player"
	controller.State = "combat"

	fired := false
	for i := 0; i < 600 && !fired; i++ {
		sim.Tick()
		if len(sim.GetAllProjectiles()) > 0 {
			fired = true
		}
	}

	if !fired {
		t.Error("An AI ship in combat should launch a real torpedo projectile")
	}
}

func TestAlertLevelRoundTrip(t *testing.T) {
	sim := newTestSim()
	sim.SetAlertLevel("red")
	if sim.GetAlertLevel() != "red" {
		t.Errorf("Expected red alert, got %s", sim.GetAlertLevel())
	}
}

func TestOnTickHookRuns(t *testing.T) {
	sim := newTestSim()
	called := 0
	sim.OnTick = func(dt float64) { called++ }

	sim.Tick()
	if called != 1 {
		t.Errorf("Expected OnTick to be called once, got %d", called)
	}
}
