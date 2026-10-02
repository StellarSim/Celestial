package ship

import (
	"celestial/internal/config"
	"math"
	"testing"
)

func testClass() *config.ShipClass {
	return &config.ShipClass{
		ID:           "test_ship",
		Name:         "Test Ship",
		Mass:         100000,
		MaxSpeed:     200,
		Acceleration: 50,
		TurnRate:     1.0,
		Engines: []config.EngineConfig{
			{ID: "main_1", Type: "main", Thrust: 50000, Health: 100, PowerDraw: 100},
		},
		Weapons: []config.WeaponConfig{
			{ID: "phaser_1", Type: "phaser", Damage: 25, Range: 2000, CooldownTime: 2.0, Health: 100, PowerDraw: 50},
			{ID: "torpedo_bay_1", Type: "torpedo", Damage: 100, Range: 5000, CooldownTime: 5.0, Health: 100, PowerDraw: 20, AmmoCapacity: 10},
		},
		Shields: config.ShieldConfig{
			RechargeRate: 10,
			PowerDraw:    100,
			Emitters: []config.EmitterConfig{
				{ID: "forward", Facing: "forward", Strength: 500, Health: 100},
				{ID: "aft", Facing: "aft", Strength: 500, Health: 100},
				{ID: "port", Facing: "port", Strength: 400, Health: 100},
				{ID: "starboard", Facing: "starboard", Strength: 400, Health: 100},
			},
		},
		Hull: config.HullConfig{
			Sections: []config.HullSectionConfig{
				{ID: "forward", Armor: 200, Health: 500},
				{ID: "aft", Armor: 200, Health: 500},
				{ID: "port", Armor: 150, Health: 400},
				{ID: "starboard", Armor: 150, Health: 400},
			},
		},
		Subsystems: []config.SubsystemConfig{
			{ID: "sensors", Type: "sensors", Health: 100, PowerDraw: 30},
		},
		LaunchBays: []config.LaunchBayConfig{},
	}
}

func TestNewShip(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	if sh.ID != "ship_1" {
		t.Errorf("Expected ID ship_1, got %s", sh.ID)
	}
	if sh.ClassID != "test_ship" {
		t.Errorf("Expected ClassID test_ship, got %s", sh.ClassID)
	}
	if len(sh.Engines) != 1 {
		t.Errorf("Expected 1 engine, got %d", len(sh.Engines))
	}
	if len(sh.Weapons) != 2 {
		t.Errorf("Expected 2 weapons, got %d", len(sh.Weapons))
	}
	if !sh.IsPlayer {
		t.Error("Expected player ship")
	}
	if len(sh.Crew) != 8 {
		t.Errorf("Player ship should have 8 crew members, got %d", len(sh.Crew))
	}
}

func TestBreakersStartOn(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	if len(sh.Power.Breakers) != len(BreakerNames) {
		t.Fatalf("Expected %d breakers, got %d", len(BreakerNames), len(sh.Power.Breakers))
	}
	for _, name := range BreakerNames {
		if !sh.BreakerOn(name) {
			t.Errorf("Breaker %s should start closed (powered)", name)
		}
	}
}

func TestThrottleChangesVelocity(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	if v := sh.Velocity; v.X != 0 || v.Y != 0 || v.Z != 0 {
		t.Fatalf("Expected zero initial velocity, got %+v", v)
	}

	sh.SetThrottle(1.0)
	for i := 0; i < 60; i++ {
		sh.Update(1.0 / 60.0)
	}

	// Forward is local -Z, so full ahead drives negative Z.
	if sh.Velocity.Z >= 0 {
		t.Errorf("Expected negative Z velocity moving forward, got %.3f", sh.Velocity.Z)
	}
	if math.Abs(sh.Velocity.X) > 0.001 || math.Abs(sh.Velocity.Y) > 0.001 {
		t.Errorf("Expected no lateral velocity, got %+v", sh.Velocity)
	}
}

func TestSetLocalRotationRateReplacesRatherThanAccumulates(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	// Repeated identical commands, as a 10Hz helm poll would send, must not
	// build up spin: the old impulse version compounded into a full barrel roll.
	for i := 0; i < 20; i++ {
		sh.SetLocalRotationRate(0, 1.0, 0)
		sh.Update(1.0 / 60.0)
	}

	speed := math.Sqrt(
		sh.AngularVelocity.X*sh.AngularVelocity.X +
			sh.AngularVelocity.Y*sh.AngularVelocity.Y +
			sh.AngularVelocity.Z*sh.AngularVelocity.Z)
	if speed > sh.TurnRate {
		t.Errorf("Expected yaw rate near TurnRate %.2f, got %.2f", sh.TurnRate, speed)
	}
}

func TestSetLocalRotationRateZeroCoastsDownRatherThanStopping(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	// Wind up to a steady yaw.
	for i := 0; i < 240; i++ {
		sh.SetLocalRotationRate(0, 1.0, 0)
		sh.Update(1.0 / 60.0)
	}
	if math.Abs(sh.AngularVelocity.Y-sh.TurnRate) > 0.01 {
		t.Fatalf("Expected to settle at TurnRate %.2f, got %.2f", sh.TurnRate, sh.AngularVelocity.Y)
	}

	// Centring the stick must not zero the rate outright: the hull carries
	// inertia and coasts.
	sh.SetLocalRotationRate(0, 0, 0)
	sh.Update(1.0 / 60.0)
	if sh.AngularVelocity.Y <= 0 {
		t.Errorf("Expected the ship to keep coasting after the stick centres, got %+v", sh.AngularVelocity)
	}
	if sh.AngularVelocity.Y >= sh.TurnRate {
		t.Errorf("Expected the coasting rate to decay, got %.3f", sh.AngularVelocity.Y)
	}

	// And it should bleed off completely given time.
	for i := 0; i < 600; i++ {
		sh.Update(1.0 / 60.0)
	}
	if math.Abs(sh.AngularVelocity.Y) > 0.001 {
		t.Errorf("Expected the ship to stop rotating eventually, got %.4f", sh.AngularVelocity.Y)
	}
}

func TestSetLocalRotationRateSpinsUpGradually(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	sh.SetLocalRotationRate(0, 1.0, 0)
	sh.Update(1.0 / 60.0)

	// One tick of acceleration must be a fraction of the way to the commanded
	// rate, not the whole way. This is what stops the ship snapping to full
	// deflection the instant the stick moves.
	if sh.AngularVelocity.Y >= sh.TurnRate {
		t.Errorf("Expected a gradual spin up, got %.3f of %.2f in one tick", sh.AngularVelocity.Y, sh.TurnRate)
	}
	if sh.AngularVelocity.Y <= 0 {
		t.Errorf("Expected yaw to build angular velocity about the ship up axis, got %+v", sh.AngularVelocity)
	}
}

func TestSetLocalRotationRateRollsAboutOwnAxis(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	// Identity rotation: local roll is about -Z, which is the ship's forward.
	sh.SetLocalRotationRate(0, 0, 1.0)
	for i := 0; i < 240; i++ {
		sh.Update(1.0 / 60.0)
	}
	if sh.AngularVelocity.Z >= 0 {
		t.Errorf("Expected roll to turn about local -Z (forward), got %+v", sh.AngularVelocity)
	}
	if math.Abs(sh.AngularVelocity.X) > 0.001 || math.Abs(sh.AngularVelocity.Y) > 0.001 {
		t.Errorf("Expected roll to leave pitch and yaw untouched, got %+v", sh.AngularVelocity)
	}

	// Rolling must carry the up vector over, so the helm can recover from an
	// inverted attitude rather than being stuck on its back.
	if sh.Up().Y > 0 {
		t.Errorf("Expected sustained roll to carry the up vector past vertical, got %+v", sh.Up())
	}
}

func TestSetLocalRotationRateClampsRunawayDemand(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	sh.SetLocalRotationRate(50, 50, 50)
	if sh.TurnCommand.X != maxTurnDemand || sh.TurnCommand.Y != maxTurnDemand || sh.TurnCommand.Z != maxTurnDemand {
		t.Errorf("Expected demand clamped to %.1f per axis, got %+v", maxTurnDemand, sh.TurnCommand)
	}

	// A diagonal demand must not settle faster or slower per axis than a
	// single axis, since the step is scaled by the whole gap.
	for i := 0; i < 600; i++ {
		sh.Update(1.0 / 60.0)
	}
	speed := math.Sqrt(
		sh.AngularVelocity.X*sh.AngularVelocity.X +
			sh.AngularVelocity.Y*sh.AngularVelocity.Y +
			sh.AngularVelocity.Z*sh.AngularVelocity.Z)
	limit := maxTurnDemand * sh.TurnRate * math.Sqrt(3)
	if math.Abs(speed-limit) > 0.01 {
		t.Errorf("Expected to settle at the diagonal limit %.2f, got %.2f", limit, speed)
	}
}

func TestAsternReversesThrust(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	sh.SetThrottle(-1.0)
	for i := 0; i < 30; i++ {
		sh.Update(1.0 / 60.0)
	}
	if sh.Velocity.Z <= 0 {
		t.Errorf("Expected positive Z velocity moving astern, got %.3f", sh.Velocity.Z)
	}
}

func TestBreakerOffStopsEngines(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	sh.SetBreaker("engines", false)
	sh.SetThrottle(1.0)
	for i := 0; i < 60; i++ {
		sh.Update(1.0 / 60.0)
	}
	if math.Abs(sh.Velocity.Z) > 0.001 {
		t.Errorf("Engines breaker off should produce no thrust, got velocity %.3f", sh.Velocity.Z)
	}
}

func TestBreakerOffReducesPowerDraw(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	// One update to populate the baseline consumption.
	sh.Update(0.016)
	sh.mu.RLock()
	withDraw := sh.Power.Consumption
	sh.mu.RUnlock()

	sh.SetBreaker("weapons", false)
	sh.SetBreaker("shields", false)

	sh.Update(0.1)

	sh.mu.RLock()
	withoutDraw := sh.Power.Consumption
	sh.mu.RUnlock()

	if withDraw <= 0 {
		t.Fatalf("Expected a positive baseline consumption, got %.2f", withDraw)
	}
	if withoutDraw >= withDraw {
		t.Errorf("Expected lower consumption with weapons/shields off: %.2f -> %.2f", withDraw, withoutDraw)
	}
}

func TestBreakerOffDisablesWeapons(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	sh.MutateWeapon("torpedo_bay_1", func(w *Weapon) error {
		w.Armed = true
		w.Loaded = true
		w.Locked = true
		return nil
	})

	sh.SetBreaker("weapons", false)
	if sh.FireWeapon("torpedo_bay_1", "target_1") {
		t.Error("Weapons breaker off should prevent firing")
	}
}

func TestShieldsAbsorbBeforeHull(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), false)

	initialShields := sh.Shields.Emitters["forward"].Strength
	initialHull := sh.Hull.Sections["forward"].Health

	sh.TakeDamage(100, "forward")

	if sh.Shields.Emitters["forward"].Strength >= initialShields {
		t.Error("Shields should have absorbed the hit")
	}
	if sh.Hull.Sections["forward"].Health != initialHull {
		t.Error("Hull should be untouched while shields hold")
	}
}

func TestDamageFromBehindHitsAft(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), false)

	// A point behind the ship (forward is local -Z).
	behind := sh.Position
	behind.Z += 100
	if facing := sh.FacingFor(behind); facing != SectionAft {
		t.Fatalf("Expected aft facing, got %s", facing)
	}

	sh.SetShieldsEnabled(false)
	sh.SetBreaker("shields", false)
	aftHull := sh.Hull.Sections["aft"].Health
	forwardHull := sh.Hull.Sections["forward"].Health

	sh.ApplyTypedDamage(60, sh.FacingFor(behind), "kinetic")

	if sh.Hull.Sections["aft"].Health >= aftHull {
		t.Error("Aft hull should take damage from a hit astern")
	}
	if sh.Hull.Sections["forward"].Health != forwardHull {
		t.Error("Forward hull should be untouched by a hit astern")
	}
}

func TestDamageFromSideHitsStarboard(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), false)

	right := sh.Position
	right.X += 100
	if facing := sh.FacingFor(right); facing != SectionStarboard {
		t.Fatalf("Expected starboard facing, got %s", facing)
	}

	left := sh.Position
	left.X -= 100
	if facing := sh.FacingFor(left); facing != SectionPort {
		t.Fatalf("Expected port facing, got %s", facing)
	}
}

func TestFireDrainsOxygenWhenBurning(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	sh.StartFire("forward")
	sh.mu.RLock()
	section := sh.Hull.Sections["forward"]
	if !section.OnFire {
		sh.mu.RUnlock()
		t.Fatal("Expected forward section on fire")
	}
	comp := sh.LifeSupport.Compartments["bridge"]
	initialOxygen := comp.Oxygen
	sh.mu.RUnlock()

	for i := 0; i < 60; i++ {
		sh.Update(1.0 / 60.0)
	}

	sh.mu.RLock()
	afterOxygen := sh.LifeSupport.Compartments["bridge"].Oxygen
	sh.mu.RUnlock()

	if afterOxygen >= initialOxygen {
		t.Errorf("Fire should consume oxygen: %.2f -> %.2f", initialOxygen, afterOxygen)
	}
}

func TestBreachMarksCompartmentAndDrainsAir(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	sh.SetShieldsEnabled(false)
	sh.SetBreaker("shields", false)

	// Destroy the forward hull section outright.
	sh.RepairSection("forward", -10000)
	sh.mu.RLock()
	hullNow := sh.Hull.Sections["forward"].Health
	sh.mu.RUnlock()
	if hullNow > 0 {
		t.Fatalf("Forward section should be destroyed, health %.2f", hullNow)
	}

	sh.mu.RLock()
	section := sh.Hull.Sections["forward"]
	breached := section.Breached
	sh.mu.RUnlock()
	if !breached {
		t.Fatal("Forward section should be breached after taking heavy damage")
	}

	sh.mu.RLock()
	comp := sh.LifeSupport.Compartments["bridge"]
	if !comp.Breached {
		sh.mu.RUnlock()
		t.Fatal("A forward hull breach should breach the bridge compartment")
	}
	initialPressure := comp.Pressure
	sh.mu.RUnlock()

	for i := 0; i < 60; i++ {
		sh.Update(1.0 / 60.0)
	}

	sh.mu.RLock()
	afterPressure := sh.LifeSupport.Compartments["bridge"].Pressure
	sh.mu.RUnlock()

	if afterPressure >= initialPressure {
		t.Errorf("Breached compartment should lose pressure: %.2f -> %.2f", initialPressure, afterPressure)
	}
}

func TestExtinguishAndSealClearStates(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)

	sh.StartFire("aft")
	sh.RepairSection("aft", -10000)

	sh.ExtinguishFire("aft")
	sh.SealBreach("aft")

	sh.mu.RLock()
	section := sh.Hull.Sections["aft"]
	comp := sh.LifeSupport.Compartments["engineering"]
	sh.mu.RUnlock()

	if section.OnFire {
		t.Error("Extinguish should clear the fire")
	}
	if section.Breached {
		t.Error("Seal should clear the breach")
	}
	if comp.Breached {
		t.Error("Seal should clear the compartment breach")
	}
	if comp.Pressure != comp.MaxPressure {
		t.Errorf("Seal should restore pressure, got %.2f", comp.Pressure)
	}
}

func TestWeaponRangeIsHonoured(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), false)

	near := Vector3{X: 0, Y: 0, Z: -1000}
	far := Vector3{X: 0, Y: 0, Z: -3000}

	if !sh.InWeaponRange("phaser_1", near) {
		t.Error("Target within 2000m range should be in range")
	}
	if sh.InWeaponRange("phaser_1", far) {
		t.Error("Target beyond 2000m range should be out of range")
	}
}

func TestTorpedoFireGating(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), false)

	if sh.FireWeapon("torpedo_bay_1", "target_1") {
		t.Error("Torpedo should not fire unarmed")
	}

	sh.MutateWeapon("torpedo_bay_1", func(w *Weapon) error { w.Armed = true; return nil })
	if sh.FireWeapon("torpedo_bay_1", "target_1") {
		t.Error("Torpedo should not fire unloaded")
	}

	sh.MutateWeapon("torpedo_bay_1", func(w *Weapon) error { w.Loaded = true; return nil })
	if sh.FireWeapon("torpedo_bay_1", "target_1") {
		t.Error("Torpedo should not fire without a lock")
	}

	sh.MutateWeapon("torpedo_bay_1", func(w *Weapon) error { w.Locked = true; return nil })
	if !sh.FireWeapon("torpedo_bay_1", "target_1") {
		t.Fatal("Torpedo should fire once armed, loaded and locked")
	}
	if sh.FireWeapon("torpedo_bay_1", "target_1") {
		t.Error("Torpedo should be on cooldown after firing")
	}
}

func TestCloneIsDeepCopy(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), true)
	sh.SetThrottle(0.5)

	clone := sh.Clone()
	if clone.ID != sh.ID {
		t.Fatalf("Clone should keep identity, got %s", clone.ID)
	}
	if clone.Throttle != sh.Throttle {
		t.Errorf("Clone should copy throttle: %.2f vs %.2f", clone.Throttle, sh.Throttle)
	}

	clone.SetThrottle(1.0)
	clone.RepairSection("forward", -250)

	if sh.Throttle == 1.0 {
		t.Error("Mutating the clone must not affect the original")
	}
	if sh.Hull.Sections["forward"].Health == clone.Hull.Sections["forward"].Health {
		t.Error("Hull sections should be independently copied")
	}

	clone.SetBreaker("engines", false)
	if !sh.BreakerOn("engines") {
		t.Error("Breakers should be independently copied")
	}
}

func TestSetShieldsEnabled(t *testing.T) {
	sh := NewShip("ship_1", "test_ship", "Test Ship", testClass(), false)

	sh.SetShieldsEnabled(false)
	initialHull := sh.Hull.Sections["forward"].Health
	sh.TakeDamage(100, "forward")
	if sh.Hull.Sections["forward"].Health == initialHull {
		t.Error("Shields down should let damage reach the hull")
	}
}
