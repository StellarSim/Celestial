package ship

import (
	"celestial/internal/config"
	"math"
	"sync"
)

type Ship struct {
	mu sync.RWMutex

	ID       string
	ClassID  string
	Name     string
	IsPlayer bool

	Position        Vector3
	Velocity        Vector3
	Rotation        Quaternion
	AngularVelocity Vector3

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
	ship := &Ship{
		ID:              id,
		ClassID:         classID,
		Name:            name,
		IsPlayer:        isPlayer,
		Position:        Vector3{0, 0, 0},
		Velocity:        Vector3{0, 0, 0},
		Rotation:        Quaternion{1, 0, 0, 0},
		AngularVelocity: Vector3{0, 0, 0},
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
		ship.Engines[engCfg.ID] = &Engine{
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
		ship.Weapons[wpnCfg.ID] = &Weapon{
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
			AmmoCapacity: wpnCfg.AmmoCapacity,
			AmmoCount:    wpnCfg.AmmoCapacity,
		}
	}

	ship.Shields = &ShieldSystem{
		Emitters:     make(map[string]*ShieldEmitter),
		RechargeRate: class.Shields.RechargeRate,
		PowerDraw:    class.Shields.PowerDraw,
		Enabled:      true,
	}
	for _, emCfg := range class.Shields.Emitters {
		ship.Shields.Emitters[emCfg.ID] = &ShieldEmitter{
			ID:          emCfg.ID,
			Facing:      emCfg.Facing,
			MaxStrength: emCfg.Strength,
			Strength:    emCfg.Strength,
			MaxHealth:   emCfg.Health,
			Health:      emCfg.Health,
		}
	}

	ship.Hull = &HullSystem{
		Sections: make(map[string]*HullSection),
	}
	for _, secCfg := range class.Hull.Sections {
		ship.Hull.Sections[secCfg.ID] = &HullSection{
			ID:        secCfg.ID,
			MaxArmor:  secCfg.Armor,
			Armor:     secCfg.Armor,
			MaxHealth: secCfg.Health,
			Health:    secCfg.Health,
		}
	}

	for _, subCfg := range class.Subsystems {
		ship.Subsystems[subCfg.ID] = &Subsystem{
			ID:        subCfg.ID,
			Type:      subCfg.Type,
			MaxHealth: subCfg.Health,
			Health:    subCfg.Health,
			Enabled:   true,
			PowerDraw: subCfg.PowerDraw,
		}
	}

	for _, bayCfg := range class.LaunchBays {
		ship.LaunchBays[bayCfg.ID] = &LaunchBay{
			ID:        bayCfg.ID,
			Capacity:  bayCfg.Capacity,
			Current:   bayCfg.Capacity,
			MaxHealth: bayCfg.Health,
			Health:    bayCfg.Health,
		}
	}

	ship.Power = &PowerSystem{
		MaxCapacity:     10000,
		CurrentCapacity: 10000,
		Generation:      1000,
		Consumption:     0,
		Breakers:        make(map[string]*Breaker),
	}

	ship.LifeSupport = &LifeSupportSystem{
		Compartments: make(map[string]*Compartment),
	}
	compartmentNames := []string{"bridge", "engineering", "weapons_bay", "crew_quarters", "cargo_bay"}
	for _, name := range compartmentNames {
		ship.LifeSupport.Compartments[name] = &Compartment{
			ID:          name,
			MaxPressure: 101.3,
			Pressure:    101.3,
			MaxOxygen:   21.0,
			Oxygen:      21.0,
			Temperature: 20.0,
		}
	}

	if isPlayer {
		roles := []string{"engineer", "flight", "weapons", "captain", "comms", "operations", "relay", "first_officer"}
		for _, role := range roles {
			ship.Crew[role] = &CrewMember{
				Role:   role,
				Health: 100.0,
				Status: "healthy",
			}
		}
	}

	return ship
}

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
	totalThrust := Vector3{0, 0, 0}
	for _, engine := range s.Engines {
		if engine.Enabled && engine.Health > 0 {
			thrustFactor := engine.Health / engine.MaxHealth
			thrust := engine.Thrust * thrustFactor
			totalThrust.Z += thrust
		}
	}

	forward := s.getForwardVector()
	thrustWorld := Vector3{
		X: forward.X * totalThrust.Z,
		Y: forward.Y * totalThrust.Z,
		Z: forward.Z * totalThrust.Z,
	}

	accel := Vector3{
		X: thrustWorld.X / s.Mass,
		Y: thrustWorld.Y / s.Mass,
		Z: thrustWorld.Z / s.Mass,
	}

	s.Velocity.X += accel.X * dt
	s.Velocity.Y += accel.Y * dt
	s.Velocity.Z += accel.Z * dt

	drag := 0.98
	s.Velocity.X *= drag
	s.Velocity.Y *= drag
	s.Velocity.Z *= drag

	speed := math.Sqrt(s.Velocity.X*s.Velocity.X + s.Velocity.Y*s.Velocity.Y + s.Velocity.Z*s.Velocity.Z)
	if speed > s.MaxSpeed {
		scale := s.MaxSpeed / speed
		s.Velocity.X *= scale
		s.Velocity.Y *= scale
		s.Velocity.Z *= scale
	}

	s.Position.X += s.Velocity.X * dt
	s.Position.Y += s.Velocity.Y * dt
	s.Position.Z += s.Velocity.Z * dt

	rotDrag := 0.95
	s.AngularVelocity.X *= rotDrag
	s.AngularVelocity.Y *= rotDrag
	s.AngularVelocity.Z *= rotDrag

	angle := math.Sqrt(s.AngularVelocity.X*s.AngularVelocity.X+
		s.AngularVelocity.Y*s.AngularVelocity.Y+
		s.AngularVelocity.Z*s.AngularVelocity.Z) * dt
	if angle > 0.001 {
		axis := Vector3{
			X: s.AngularVelocity.X / angle,
			Y: s.AngularVelocity.Y / angle,
			Z: s.AngularVelocity.Z / angle,
		}
		deltaQ := axisAngleToQuaternion(axis, angle)
		s.Rotation = multiplyQuaternions(deltaQ, s.Rotation)
		s.Rotation = normalizeQuaternion(s.Rotation)
	}
}

func (s *Ship) updatePower(dt float64) {
	consumption := 0.0
	for _, engine := range s.Engines {
		if engine.Enabled {
			consumption += engine.PowerDraw
		}
	}
	for _, weapon := range s.Weapons {
		if weapon.Enabled {
			consumption += weapon.PowerDraw
		}
	}
	if s.Shields.Enabled {
		consumption += s.Shields.PowerDraw
	}
	for _, subsystem := range s.Subsystems {
		if subsystem.Enabled {
			consumption += subsystem.PowerDraw
		}
	}

	s.Power.Consumption = consumption
	s.Power.CurrentCapacity += (s.Power.Generation - consumption) * dt

	if s.Power.CurrentCapacity > s.Power.MaxCapacity {
		s.Power.CurrentCapacity = s.Power.MaxCapacity
	}
	if s.Power.CurrentCapacity < 0 {
		s.Power.CurrentCapacity = 0
	}
}

func (s *Ship) updateShields(dt float64) {
	if !s.Shields.Enabled {
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
			if section.Health < 0 {
				section.Health = 0
			}
		}
	}

	for _, subsystem := range s.Subsystems {
		if subsystem.OnFire {
			subsystem.Health -= 5.0 * dt
			if subsystem.Health < 0 {
				subsystem.Health = 0
			}
		}
	}
}

func (s *Ship) updateLifeSupport(dt float64) {
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

func (s *Ship) getForwardVector() Vector3 {
	return Vector3{
		X: 2 * (s.Rotation.X*s.Rotation.Z + s.Rotation.W*s.Rotation.Y),
		Y: 2 * (s.Rotation.Y*s.Rotation.Z - s.Rotation.W*s.Rotation.X),
		Z: 1 - 2*(s.Rotation.X*s.Rotation.X+s.Rotation.Y*s.Rotation.Y),
	}
}

func (s *Ship) ApplyThrust(x, y, z float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Applied via input, actual thrust calculated in updatePhysics
}

func (s *Ship) ApplyRotation(pitch, yaw, roll float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.AngularVelocity.X += pitch * s.TurnRate
	s.AngularVelocity.Y += yaw * s.TurnRate
	s.AngularVelocity.Z += roll * s.TurnRate
}

func (s *Ship) FireWeapon(weaponID string, targetID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	weapon, ok := s.Weapons[weaponID]
	if !ok || weapon.Health <= 0 || weapon.Cooldown > 0 {
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

func (s *Ship) TakeDamage(amount float64, location string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if location == "" {
		location = "forward"
	}

	emitter, hasEmitter := s.Shields.Emitters[location]
	if hasEmitter && emitter.Strength > 0 {
		emitter.Strength -= amount
		if emitter.Strength < 0 {
			overflow := -emitter.Strength
			emitter.Strength = 0
			amount = overflow
		} else {
			return
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
		if section.Health <= 0 {
			section.Health = 0
			section.Breached = true
		}
	}
}

func axisAngleToQuaternion(axis Vector3, angle float64) Quaternion {
	halfAngle := angle * 0.5
	s := math.Sin(halfAngle)
	return Quaternion{
		W: math.Cos(halfAngle),
		X: axis.X * s,
		Y: axis.Y * s,
		Z: axis.Z * s,
	}
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
		return Quaternion{1, 0, 0, 0}
	}
	return Quaternion{
		W: q.W / mag,
		X: q.X / mag,
		Y: q.Y / mag,
		Z: q.Z / mag,
	}
}
