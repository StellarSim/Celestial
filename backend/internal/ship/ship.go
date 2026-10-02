package ship

import (
	"celestial/internal/config"
	"fmt"
	"math"
	"math/rand"
	"sync"
)

// Canonical damage section names.
const (
	SectionForward   = "forward"
	SectionAft       = "aft"
	SectionPort      = "port"
	SectionStarboard = "starboard"
)

const (
	// Turn demand is clamped to this multiple of the class TurnRate, so the
	// helm hard-turn buttons can ask for twice the normal rate without runaway.
	maxTurnDemand = 2.0
	// turnAccel is the angular acceleration in rad/s^2 the ship uses to wind
	// up to and unwind from a commanded turn rate. Higher is more responsive,
	// lower is more inertia.
	turnAccel = 6.0
)

const (
	// Drag is a function of throttle rather than a single constant. Under
	// thrust it stays low so that MaxSpeed, not the drag terminal velocity,
	// is what caps the ship, and the engines dominate the ramp. As the
	// throttle closes it ramps up so the ship settles instead of coasting
	// indefinitely. A single rate cannot give both a quick ramp and a
	// decisive stop.
	dragRateThrusting = 0.02
	dragRateCoasting  = 2.0
)

// Canonical power breaker names.
var BreakerNames = []string{
	"reactor", "engines", "shields", "weapons",
	"sensors", "comms", "life_support", "navigation",
}

type Ship struct {
	mu sync.RWMutex

	ID         string
	ClassID    string
	Name       string
	IsPlayer   bool
	Faction    string
	AlertLevel string

	Position        Vector3
	Velocity        Vector3
	Rotation        Quaternion
	AngularVelocity Vector3

	// Flight control inputs (server authoritative).
	Throttle    float64 // -1 (astern) .. 1 (ahead)
	ThrustAxis  Vector3 // desired thrust direction in ship-local space
	TurnCommand Vector3 // commanded turn demand in ship-local pitch, yaw, roll

	Mass         float64
	MaxSpeed     float64
	Acceleration float64
	TurnRate     float64

	Engines     map[string]*Engine
	Weapons     map[string]*Weapon
	Shields     *ShieldSystem
	Hull        *HullSystem
	Subsystems  map[string]*Subsystem
	LaunchBays  map[string]*LaunchBay
	Power       *PowerSystem
	LifeSupport *LifeSupportSystem

	Crew map[string]*CrewMember

	TargetID string
	Docked   bool
}

type Vector3 struct {
	X, Y, Z float64
}

type Quaternion struct {
	W, X, Y, Z float64
}

type Engine struct {
	ID        string
	Type      string
	Thrust    float64
	MaxHealth float64
	Health    float64
	Enabled   bool
	PowerDraw float64
	OnFire    bool
}

type Weapon struct {
	ID           string
	Type         string
	Damage       float64
	Range        float64
	CooldownTime float64
	Cooldown     float64
	MaxHealth    float64
	Health       float64
	Enabled      bool
	PowerDraw    float64
	OnFire       bool
	Armed        bool
	Loaded       bool
	Locked       bool
	AmmoCapacity int
	AmmoCount    int
	Facing       Vector3
}

type ShieldSystem struct {
	Emitters     map[string]*ShieldEmitter
	RechargeRate float64
	PowerDraw    float64
	Enabled      bool
}

type ShieldEmitter struct {
	ID          string
	Facing      string
	MaxStrength float64
	Strength    float64
	MaxHealth   float64
	Health      float64
	OnFire      bool
}

type HullSystem struct {
	Sections map[string]*HullSection
}

type HullSection struct {
	ID        string
	MaxArmor  float64
	Armor     float64
	MaxHealth float64
	Health    float64
	Breached  bool
	OnFire    bool
}

type Subsystem struct {
	ID        string
	Type      string
	MaxHealth float64
	Health    float64
	Enabled   bool
	PowerDraw float64
	OnFire    bool
}

type LaunchBay struct {
	ID        string
	Capacity  int
	Current   int
	MaxHealth float64
	Health    float64
	OnFire    bool
}

type PowerSystem struct {
	MaxCapacity     float64
	CurrentCapacity float64
	Generation      float64
	Consumption     float64
	Breakers        map[string]*Breaker
}

type Breaker struct {
	ID      string
	System  string
	Enabled bool
	Load    float64
}

type LifeSupportSystem struct {
	Compartments map[string]*Compartment
}

type Compartment struct {
	ID          string
	MaxPressure float64
	Pressure    float64
	MaxOxygen   float64
	Oxygen      float64
	Temperature float64
	OnFire      bool
	Breached    bool
}

type CrewMember struct {
	Role   string
	Health float64
	Status string
}

func NewShip(id, classID, name string, class *config.ShipClass, isPlayer bool) *Ship {
	faction := "hostile"
	if isPlayer {
		faction = "player"
	}
	sh := &Ship{
		ID:              id,
		ClassID:         classID,
		Name:            name,
		IsPlayer:        isPlayer,
		Faction:         faction,
		AlertLevel:      "normal",
		Position:        Vector3{X: 0, Y: 0, Z: 0},
		Velocity:        Vector3{X: 0, Y: 0, Z: 0},
		Rotation:        Quaternion{W: 1, X: 0, Y: 0, Z: 0},
		AngularVelocity: Vector3{X: 0, Y: 0, Z: 0},
		Throttle:        0,
		ThrustAxis:      Vector3{X: 0, Y: 0, Z: 1},
		TurnCommand:     Vector3{X: 0, Y: 0, Z: 0},
		Mass:            class.Mass,
		MaxSpeed:        class.MaxSpeed,
		Acceleration:    class.Acceleration,
		TurnRate:        class.TurnRate,
		Engines:         make(map[string]*Engine),
		Weapons:         make(map[string]*Weapon),
		Subsystems:      make(map[string]*Subsystem),
		LaunchBays:      make(map[string]*LaunchBay),
		Crew:            make(map[string]*CrewMember),
	}

	for _, engCfg := range class.Engines {
		sh.Engines[engCfg.ID] = &Engine{
			ID:        engCfg.ID,
			Type:      engCfg.Type,
			Thrust:    engCfg.Thrust,
			MaxHealth: engCfg.Health,
			Health:    engCfg.Health,
			Enabled:   true,
			PowerDraw: engCfg.PowerDraw,
		}
	}

	for _, wpnCfg := range class.Weapons {
		facing := Vector3{X: 0, Y: 0, Z: 1}
		loaded := wpnCfg.AmmoCapacity > 0 && wpnCfg.Type == "torpedo"
		sh.Weapons[wpnCfg.ID] = &Weapon{
			ID:           wpnCfg.ID,
			Type:         wpnCfg.Type,
			Damage:       wpnCfg.Damage,
			Range:        wpnCfg.Range,
			CooldownTime: wpnCfg.CooldownTime,
			Cooldown:     0,
			MaxHealth:    wpnCfg.Health,
			Health:       wpnCfg.Health,
			Enabled:      true,
			PowerDraw:    wpnCfg.PowerDraw,
			Armed:        false,
			Loaded:       loaded,
			Locked:       false,
			AmmoCapacity: wpnCfg.AmmoCapacity,
			AmmoCount:    wpnCfg.AmmoCapacity,
			Facing:       facing,
		}
	}

	sh.Shields = &ShieldSystem{
		Emitters:     make(map[string]*ShieldEmitter),
		RechargeRate: class.Shields.RechargeRate,
		PowerDraw:    class.Shields.PowerDraw,
		Enabled:      true,
	}
	for _, emCfg := range class.Shields.Emitters {
		sh.Shields.Emitters[emCfg.ID] = &ShieldEmitter{
			ID:          emCfg.ID,
			Facing:      emCfg.Facing,
			MaxStrength: emCfg.Strength,
			Strength:    emCfg.Strength,
			MaxHealth:   emCfg.Health,
			Health:      emCfg.Health,
		}
	}

	sh.Hull = &HullSystem{Sections: make(map[string]*HullSection)}
	for _, secCfg := range class.Hull.Sections {
		sh.Hull.Sections[secCfg.ID] = &HullSection{
			ID:        secCfg.ID,
			MaxArmor:  secCfg.Armor,
			Armor:     secCfg.Armor,
			MaxHealth: secCfg.Health,
			Health:    secCfg.Health,
		}
	}

	for _, subCfg := range class.Subsystems {
		sh.Subsystems[subCfg.ID] = &Subsystem{
			ID:        subCfg.ID,
			Type:      subCfg.Type,
			MaxHealth: subCfg.Health,
			Health:    subCfg.Health,
			Enabled:   true,
			PowerDraw: subCfg.PowerDraw,
		}
	}

	for _, bayCfg := range class.LaunchBays {
		sh.LaunchBays[bayCfg.ID] = &LaunchBay{
			ID:        bayCfg.ID,
			Capacity:  bayCfg.Capacity,
			Current:   bayCfg.Capacity,
			MaxHealth: bayCfg.Health,
			Health:    bayCfg.Health,
		}
	}

	// Power: reactor generates; each canonical breaker starts closed (on).
	sh.Power = &PowerSystem{
		MaxCapacity:     10000,
		CurrentCapacity: 10000,
		Generation:      1000,
		Consumption:     0,
		Breakers:        make(map[string]*Breaker),
	}
	for _, b := range BreakerNames {
		sh.Power.Breakers[b] = &Breaker{ID: b, System: b, Enabled: true}
	}

	sh.LifeSupport = &LifeSupportSystem{Compartments: make(map[string]*Compartment)}
	for _, name := range []string{"bridge", "engineering", "weapons_bay", "crew_quarters", "cargo_bay"} {
		sh.LifeSupport.Compartments[name] = &Compartment{
			ID:          name,
			MaxPressure: 101.3,
			Pressure:    101.3,
			MaxOxygen:   21.0,
			Oxygen:      21.0,
			Temperature: 20.0,
		}
	}

	if isPlayer {
		for _, role := range []string{"engineer", "flight", "weapons", "captain", "comms", "operations", "relay", "first_officer"} {
			sh.Crew[role] = &CrewMember{Role: role, Health: 100.0, Status: "healthy"}
		}
	}

	return sh
}

// ---------------------------------------------------------------------------
// Frame update
// ---------------------------------------------------------------------------

func (s *Ship) Update(dt float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.updatePhysics(dt)
	s.updatePower(dt)
	s.updateShields(dt)
	s.updateWeapons(dt)
	s.updateDamage(dt)
	s.updateLifeSupport(dt)
}

func (s *Ship) updatePhysics(dt float64) {
	// Total main-engine thrust available, scaled by health.
	mainThrust := 0.0
	enginesOnline := s.breakerOn("engines")
	if enginesOnline {
		for _, engine := range s.Engines {
			if engine.Type == "main" && engine.Enabled && engine.Health > 0 {
				mainThrust += engine.Thrust * (engine.Health / engine.MaxHealth)
			}
		}
	}

	fwd := s.forwardLocked()
	right := s.rightLocked()
	up := s.upLocked()

	axis := s.ThrustAxis
	// Blend the desired local thrust direction with forward for heading control.
	localX := clampf(axis.X, -1.0, 1.0)
	localY := clampf(axis.Y, -1.0, 1.0)
	localZ := clampf(axis.Z, -1.0, 1.0)
	localMag := math.Sqrt(localX*localX + localY*localY + localZ*localZ)
	if localMag < 0.0001 {
		localX, localY, localZ, localMag = 0, 0, 1, 1
	}
	localX /= localMag
	localY /= localMag
	localZ /= localMag

	dir := Vector3{
		X: right.X*localX + up.X*localY + fwd.X*localZ,
		Y: right.Y*localX + up.Y*localY + fwd.Y*localZ,
		Z: right.Z*localX + up.Z*localY + fwd.Z*localZ,
	}

	// Acceleration comes from ship config, scaled by how much main thrust is
	// actually available (damaged or offline engines accelerate less) and by the
	// throttle setting.
	maxThrust := 0.0
	for _, engine := range s.Engines {
		if engine.Type == "main" {
			maxThrust += engine.Thrust
		}
	}
	thrustFraction := 1.0
	if maxThrust > 0 {
		thrustFraction = clampf(mainThrust/maxThrust, 0.0, 1.0)
	}

	magnitude := s.Acceleration * thrustFraction * s.Throttle
	accel := Vector3{X: dir.X * magnitude, Y: dir.Y * magnitude, Z: dir.Z * magnitude}

	s.Velocity.X += accel.X * dt
	s.Velocity.Y += accel.Y * dt
	s.Velocity.Z += accel.Z * dt

	// Frame-rate independent drag: velocity *= exp(-k*dt). The rate follows
	// the throttle, so a ship under full thrust accelerates against almost no
	// damping and is limited by MaxSpeed, while releasing the throttle brings
	// damping up sharply and stops the ship in about a second.
	throttleMag := math.Abs(s.Throttle)
	dragRate := dragRateCoasting + (dragRateThrusting-dragRateCoasting)*throttleMag
	damp := math.Exp(-dragRate * dt)
	s.Velocity.X *= damp
	s.Velocity.Y *= damp
	s.Velocity.Z *= damp

	speed := math.Sqrt(s.Velocity.X*s.Velocity.X + s.Velocity.Y*s.Velocity.Y + s.Velocity.Z*s.Velocity.Z)
	if s.MaxSpeed > 0 && speed > s.MaxSpeed {
		scale := s.MaxSpeed / speed
		s.Velocity.X *= scale
		s.Velocity.Y *= scale
		s.Velocity.Z *= scale
	}

	s.Position.X += s.Velocity.X * dt
	s.Position.Y += s.Velocity.Y * dt
	s.Position.Z += s.Velocity.Z * dt

	// Chase the commanded turn rate at a bounded angular acceleration. The
	// ship keeps spinning a little after the stick centres and winds up
	// gradually when the stick deflects, which is what gives the hull its
	// rotational inertia. Spinning up and spinning down share a limit so the
	// helm feels symmetric.
	cmd := s.commandedAngularVelocityLocked()
	deltaV := Vector3{
		X: cmd.X - s.AngularVelocity.X,
		Y: cmd.Y - s.AngularVelocity.Y,
		Z: cmd.Z - s.AngularVelocity.Z,
	}
	if mag := math.Sqrt(deltaV.X*deltaV.X + deltaV.Y*deltaV.Y + deltaV.Z*deltaV.Z); mag > 0 {
		// Scale the step by the whole gap so a diagonal demand does not
		// accelerate faster than a single axis.
		step := minf(1.0, turnAccel*dt/mag)
		s.AngularVelocity.X += deltaV.X * step
		s.AngularVelocity.Y += deltaV.Y * step
		s.AngularVelocity.Z += deltaV.Z * step
	}

	angSpeed := math.Sqrt(
		s.AngularVelocity.X*s.AngularVelocity.X +
			s.AngularVelocity.Y*s.AngularVelocity.Y +
			s.AngularVelocity.Z*s.AngularVelocity.Z)
	angle := angSpeed * dt
	if angle > 0.0001 {
		axisV := Vector3{
			X: s.AngularVelocity.X / angSpeed,
			Y: s.AngularVelocity.Y / angSpeed,
			Z: s.AngularVelocity.Z / angSpeed,
		}
		deltaQ := axisAngleToQuaternion(axisV, angle)
		s.Rotation = normalizeQuaternion(multiplyQuaternions(deltaQ, s.Rotation))
	}
}

func (s *Ship) updatePower(dt float64) {
	consumption := 0.0
	engineLoad := 0.0
	weaponLoad := 0.0
	shieldLoad := 0.0
	for _, br := range s.Power.Breakers {
		br.Load = 0
	}
	if s.breakerOn("engines") {
		for _, engine := range s.Engines {
			if engine.Enabled {
				consumption += engine.PowerDraw
				engineLoad += engine.PowerDraw
			}
		}
	}
	if s.breakerOn("weapons") {
		for _, weapon := range s.Weapons {
			if weapon.Enabled {
				consumption += weapon.PowerDraw
				weaponLoad += weapon.PowerDraw
			}
		}
	}
	if s.breakerOn("shields") && s.Shields.Enabled {
		consumption += s.Shields.PowerDraw
		shieldLoad += s.Shields.PowerDraw
	}
	for _, subsystem := range s.Subsystems {
		if subsystem.Enabled && s.breakerOn(subsystem.ID) {
			consumption += subsystem.PowerDraw
			if br, ok := s.Power.Breakers[subsystem.ID]; ok {
				br.Load = subsystem.PowerDraw
			}
		}
	}

	s.Power.Consumption = consumption

	// Reactor breaker off means zero generation. Integrate exactly once.
	gen := s.Power.Generation
	if !s.breakerOn("reactor") {
		gen = 0
	}
	s.Power.CurrentCapacity += (gen - consumption) * dt
	if s.Power.CurrentCapacity > s.Power.MaxCapacity {
		s.Power.CurrentCapacity = s.Power.MaxCapacity
	}
	if s.Power.CurrentCapacity < 0 {
		s.Power.CurrentCapacity = 0
	}

	// Per-breaker load readout for panels.
	if br, ok := s.Power.Breakers["reactor"]; ok {
		if s.breakerOn("reactor") {
			br.Load = gen
		} else {
			br.Load = 0
		}
	}
	if br, ok := s.Power.Breakers["engines"]; ok {
		br.Load = engineLoad
	}
	if br, ok := s.Power.Breakers["weapons"]; ok {
		br.Load = weaponLoad
	}
	if br, ok := s.Power.Breakers["shields"]; ok {
		br.Load = shieldLoad
	}
}

func (s *Ship) updateShields(dt float64) {
	if !s.Shields.Enabled || !s.breakerOn("shields") {
		return
	}
	for _, emitter := range s.Shields.Emitters {
		if emitter.Health > 0 && emitter.Strength < emitter.MaxStrength {
			emitter.Strength += s.Shields.RechargeRate * dt
			if emitter.Strength > emitter.MaxStrength {
				emitter.Strength = emitter.MaxStrength
			}
		}
	}
}

func (s *Ship) updateWeapons(dt float64) {
	for _, weapon := range s.Weapons {
		if weapon.Cooldown > 0 {
			weapon.Cooldown -= dt
			if weapon.Cooldown < 0 {
				weapon.Cooldown = 0
			}
		}
	}
}

func (s *Ship) updateDamage(dt float64) {
	for _, engine := range s.Engines {
		if engine.OnFire {
			engine.Health -= 5.0 * dt
			if engine.Health < 0 {
				engine.Health = 0
			}
		}
	}
	for _, weapon := range s.Weapons {
		if weapon.OnFire {
			weapon.Health -= 5.0 * dt
			if weapon.Health < 0 {
				weapon.Health = 0
			}
		}
	}
	for _, emitter := range s.Shields.Emitters {
		if emitter.OnFire {
			emitter.Health -= 5.0 * dt
			if emitter.Health < 0 {
				emitter.Health = 0
			}
		}
	}
	for _, section := range s.Hull.Sections {
		if section.OnFire {
			section.Health -= 5.0 * dt
			if section.Health <= 0 {
				section.Health = 0
				section.Breached = true
				s.markCompartmentBreached(section.ID)
			}
		}
	}
	for _, subsystem := range s.Subsystems {
		if subsystem.OnFire {
			subsystem.Health -= 5.0 * dt
			if subsystem.Health <= 0 {
				subsystem.Health = 0
				subsystem.Enabled = false
			}
		}
	}
}

func (s *Ship) updateLifeSupport(dt float64) {
	if !s.breakerOn("life_support") {
		return
	}
	for _, comp := range s.LifeSupport.Compartments {
		if comp.Breached {
			comp.Pressure -= 10.0 * dt
			comp.Oxygen -= 2.0 * dt
			if comp.Pressure < 0 {
				comp.Pressure = 0
			}
			if comp.Oxygen < 0 {
				comp.Oxygen = 0
			}
		}
		if comp.OnFire {
			comp.Oxygen -= 0.5 * dt
			comp.Temperature += 10.0 * dt
		}
	}
}

// ---------------------------------------------------------------------------
// Orientation helpers
// ---------------------------------------------------------------------------

func (s *Ship) forwardLocked() Vector3 {
	// Forward is local -Z mapped into world space.
	return rotateVectorByQuaternion(Vector3{X: 0, Y: 0, Z: -1}, s.Rotation)
}

func (s *Ship) rightLocked() Vector3 {
	return rotateVectorByQuaternion(Vector3{X: 1, Y: 0, Z: 0}, s.Rotation)
}

func (s *Ship) upLocked() Vector3 {
	return rotateVectorByQuaternion(Vector3{X: 0, Y: 1, Z: 0}, s.Rotation)
}

// Forward returns the ship's world-space forward vector.
func (s *Ship) Forward() Vector3 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.forwardLocked()
}

// GetPosition returns a copy of the ship's position.
func (s *Ship) GetPosition() Vector3 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Position
}

// GetVelocity returns a copy of the ship's velocity.
func (s *Ship) GetVelocity() Vector3 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Velocity
}

// WeaponsSnapshot returns copies of weapon state for safe external reads.
func (s *Ship) WeaponsSnapshot() map[string]Weapon {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]Weapon, len(s.Weapons))
	for k, v := range s.Weapons {
		out[k] = *v
	}
	return out
}

// HullSnapshot returns copies of hull section state for safe external reads.
func (s *Ship) HullSnapshot() map[string]HullSection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]HullSection, len(s.Hull.Sections))
	for k, v := range s.Hull.Sections {
		out[k] = *v
	}
	return out
}

// ShieldsSnapshot returns copies of shield emitter state for safe external reads.
func (s *Ship) ShieldsSnapshot() map[string]ShieldEmitter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]ShieldEmitter, len(s.Shields.Emitters))
	for k, v := range s.Shields.Emitters {
		out[k] = *v
	}
	return out
}

// Right returns the ship's world-space right vector.
func (s *Ship) Right() Vector3 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rightLocked()
}

// Returns the ship's world-space up vector.
func (s *Ship) Up() Vector3 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.upLocked()
}

// FacingFor returns which damage section a world-space point falls into relative
// to the ship's orientation: forward, aft, port, or starboard.
func (s *Ship) FacingFor(point Vector3) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.facingForLocked(point)
}

func (s *Ship) facingForLocked(point Vector3) string {
	rel := Vector3{X: point.X - s.Position.X, Y: point.Y - s.Position.Y, Z: point.Z - s.Position.Z}
	fwd := s.forwardLocked()
	right := s.rightLocked()
	// Choose the axis the point is most aligned with (forward/-forward/right/-right).
	fDot := rel.X*fwd.X + rel.Y*fwd.Y + rel.Z*fwd.Z
	rDot := rel.X*right.X + rel.Y*right.Y + rel.Z*right.Z
	if math.Abs(fDot) >= math.Abs(rDot) {
		if fDot >= 0 {
			return SectionForward
		}
		return SectionAft
	}
	if rDot >= 0 {
		return SectionStarboard
	}
	return SectionPort
}

func rotateVectorByQuaternion(v Vector3, q Quaternion) Vector3 {
	// Standard v' = v + 2*w*(q_vec x v) + 2*(q_vec x (q_vec x v))
	qx, qy, qz := q.X, q.Y, q.Z

	c1 := Vector3{
		X: 2.0 * (qy*v.Z - qz*v.Y),
		Y: 2.0 * (qz*v.X - qx*v.Z),
		Z: 2.0 * (qx*v.Y - qy*v.X),
	}
	c2 := Vector3{
		X: 2.0 * (qy*c1.Z - qz*c1.Y),
		Y: 2.0 * (qz*c1.X - qx*c1.Z),
		Z: 2.0 * (qx*c1.Y - qy*c1.X),
	}
	return Vector3{
		X: v.X + q.W*c1.X + c2.X,
		Y: v.Y + q.W*c1.Y + c2.Y,
		Z: v.Z + q.W*c1.Z + c2.Z,
	}
}

// ---------------------------------------------------------------------------
// Input (server authoritative flight control)
// ---------------------------------------------------------------------------

func (s *Ship) ApplyThrust(x, y, z float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Throttle = clampf(z, -1.0, 1.0)
	s.ThrustAxis = Vector3{X: x, Y: y, Z: z}
}

func (s *Ship) SetThrottle(t float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Throttle = clampf(t, -1.0, 1.0)
}

// SetLocalRotationRate commands a turn rate about the ship's own pitch, yaw
// and roll axes. Each argument is a demand in the range
// -maxTurnDemand..maxTurnDemand, scaled by the class TurnRate, and the call
// overwrites the previous command rather than adding to it, so the turn rate
// depends on the stick position and not on how often commands arrive. The
// ship accelerates toward the commanded rate over time rather than snapping
// to it, so it still carries rotational inertia when the stick centres.
func (s *Ship) SetLocalRotationRate(pitch, yaw, roll float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TurnCommand = Vector3{
		X: clampf(pitch, -maxTurnDemand, maxTurnDemand),
		Y: clampf(yaw, -maxTurnDemand, maxTurnDemand),
		Z: clampf(roll, -maxTurnDemand, maxTurnDemand),
	}
}

// commandedAngularVelocityLocked resolves TurnCommand into a world space
// angular velocity from the ship's current basis.
func (s *Ship) commandedAngularVelocityLocked() Vector3 {
	p := s.TurnCommand.X * s.TurnRate
	y := s.TurnCommand.Y * s.TurnRate
	r := s.TurnCommand.Z * s.TurnRate
	if p == 0 && y == 0 && r == 0 {
		return Vector3{}
	}

	right := s.rightLocked()
	up := s.upLocked()
	forward := s.forwardLocked()
	return Vector3{
		X: right.X*p + up.X*y + forward.X*r,
		Y: right.Y*p + up.Y*y + forward.Y*r,
		Z: right.Z*p + up.Z*y + forward.Z*r,
	}
}

// ---------------------------------------------------------------------------
// Power breakers
// ---------------------------------------------------------------------------

func (s *Ship) SetBreaker(name string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if br, ok := s.Power.Breakers[name]; ok {
		br.Enabled = enabled
	}
}

// breakerOn must be called with s.mu held (read or write).
func (s *Ship) breakerOn(name string) bool {
	br, ok := s.Power.Breakers[name]
	return ok && br.Enabled
}

// BreakerOn reports whether a named breaker is closed (supplying power).
func (s *Ship) BreakerOn(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.breakerOn(name)
}

// SubsystemOnline reports whether a subsystem is powered and undamaged.
func (s *Ship) SubsystemOnline(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.subsystemOnlineLocked(name)
}

func (s *Ship) subsystemOnlineLocked(name string) bool {
	if !s.breakerOn(name) {
		return false
	}
	if sub, ok := s.Subsystems[name]; ok {
		return sub.Enabled && sub.Health > 0
	}
	return true
}

func (s *Ship) SetShieldsEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Shields.Enabled = enabled
}

// Moves the ship instantly and clears velocity (GM teleport).
func (s *Ship) SetPosition(p Vector3) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Position = p
	s.Velocity = Vector3{}
}

// Refills every emitter to max.
func (s *Ship) RestoreShields() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Shields == nil {
		return
	}
	for _, e := range s.Shields.Emitters {
		e.Strength = e.MaxStrength
	}
	s.Shields.Enabled = true
}

// Drains every emitter by amount.
func (s *Ship) DamageShields(amount float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Shields == nil {
		return
	}
	for _, e := range s.Shields.Emitters {
		e.Strength = math.Max(0, e.Strength-amount)
	}
}

// Repairs every section to max and clears damage flags.
func (s *Ship) RestoreHull() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Hull == nil {
		return
	}
	for _, sec := range s.Hull.Sections {
		sec.Health = sec.MaxHealth
		sec.Breached = false
		sec.OnFire = false
	}
}

// Updates the display name.
func (s *Ship) SetName(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Name = name
}

// Updates the faction tag.
func (s *Ship) SetFaction(faction string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Faction = faction
}

// GetFaction reads the faction tag under the ship lock.
func (s *Ship) GetFaction() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Faction
}

// MutateWeapon applies fn to a weapon under the ship lock.
func (s *Ship) MutateWeapon(weaponID string, fn func(*Weapon) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, ok := s.Weapons[weaponID]
	if !ok {
		return fmt.Errorf("weapon not found: %s", weaponID)
	}
	return fn(w)
}

// ---------------------------------------------------------------------------
// Weapons
// ---------------------------------------------------------------------------

// FireWeapon attempts to fire. Checks range, cooldown, power, and (for
// torpedoes) armed/loaded/locked/ammo gating. Returns success.
func (s *Ship) FireWeapon(weaponID string, targetID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fireWeaponLocked(weaponID, targetID)
}

func (s *Ship) fireWeaponLocked(weaponID string, targetID string) bool {
	weapon, ok := s.Weapons[weaponID]
	if !ok || weapon.Health <= 0 || weapon.Cooldown > 0 {
		return false
	}
	if !weapon.Enabled || !s.breakerOn("weapons") {
		return false
	}

	if weapon.Type == "torpedo" {
		if !weapon.Armed || !weapon.Loaded || !weapon.Locked {
			return false
		}
		if weapon.AmmoCount <= 0 {
			return false
		}
		weapon.AmmoCount--
		weapon.Loaded = false
	}

	weapon.Cooldown = weapon.CooldownTime
	s.TargetID = targetID
	return true
}

// InWeaponRange reports whether target is within the weapon's configured range.
func (s *Ship) InWeaponRange(weaponID string, targetPos Vector3) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w, ok := s.Weapons[weaponID]
	if !ok {
		return false
	}
	if w.Range <= 0 {
		return true
	}
	d := Vector3{X: targetPos.X - s.Position.X, Y: targetPos.Y - s.Position.Y, Z: targetPos.Z - s.Position.Z}
	return math.Sqrt(d.X*d.X+d.Y*d.Y+d.Z*d.Z) <= w.Range
}

// ---------------------------------------------------------------------------
// Damage
// ---------------------------------------------------------------------------

// TakeDamage applies damage at a damage-section location. Shields absorb first;
// overflow goes to hull, possibly starting fires / causing breaches.
func (s *Ship) TakeDamage(amount float64, location string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.takeDamageLocked(amount, location, "kinetic")
}

// ApplyTypedDamage applies damage of a given type, which affects fire/overload
// chances for energy and explosive ordnance.
func (s *Ship) ApplyTypedDamage(amount float64, location, damageType string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch damageType {
	case "energy":
		s.takeDamageLocked(amount*1.5, location, damageType)
		s.causeSystemOverloadLocked()
	case "explosive":
		s.takeDamageLocked(amount, location, damageType)
		for _, loc := range adjacentLocations(location) {
			s.takeDamageLocked(amount*0.5, loc, damageType)
		}
		if math_rand() < 0.5 {
			s.startFireLocked(location)
		}
	default:
		s.takeDamageLocked(amount, location, damageType)
	}
	s.checkCascadingFailuresLocked(location)
}

func (s *Ship) takeDamageLocked(amount float64, location string, damageType string) {
	if location == "" {
		location = SectionForward
	}

	// Shields absorb.
	if s.Shields.Enabled && s.breakerOn("shields") {
		if emitter, has := s.Shields.Emitters[location]; has && emitter.Strength > 0 {
			emitter.Strength -= amount
			if emitter.Strength < 0 {
				amount = -emitter.Strength
				emitter.Strength = 0
			} else {
				return
			}
		}
	}

	section, hasSection := s.Hull.Sections[location]
	if hasSection {
		if section.Armor > 0 {
			section.Armor -= amount * 0.5
			if section.Armor < 0 {
				section.Armor = 0
			}
		}
		section.Health -= amount
		if damageType == "kinetic" && math_rand() < 0.3 {
			s.startFireLocked(location)
		}
		if section.Health <= 0 {
			section.Health = 0
			section.Breached = true
			s.markCompartmentBreached(location)
		}
	}
}

func (s *Ship) checkCascadingFailuresLocked(location string) {
	section, ok := s.Hull.Sections[location]
	if !ok {
		return
	}
	if section.Health <= 0 && !section.Breached {
		section.Breached = true
		s.markCompartmentBreached(location)
	}
	if section.OnFire {
		for _, adj := range adjacentLocations(location) {
			if math_rand() < 0.1 {
				s.startFireLocked(adj)
			}
		}
	}
}

func (s *Ship) startFireLocked(location string) {
	if sec, ok := s.Hull.Sections[location]; ok {
		sec.OnFire = true
	}
	// Fire in a hull section burns in the compartment behind it.
	if comp, ok := s.LifeSupport.Compartments[sectionCompartment(location)]; ok {
		comp.OnFire = true
	}
}

func (s *Ship) causeSystemOverloadLocked() {
	for _, subsystem := range s.Subsystems {
		if math_rand() < 0.1 {
			subsystem.Health -= 20
			if subsystem.Health <= 0 {
				subsystem.Health = 0
				subsystem.Enabled = false
			}
		}
	}
}

func (s *Ship) markCompartmentBreached(sectionID string) {
	compartment := sectionCompartment(sectionID)
	if comp, ok := s.LifeSupport.Compartments[compartment]; ok {
		comp.Breached = true
	}
}

func sectionCompartment(sectionID string) string {
	switch sectionID {
	case SectionForward:
		return "bridge"
	case SectionAft:
		return "engineering"
	case SectionPort, SectionStarboard:
		return "crew_quarters"
	default:
		return sectionID
	}
}

func adjacentLocations(location string) []string {
	switch location {
	case SectionForward:
		return []string{SectionPort, SectionStarboard, "bridge"}
	case SectionAft:
		return []string{SectionPort, SectionStarboard, "engineering"}
	case SectionPort, SectionStarboard:
		return []string{SectionForward, SectionAft}
	default:
		return nil
	}
}

func math_rand() float64 {
	return rand.Float64()
}

// Public mutation helpers for the action catalog (all lock-protected).

func (s *Ship) StartFire(section string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startFireLocked(section)
}

func (s *Ship) ExtinguishFire(section string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sec, ok := s.Hull.Sections[section]; ok {
		sec.OnFire = false
	}
	if comp, ok := s.LifeSupport.Compartments[section]; ok {
		comp.OnFire = false
	}
}

func (s *Ship) SealBreach(section string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sec, ok := s.Hull.Sections[section]; ok {
		sec.Breached = false
		if sec.Health > 0 {
			sec.Armor = math.Min(sec.Armor, sec.MaxArmor)
		}
	}
	if comp, ok := s.LifeSupport.Compartments[sectionCompartment(section)]; ok {
		comp.Breached = false
		comp.Pressure = comp.MaxPressure
		comp.Oxygen = comp.MaxOxygen
	}
}

// RepairSection restores hull health and clears damage state. A negative amount
// removes health instead, which is how tests and GM tooling tear a section down.
func (s *Ship) RepairSection(section string, amount float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sec, ok := s.Hull.Sections[section]; ok {
		sec.Health = math.Max(0, math.Min(sec.Health+amount, sec.MaxHealth))
		if sec.Health <= 0 {
			sec.Breached = true
			s.markCompartmentBreached(section)
			return
		}
		sec.Breached = false
		sec.OnFire = false
	}
}

func (s *Ship) SetTarget(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TargetID = id
}

// SetAlertLevel updates the ship's alert level under the ship lock.
func (s *Ship) SetAlertLevel(level string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AlertLevel = level
}

// GetAlertLevel reads the ship's alert level under the ship lock.
func (s *Ship) GetAlertLevel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.AlertLevel
}

// SetDocked updates the docked flag under the ship lock.
func (s *Ship) SetDocked(docked bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Docked = docked
}

// GetTargetID reads the locked target under the ship lock.
func (s *Ship) GetTargetID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TargetID
}

// ---------------------------------------------------------------------------
// Clone (deep copy for snapshots)
// ---------------------------------------------------------------------------

// Clone returns a deep copy of the ship for snapshotting.
func (s *Ship) Clone() *Ship {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp := &Ship{
		ID:              s.ID,
		ClassID:         s.ClassID,
		Name:            s.Name,
		IsPlayer:        s.IsPlayer,
		Faction:         s.Faction,
		AlertLevel:      s.AlertLevel,
		Position:        s.Position,
		Velocity:        s.Velocity,
		Rotation:        s.Rotation,
		AngularVelocity: s.AngularVelocity,
		Throttle:        s.Throttle,
		ThrustAxis:      s.ThrustAxis,
		TurnCommand:     s.TurnCommand,
		Mass:            s.Mass,
		MaxSpeed:        s.MaxSpeed,
		Acceleration:    s.Acceleration,
		TurnRate:        s.TurnRate,
		TargetID:        s.TargetID,
		Docked:          s.Docked,
	}

	cp.Engines = make(map[string]*Engine, len(s.Engines))
	for k, v := range s.Engines {
		e := *v
		cp.Engines[k] = &e
	}
	cp.Weapons = make(map[string]*Weapon, len(s.Weapons))
	for k, v := range s.Weapons {
		w := *v
		cp.Weapons[k] = &w
	}

	cp.Shields = &ShieldSystem{
		Emitters:     make(map[string]*ShieldEmitter, len(s.Shields.Emitters)),
		RechargeRate: s.Shields.RechargeRate,
		PowerDraw:    s.Shields.PowerDraw,
		Enabled:      s.Shields.Enabled,
	}
	for k, v := range s.Shields.Emitters {
		e := *v
		cp.Shields.Emitters[k] = &e
	}

	cp.Hull = &HullSystem{Sections: make(map[string]*HullSection, len(s.Hull.Sections))}
	for k, v := range s.Hull.Sections {
		sec := *v
		cp.Hull.Sections[k] = &sec
	}

	cp.Subsystems = make(map[string]*Subsystem, len(s.Subsystems))
	for k, v := range s.Subsystems {
		sub := *v
		cp.Subsystems[k] = &sub
	}

	cp.LaunchBays = make(map[string]*LaunchBay, len(s.LaunchBays))
	for k, v := range s.LaunchBays {
		b := *v
		cp.LaunchBays[k] = &b
	}

	cp.Power = &PowerSystem{
		MaxCapacity:     s.Power.MaxCapacity,
		CurrentCapacity: s.Power.CurrentCapacity,
		Generation:      s.Power.Generation,
		Consumption:     s.Power.Consumption,
		Breakers:        make(map[string]*Breaker, len(s.Power.Breakers)),
	}
	for k, v := range s.Power.Breakers {
		b := *v
		cp.Power.Breakers[k] = &b
	}

	cp.LifeSupport = &LifeSupportSystem{Compartments: make(map[string]*Compartment, len(s.LifeSupport.Compartments))}
	for k, v := range s.LifeSupport.Compartments {
		c := *v
		cp.LifeSupport.Compartments[k] = &c
	}

	cp.Crew = make(map[string]*CrewMember, len(s.Crew))
	for k, v := range s.Crew {
		c := *v
		cp.Crew[k] = &c
	}

	return cp
}

// ---------------------------------------------------------------------------
// Quaternion math
// ---------------------------------------------------------------------------

func axisAngleToQuaternion(axis Vector3, angle float64) Quaternion {
	half := angle * 0.5
	s := math.Sin(half)
	return Quaternion{W: math.Cos(half), X: axis.X * s, Y: axis.Y * s, Z: axis.Z * s}
}

func multiplyQuaternions(q1, q2 Quaternion) Quaternion {
	return Quaternion{
		W: q1.W*q2.W - q1.X*q2.X - q1.Y*q2.Y - q1.Z*q2.Z,
		X: q1.W*q2.X + q1.X*q2.W + q1.Y*q2.Z - q1.Z*q2.Y,
		Y: q1.W*q2.Y - q1.X*q2.Z + q1.Y*q2.W + q1.Z*q2.X,
		Z: q1.W*q2.Z + q1.X*q2.Y - q1.Y*q2.X + q1.Z*q2.W,
	}
}

func normalizeQuaternion(q Quaternion) Quaternion {
	mag := math.Sqrt(q.W*q.W + q.X*q.X + q.Y*q.Y + q.Z*q.Z)
	if mag < 0.0001 {
		return Quaternion{W: 1, X: 0, Y: 0, Z: 0}
	}
	return Quaternion{W: q.W / mag, X: q.X / mag, Y: q.Y / mag, Z: q.Z / mag}
}

func clampf(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
