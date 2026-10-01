package input

import (
	"celestial/internal/config"
	"celestial/internal/ship"
	"celestial/internal/simulation"
	"testing"
)

func testClasses() map[string]*config.ShipClass {
	return map[string]*config.ShipClass{
		"cruiser": {
			ID: "cruiser", Name: "Cruiser",
			Mass: 500000, MaxSpeed: 250, Acceleration: 50, TurnRate: 0.8,
			Engines: []config.EngineConfig{
				{ID: "main_1", Type: "main", Thrust: 100000, Health: 100, PowerDraw: 150},
			},
			Weapons: []config.WeaponConfig{
				{ID: "phaser_1", Type: "phaser", Damage: 25, Range: 2000, CooldownTime: 2, Health: 100, PowerDraw: 50},
				{ID: "torpedo_bay_1", Type: "torpedo", Damage: 100, Range: 5000, CooldownTime: 5, Health: 100, PowerDraw: 20, AmmoCapacity: 4},
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
				{ID: "comms", Type: "communications", Health: 100, PowerDraw: 20},
			},
			LaunchBays: []config.LaunchBayConfig{},
		},
	}
}

func newTestRouter(t *testing.T) (*ActionRouter, *simulation.Simulator, *ship.Ship) {
	t.Helper()
	sim := simulation.NewSimulator(60, testClasses())
	if err := sim.SpawnShip("player", "cruiser", "Endeavour", true, ship.Vector3{}); err != nil {
		t.Fatalf("spawn player: %v", err)
	}
	if err := sim.SpawnShip("raider", "cruiser", "Raider", false, ship.Vector3{X: 500, Y: 0, Z: 0}); err != nil {
		t.Fatalf("spawn raider: %v", err)
	}
	return NewActionRouter(sim), sim, sim.GetShip("player")
}

func route(t *testing.T, r *ActionRouter, role, system, action string, value interface{}) error {
	t.Helper()
	return r.RouteAction(&Action{Role: role, System: system, Action: action, Value: value})
}

func TestUnknownActionErrors(t *testing.T) {
	r, _, _ := newTestRouter(t)

	if err := route(t, r, "engineer", "power", "explode", nil); err == nil {
		t.Error("Unknown action should return an error")
	}
	if err := route(t, r, "nobody", "power", "toggle_breaker", map[string]interface{}{"breaker": "engines"}); err == nil {
		t.Error("Unknown role should return an error")
	}
	if err := route(t, r, "engineer", "", "", nil); err == nil {
		t.Error("Missing system/action should return an error")
	}
}

func TestCatalogHasNoRoleGaps(t *testing.T) {
	r, _, _ := newTestRouter(t)
	if len(r.CatalogKeys()) == 0 {
		t.Error("Catalog should not be empty")
	}
}

func TestToggleBreakerFromValue(t *testing.T) {
	r, _, player := newTestRouter(t)

	err := route(t, r, "engineer", "power", "toggle_breaker", map[string]interface{}{
		"breaker": "engines",
		"enabled": false,
	})
	if err != nil {
		t.Fatalf("toggle_breaker failed: %v", err)
	}
	if player.BreakerOn("engines") {
		t.Error("Breaker should be open after the action")
	}

	// A bare panel press toggles.
	if err := route(t, r, "engineer", "power", "toggle_breaker", map[string]interface{}{
		"breaker": "engines",
	}); err != nil {
		t.Fatalf("toggle_breaker failed: %v", err)
	}
	if !player.BreakerOn("engines") {
		t.Error("Breaker should close again after a second press")
	}

	if err := route(t, r, "engineer", "power", "toggle_breaker", map[string]interface{}{}); err == nil {
		t.Error("toggle_breaker without a breaker name should error")
	}
}

func TestSetThrottle(t *testing.T) {
	r, _, player := newTestRouter(t)

	if err := route(t, r, "flight", "flight", "set_throttle", map[string]interface{}{"throttle": 0.75}); err != nil {
		t.Fatalf("set_throttle failed: %v", err)
	}
	if player.Throttle != 0.75 {
		t.Errorf("Expected throttle 0.75, got %.2f", player.Throttle)
	}

	if err := route(t, r, "flight", "flight", "set_throttle", map[string]interface{}{"throttle": 5}); err != nil {
		t.Fatalf("Throttle should clamp rather than error: %v", err)
	}
	if player.Throttle != 1 {
		t.Errorf("Throttle should clamp to 1, got %.2f", player.Throttle)
	}
}

func TestRepairExtinguishSealFromSection(t *testing.T) {
	r, _, player := newTestRouter(t)

	player.StartFire(ship.SectionForward)
	player.RepairSection(ship.SectionForward, -5000)

	if err := route(t, r, "engineer", "damage", "extinguish", map[string]interface{}{"section": "forward"}); err != nil {
		t.Fatalf("extinguish failed: %v", err)
	}
	if player.HullSnapshot()[ship.SectionForward].OnFire {
		t.Error("Extinguish should clear the fire")
	}

	if err := route(t, r, "engineer", "damage", "seal_breach", map[string]interface{}{"section": "aft"}); err != nil {
		t.Fatalf("seal_breach failed: %v", err)
	}

	player.RepairSection(ship.SectionPort, -100)
	before := player.HullSnapshot()[ship.SectionPort].Health
	if err := route(t, r, "engineer", "damage", "repair", map[string]interface{}{"section": "port", "amount": 25}); err != nil {
		t.Fatalf("repair failed: %v", err)
	}
	if player.HullSnapshot()[ship.SectionPort].Health != before+25 {
		t.Errorf("Repair should raise hull health by 25: %.2f -> %.2f",
			before, player.HullSnapshot()[ship.SectionPort].Health)
	}

	if err := route(t, r, "engineer", "damage", "repair", map[string]interface{}{"section": "nose"}); err == nil {
		t.Error("Unknown section should error")
	}
}

func TestSectionAliasesAreAccepted(t *testing.T) {
	r, _, _ := newTestRouter(t)

	for _, section := range []string{"bow", "stern", "forward", "aft"} {
		if err := route(t, r, "engineer", "damage", "repair", map[string]interface{}{"section": section}); err != nil {
			t.Errorf("Section alias %q should be accepted: %v", section, err)
		}
	}
}

func TestSetTargetValidatesExistence(t *testing.T) {
	r, _, player := newTestRouter(t)

	if err := route(t, r, "weapons", "weapons", "set_target", map[string]interface{}{"target_id": "raider"}); err != nil {
		t.Fatalf("set_target failed: %v", err)
	}
	if player.TargetID != "raider" {
		t.Errorf("Expected target raider, got %s", player.TargetID)
	}

	if err := route(t, r, "weapons", "weapons", "set_target", map[string]interface{}{"target_id": "ghost"}); err == nil {
		t.Error("Setting a nonexistent target should error")
	}

	if err := route(t, r, "weapons", "weapons", "clear_target", nil); err != nil {
		t.Fatalf("clear_target failed: %v", err)
	}
	if player.TargetID != "" {
		t.Error("clear_target should empty the target")
	}
}

func TestTorpedoFireHonoursRange(t *testing.T) {
	r, sim, player := newTestRouter(t)

	route(t, r, "weapons", "weapons", "set_target", map[string]interface{}{"target_id": "raider"})
	route(t, r, "weapons", "torpedo", "arm", map[string]interface{}{"bay_id": 1})
	route(t, r, "weapons", "torpedo", "load", map[string]interface{}{"bay_id": 1})
	// Targeting auto-locks ready tubes; force an unlocked state to verify
	// the fire gate still rejects unlocked bays.
	route(t, r, "weapons", "torpedo", "lock", map[string]interface{}{"bay_id": 1, "locked": false})

	// Unlocked: firing should fail.
	if err := route(t, r, "weapons", "torpedo", "fire", map[string]interface{}{"bay_id": 1}); err == nil {
		t.Error("Firing without a lock should fail")
	}

	route(t, r, "weapons", "torpedo", "lock", map[string]interface{}{"bay_id": 1})

	// Move the raider beyond the torpedo range of 5000.
	raider := sim.GetShip("raider")
	raider.Position.X = 9000

	if err := route(t, r, "weapons", "torpedo", "fire", map[string]interface{}{"bay_id": 1}); err == nil {
		t.Error("Firing past weapon range should fail")
	}

	raider.Position.X = 500
	if err := route(t, r, "weapons", "torpedo", "fire", map[string]interface{}{"bay_id": 1}); err != nil {
		t.Fatalf("Firing in range should succeed: %v", err)
	}
	if len(sim.GetAllProjectiles()) == 0 {
		t.Error("Firing a torpedo should spawn a projectile")
	}
	if player.HullSnapshot() == nil {
		t.Error("Sanity: ship snapshot should be readable")
	}
}

func TestPhaserFireDamagesTarget(t *testing.T) {
	r, sim, _ := newTestRouter(t)

	route(t, r, "weapons", "weapons", "set_target", map[string]interface{}{"target_id": "raider"})

	raider := sim.GetShip("raider")

	// Shields up: the hit should be absorbed by the facing emitter.
	shieldsBefore := raider.ShieldsSnapshot()["port"].Strength
	if err := route(t, r, "weapons", "phaser", "fire", map[string]interface{}{"array_id": "phaser_1"}); err != nil {
		t.Fatalf("phaser fire failed: %v", err)
	}
	if raider.ShieldsSnapshot()["port"].Strength >= shieldsBefore {
		t.Error("Phaser fire should reduce the target's shields")
	}

	// Shields down: the same hit should reach the hull once the array recharges.
	raider.SetShieldsEnabled(false)
	raider.SetBreaker("shields", false)
	for i := 0; i < 200; i++ {
		sim.Tick()
	}
	hullBefore := 0.0
	for _, sec := range raider.HullSnapshot() {
		hullBefore += sec.Health
	}

	if err := route(t, r, "weapons", "phaser", "fire", map[string]interface{}{"array_id": "phaser_1"}); err != nil {
		t.Fatalf("Second phaser fire should succeed after recharge: %v", err)
	}
	hullAfter := 0.0
	for _, sec := range raider.HullSnapshot() {
		hullAfter += sec.Health
	}
	if hullAfter >= hullBefore {
		t.Error("Phaser fire should damage the target hull once shields are down")
	}
}

func TestPhaserFireSpawnsBeamProjectile(t *testing.T) {
	r, sim, _ := newTestRouter(t)

	route(t, r, "weapons", "weapons", "set_target", map[string]interface{}{"target_id": "raider"})

	if err := route(t, r, "weapons", "phaser", "fire", map[string]interface{}{"array_id": "phaser_1"}); err != nil {
		t.Fatalf("phaser fire failed: %v", err)
	}

	projs := sim.GetAllProjectiles()
	if len(projs) != 1 {
		t.Fatalf("phaser fire should publish one beam projectile, got %d", len(projs))
	}
	for _, p := range projs {
		if p.Type != "phaser" {
			t.Errorf("beam projectile type = %q, want phaser", p.Type)
		}
		if p.TargetID != "raider" {
			t.Errorf("beam projectile target = %q, want raider", p.TargetID)
		}
		if p.Damage != 0 {
			t.Errorf("beam projectile carries damage %v, want 0 (resolved instantly)", p.Damage)
		}
	}

	// The beam is transient and must not deal further damage as it expires.
	hullBefore := 0.0
	for _, sec := range sim.GetShip("raider").HullSnapshot() {
		hullBefore += sec.Health
	}
	for i := 0; i < 200; i++ {
		sim.Tick()
	}
	for _, p := range sim.GetAllProjectiles() {
		if p.SourceID == "player" {
			t.Errorf("player beam projectile %q should expire, still present", p.ID)
		}
	}
	hullAfter := 0.0
	for _, sec := range sim.GetShip("raider").HullSnapshot() {
		hullAfter += sec.Health
	}
	if hullAfter != hullBefore {
		t.Errorf("expiring beam changed hull %v -> %v, want no further damage", hullBefore, hullAfter)
	}
}

func TestPhaserFireHonoursRange(t *testing.T) {
	r, sim, _ := newTestRouter(t)

	route(t, r, "weapons", "weapons", "set_target", map[string]interface{}{"target_id": "raider"})
	sim.GetShip("raider").Position.X = 9000

	if err := route(t, r, "weapons", "phaser", "fire", map[string]interface{}{"array_id": "phaser_1"}); err == nil {
		t.Error("Firing past weapon range should fail")
	}
}

func TestPhaserFireBlockedWhenOffline(t *testing.T) {
	r, _, player := newTestRouter(t)

	player.SetBreaker("weapons", false)
	route(t, r, "weapons", "weapons", "set_target", map[string]interface{}{"target_id": "raider"})

	if err := route(t, r, "weapons", "phaser", "fire", map[string]interface{}{"array_id": "phaser_1"}); err == nil {
		t.Error("Phasers should not fire with the weapons breaker off")
	}
}

func TestSetAlert(t *testing.T) {
	r, sim, _ := newTestRouter(t)

	if err := route(t, r, "captain", "alert", "set_level", map[string]interface{}{"level": "red"}); err != nil {
		t.Fatalf("set_level failed: %v", err)
	}
	if sim.GetAlertLevel() != "red" {
		t.Errorf("Expected red alert, got %s", sim.GetAlertLevel())
	}

	if err := route(t, r, "captain", "alert", "set_level", map[string]interface{}{"level": "green"}); err != nil {
		t.Fatalf("green should map to normal: %v", err)
	}
	if sim.GetAlertLevel() != "normal" {
		t.Errorf("green should normalise, got %s", sim.GetAlertLevel())
	}

	if err := route(t, r, "captain", "alert", "set_level", map[string]interface{}{"level": "purple"}); err == nil {
		t.Error("Invalid alert level should error")
	}
}

func TestHailAndFrequency(t *testing.T) {
	r, _, _ := newTestRouter(t)

	if err := route(t, r, "communications", "comms", "set_frequency", map[string]interface{}{"frequency": 121.5}); err != nil {
		t.Fatalf("set_frequency failed: %v", err)
	}
	_, _, freq := r.CommState()
	if freq != 121.5 {
		t.Errorf("Expected 121.5, got %.2f", freq)
	}

	if err := route(t, r, "communications", "comms", "hail", map[string]interface{}{"target_id": "raider"}); err != nil {
		t.Fatalf("hail failed: %v", err)
	}
	hailing, target, _ := r.CommState()
	if !hailing || target != "raider" {
		t.Errorf("Expected hailing raider, got hailing=%v target=%s", hailing, target)
	}

	if err := route(t, r, "communications", "comms", "hail", map[string]interface{}{"target_id": "ghost"}); err == nil {
		t.Error("Hailing a nonexistent ship should error")
	}
}

func TestScanProgressIsServerOwned(t *testing.T) {
	r, _, _ := newTestRouter(t)

	if err := route(t, r, "operations", "sensors", "set_mode", map[string]interface{}{"mode": "deep_scan"}); err != nil {
		t.Fatalf("set_mode failed: %v", err)
	}
	if err := route(t, r, "operations", "sensors", "deep_scan", map[string]interface{}{"target_id": "raider"}); err != nil {
		t.Fatalf("deep_scan failed: %v", err)
	}

	active, target, progress, mode := r.ScanState()
	if !active || target != "raider" || mode != "deep_scan" {
		t.Fatalf("Expected an active deep scan of raider, got active=%v target=%s mode=%s", active, target, mode)
	}
	if progress != 0 {
		t.Errorf("Scan progress should start at 0, got %.3f", progress)
	}

	// Advance the server-side timer.
	for i := 0; i < 60; i++ {
		r.Update(1.0 / 60.0)
	}
	_, _, progress, _ = r.ScanState()
	if progress <= 0 {
		t.Errorf("Scan progress should advance on the server, got %.3f", progress)
	}
}

func TestScanRequiresSensorsOnline(t *testing.T) {
	r, _, player := newTestRouter(t)

	player.SetBreaker("sensors", false)
	if err := route(t, r, "operations", "sensors", "initiate_scan", map[string]interface{}{"target_id": "raider"}); err == nil {
		t.Error("Scanning with sensors offline should fail")
	}
}

func TestShieldsRaiseLower(t *testing.T) {
	r, _, player := newTestRouter(t)

	if err := route(t, r, "operations", "shields", "lower", nil); err != nil {
		t.Fatalf("lower failed: %v", err)
	}
	if player.Shields.Enabled {
		t.Error("Shields should be down")
	}

	if err := route(t, r, "operations", "shields", "raise", nil); err != nil {
		t.Fatalf("raise failed: %v", err)
	}
	if !player.Shields.Enabled {
		t.Error("Shields should be up")
	}

	player.SetBreaker("shields", false)
	if err := route(t, r, "operations", "shields", "raise", nil); err == nil {
		t.Error("Raising shields with the breaker off should fail")
	}
}

func TestTransporterRequiresPower(t *testing.T) {
	r, _, player := newTestRouter(t)

	if err := route(t, r, "operations", "transporter", "beam_up", nil); err != nil {
		t.Fatalf("beam_up failed: %v", err)
	}
	active, _ := r.TransporterState()
	if !active {
		t.Error("Transporter should be active")
	}

	player.SetBreaker("life_support", false)
	if err := route(t, r, "operations", "transporter", "emergency", nil); err == nil {
		t.Error("Emergency transport with no life support power should fail")
	}
}

func TestRepairTeamDeployRepairs(t *testing.T) {
	r, _, player := newTestRouter(t)

	player.RepairSection(ship.SectionForward, -200)
	before := player.HullSnapshot()[ship.SectionForward].Health

	if err := route(t, r, "first_officer", "crew", "deploy_team", map[string]interface{}{
		"team":    0,
		"section": "forward",
	}); err != nil {
		t.Fatalf("deploy_team failed: %v", err)
	}

	teams := r.RepairTeams()
	if len(teams) != 3 {
		t.Fatalf("Expected 3 repair teams, got %d", len(teams))
	}
	if teams[0]["status"] != "deployed" || teams[0]["location"] != ship.SectionForward {
		t.Errorf("Expected alpha deployed to forward, got %v", teams[0])
	}

	// Two seconds of server time should see the team do work.
	for i := 0; i < 180; i++ {
		r.Update(1.0 / 60.0)
	}
	if player.HullSnapshot()[ship.SectionForward].Health <= before {
		t.Error("A deployed repair team should repair the section")
	}

	if err := route(t, r, "first_officer", "crew", "recall_team", map[string]interface{}{"team": 0}); err != nil {
		t.Fatalf("recall_team failed: %v", err)
	}
	if r.RepairTeams()[0]["status"] != "standing_by" {
		t.Error("Recalled team should be standing by")
	}
}

func TestOrdersAndWaypoints(t *testing.T) {
	r, _, _ := newTestRouter(t)

	route(t, r, "captain", "command", "issue_order", map[string]interface{}{"order": "Hold the line"})
	route(t, r, "captain", "command", "issue_order", map[string]interface{}{"order": "Make ready"})
	orders := r.Orders()
	if len(orders) != 2 || orders[0] != "Hold the line" {
		t.Errorf("Expected 2 orders, got %v", orders)
	}
	route(t, r, "captain", "command", "clear_orders", nil)
	if len(r.Orders()) != 0 {
		t.Error("clear_orders should empty the list")
	}

	err := route(t, r, "relay", "navigation", "set_waypoint", map[string]interface{}{
		"name": "Waypoint Alpha",
		"x":    100.0, "y": 0.0, "z": 200.0,
	})
	if err != nil {
		t.Fatalf("set_waypoint failed: %v", err)
	}
	if len(r.Waypoints()) != 1 {
		t.Error("Expected one waypoint")
	}
	route(t, r, "relay", "navigation", "clear_waypoint", nil)
	if len(r.Waypoints()) != 0 {
		t.Error("clear_waypoint should empty the list")
	}
}

func TestSelfDestructIsServerTimed(t *testing.T) {
	r, sim, _ := newTestRouter(t)

	if err := route(t, r, "captain", "self_destruct", "arm", nil); err != nil {
		t.Fatalf("arm failed: %v", err)
	}
	if r.SelfDestructRemaining() <= 0 {
		t.Error("Server should own the destruct countdown")
	}
	if err := route(t, r, "captain", "self_destruct", "arm", nil); err == nil {
		t.Error("Arming twice should error")
	}
	if err := route(t, r, "captain", "self_destruct", "abort", nil); err != nil {
		t.Fatalf("abort failed: %v", err)
	}
	if r.SelfDestructRemaining() != 0 {
		t.Error("Abort should clear the countdown")
	}

	// Let the countdown run out.
	route(t, r, "captain", "self_destruct", "arm", nil)
	for i := 0; i < int(SelfDestructDuration*60)+10; i++ {
		r.Update(1.0 / 60.0)
	}
	if sim.GetShip("player") != nil {
		t.Error("Self destruct should remove the ship when the timer expires")
	}
}

func TestAutoFireFiresReadyBays(t *testing.T) {
	r, sim, _ := newTestRouter(t)

	route(t, r, "weapons", "weapons", "set_target", map[string]interface{}{"target_id": "raider"})

	// Auto fire on, but nothing is armed yet.
	if err := route(t, r, "weapons", "torpedo", "set_auto_fire", map[string]interface{}{"enabled": true}); err != nil {
		t.Fatalf("set_auto_fire failed: %v", err)
	}
	for i := 0; i < 30; i++ {
		r.Update(1.0 / 60.0)
	}
	if len(sim.GetAllProjectiles()) != 0 {
		t.Error("Auto fire should not launch before a bay is ready")
	}

	route(t, r, "weapons", "torpedo", "arm", map[string]interface{}{"bay_id": 1})
	route(t, r, "weapons", "torpedo", "load", map[string]interface{}{"bay_id": 1})
	route(t, r, "weapons", "torpedo", "lock", map[string]interface{}{"bay_id": 1})

	r.Update(1.0 / 60.0)
	if len(sim.GetAllProjectiles()) == 0 {
		t.Error("Auto fire should launch a bay that is armed, loaded and locked")
	}

	// Turning it off stops further launches.
	route(t, r, "weapons", "torpedo", "set_auto_fire", map[string]interface{}{"enabled": false})
	before := len(sim.GetAllProjectiles())
	for i := 0; i < 60; i++ {
		r.Update(1.0 / 60.0)
	}
	if len(sim.GetAllProjectiles()) > before {
		t.Error("Auto fire off should stop launching")
	}
}

func TestProbeLimit(t *testing.T) {
	r, _, _ := newTestRouter(t)

	for i := 0; i < 5; i++ {
		if err := route(t, r, "relay", "sensors", "launch_probe", nil); err != nil {
			t.Fatalf("probe %d failed: %v", i, err)
		}
	}
	if err := route(t, r, "relay", "sensors", "launch_probe", nil); err == nil {
		t.Error("Running out of probes should error")
	}
}

func TestValueCoercion(t *testing.T) {
	r, _, _ := newTestRouter(t)

	// Panels send scalars and strings, not typed JSON.
	err := route(t, r, "flight", "flight", "set_throttle", map[string]interface{}{"throttle": "0.5"})
	if err != nil {
		t.Fatalf("String throttle should be accepted: %v", err)
	}

	err = route(t, r, "engineer", "power", "toggle_breaker", map[string]interface{}{"breaker": "sensors", "enabled": "off"})
	if err != nil {
		t.Fatalf("String boolean should be accepted: %v", err)
	}
}
