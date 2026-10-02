package ai

import (
	"celestial/internal/ship"
	"log"
	"math"
	"math/rand"
)

// Combat range constants (sim meters).
const (
	engagementRange = 5000.0
	sensorRange     = 15000.0
	weaponFireRange = 2000.0
	optimalRange    = 1500.0
	retreatTrigger  = 8000.0
	closeEvadeRange = 600.0
)

type Controller struct {
	State           string
	TargetID        string
	Difficulty      float64
	AggressionLevel float64
	TacticalMode    string

	spawner ProjectileSpawner

	// Hostile reports whether faction a treats faction b as hostile.
	// The simulator wires this to the faction yaml; when nil the legacy
	// rule applies.
	Hostile func(a, b string) bool
}

// ProjectileSpawner lets the AI launch real projectiles through the simulator.
type ProjectileSpawner interface {
	SpawnTorpedo(shooter *ship.Ship, target *ship.Ship, weaponID string)
	SpawnPhaserBeam(shooter *ship.Ship, target *ship.Ship)
}

func NewController() *Controller {
	return &Controller{
		State:           "patrol",
		Difficulty:      1.0,
		AggressionLevel: 0.5,
		TacticalMode:    "balanced",
	}
}

// Clone returns a deep copy so snapshot restore does not alias live controllers.
// The spawner points at the live simulator, so it is carried over: a
// restored controller must still be able to launch projectiles.
func (c *Controller) Clone() *Controller {
	return &Controller{
		State:           c.State,
		TargetID:        c.TargetID,
		Difficulty:      c.Difficulty,
		AggressionLevel: c.AggressionLevel,
		TacticalMode:    c.TacticalMode,
		spawner:         c.spawner,
		Hostile:         c.Hostile,
	}
}

// OrderAttack forces the controller onto a target regardless of range or
// faction. Missions use it for scripted attacks (for example a pirate told
// to hit a merchant).
func (c *Controller) OrderAttack(targetID string) {
	c.TargetID = targetID
	if targetID != "" {
		c.State = "combat"
	} else if c.State == "combat" {
		c.State = "patrol"
	}
}

func (c *Controller) Update(dt float64, sh *ship.Ship, allShips map[string]*ship.Ship) {
	switch c.State {
	case "patrol":
		c.updatePatrol(sh, allShips)
	case "combat":
		c.updateCombat(sh, allShips)
	case "evade":
		c.updateEvade(sh, allShips)
	case "retreat":
		c.updateRetreat(sh, allShips)
	}

	c.evaluateState(sh, allShips)
}

func (c *Controller) updatePatrol(sh *ship.Ship, allShips map[string]*ship.Ship) {
	threat := c.findNearestThreat(sh, allShips, sensorRange)
	if threat == nil {
		// No contacts: hold position instead of flying off to infinity.
		sh.ApplyThrust(0, 0, 0)
		sh.SetLocalRotationRate(0, 0, 0)
		return
	}

	dist := distance(sh.GetPosition(), threat.GetPosition())
	if dist < engagementRange {
		c.State = "combat"
		c.TargetID = threat.ID
		log.Printf("AI ship %s entering combat with %s", sh.ID, threat.ID)
		return
	}

	// Contact beyond guns but on sensors: close the distance so the ship
	// intercepts instead of cruising past on a fixed heading.
	c.steerToward(sh, threat.GetPosition(), 0.5*c.Difficulty)
	sh.ApplyThrust(0, 0, 0.8)
}

func (c *Controller) updateCombat(sh *ship.Ship, allShips map[string]*ship.Ship) {
	target := allShips[c.TargetID]
	if target == nil || !c.isHostileTo(sh, target) {
		c.State = "patrol"
		c.TargetID = ""
		return
	}

	selfPos := sh.GetPosition()
	targetPos := target.GetPosition()
	dist := distance(selfPos, targetPos)

	c.steerToward(sh, targetPos, 0.5*c.Difficulty)

	switch {
	case dist > optimalRange*1.5:
		sh.ApplyThrust(0, 0, 0.8)
	case dist < optimalRange*0.5:
		sh.ApplyThrust(0, 0, -0.5)
	default:
		sh.ApplyThrust(0, 0, 0.3)
	}

	forward := sh.Forward()
	toTarget := normalize(ship.Vector3{
		X: targetPos.X - selfPos.X,
		Y: targetPos.Y - selfPos.Y,
		Z: targetPos.Z - selfPos.Z,
	})
	dot := toTarget.X*forward.X + toTarget.Y*forward.Y + toTarget.Z*forward.Z

	if dot > 0.95 && dist < weaponFireRange {
		c.attemptPhaserFire(sh, target)
	}

	if rand.Float64() < 0.02*c.AggressionLevel && dist < weaponFireRange {
		c.attemptTorpedoFire(sh, target)
	}
}

func (c *Controller) updateEvade(sh *ship.Ship, allShips map[string]*ship.Ship) {
	target := allShips[c.TargetID]
	if target == nil {
		c.State = "patrol"
		c.TargetID = ""
		return
	}

	selfPos := sh.GetPosition()
	targetPos := target.GetPosition()
	away := normalize(ship.Vector3{
		X: selfPos.X - targetPos.X,
		Y: selfPos.Y - targetPos.Y,
		Z: selfPos.Z - targetPos.Z,
	})
	awayPoint := ship.Vector3{X: selfPos.X + away.X*1000, Y: selfPos.Y + away.Y*1000, Z: selfPos.Z + away.Z*1000}
	c.steerToward(sh, awayPoint, 0.5)
	sh.ApplyThrust(0, 0, 1.0)

	if distance(selfPos, targetPos) > optimalRange {
		c.State = "combat"
	}
}

func (c *Controller) updateRetreat(sh *ship.Ship, allShips map[string]*ship.Ship) {
	target := allShips[c.TargetID]

	if target != nil {
		// Burn away from the threat, not along whatever heading we had.
		selfPos := sh.GetPosition()
		targetPos := target.GetPosition()
		away := normalize(ship.Vector3{
			X: selfPos.X - targetPos.X,
			Y: selfPos.Y - targetPos.Y,
			Z: selfPos.Z - targetPos.Z,
		})
		awayPoint := ship.Vector3{X: selfPos.X + away.X*1000, Y: selfPos.Y + away.Y*1000, Z: selfPos.Z + away.Z*1000}
		c.steerToward(sh, awayPoint, 0.5)
	} else {
		sh.SetLocalRotationRate(0, 0, 0)
	}
	sh.ApplyThrust(0, 0, 1.0)

	dist := math.MaxFloat64
	if target != nil {
		dist = distance(sh.GetPosition(), target.GetPosition())
	}

	if dist > retreatTrigger && c.isRecovered(sh) {
		c.State = "patrol"
		c.TargetID = ""
		log.Printf("AI ship %s ending retreat", sh.ID)
	}
}

func (c *Controller) evaluateState(sh *ship.Ship, allShips map[string]*ship.Ship) {
	if c.isCriticalDamage(sh) {
		if c.State != "retreat" {
			c.State = "retreat"
			log.Printf("AI ship %s starting retreat", sh.ID)
		}
		return
	}

	hullHealth := c.calculateHullHealth(sh)
	shieldHealth := c.calculateShieldHealth(sh)
	if hullHealth < 0.6 && shieldHealth < 0.5 && c.State == "combat" {
		c.State = "evade"
	}
}

// isCriticalDamage reports whether the ship is hurt enough to retreat.
func (c *Controller) isCriticalDamage(sh *ship.Ship) bool {
	return c.calculateHullHealth(sh) < 0.3 || c.calculateShieldHealth(sh) < 0.2
}

// isRecovered reports whether a retreating ship is healthy enough to leave
// retreat. Thresholds are higher than isCriticalDamage to add hysteresis and
// stop retreat exit and re-entry from flip-flopping every tick.
func (c *Controller) isRecovered(sh *ship.Ship) bool {
	return c.calculateHullHealth(sh) >= 0.4 && c.calculateShieldHealth(sh) >= 0.3
}

// attemptPhaserFire fires a phaser within range and applies damage.
func (c *Controller) attemptPhaserFire(sh *ship.Ship, target *ship.Ship) {
	selfPos := sh.GetPosition()
	for id, weapon := range sh.WeaponsSnapshot() {
		if weapon.Type != "phaser" || weapon.Health <= 0 || weapon.Cooldown > 0 {
			continue
		}
		if weapon.Range > 0 && distance(selfPos, target.GetPosition()) > weapon.Range {
			continue
		}
		if !sh.FireWeapon(id, target.ID) {
			return
		}
		facing := target.FacingFor(selfPos)
		target.ApplyTypedDamage(weapon.Damage*c.Difficulty, facing, "energy")
		if c.spawner != nil {
			c.spawner.SpawnPhaserBeam(sh, target)
		}
		return
	}
}

// attemptTorpedoFire consumes torpedo ammo and spawns a real projectile.
func (c *Controller) attemptTorpedoFire(sh *ship.Ship, target *ship.Ship) {
	selfPos := sh.GetPosition()
	for id, weapon := range sh.WeaponsSnapshot() {
		if weapon.Type != "torpedo" || weapon.Health <= 0 || weapon.Cooldown > 0 || weapon.AmmoCount <= 0 {
			continue
		}
		if weapon.Range > 0 && distance(selfPos, target.GetPosition()) > weapon.Range {
			continue
		}
		// AI crews keep their tubes armed and loaded.
		_ = sh.MutateWeapon(id, func(w *ship.Weapon) error {
			w.Armed = true
			if w.AmmoCount > 0 {
				w.Loaded = true
			}
			return nil
		})
		if !sh.FireWeapon(id, target.ID) {
			return
		}
		if c.spawner != nil {
			c.spawner.SpawnTorpedo(sh, target, id)
		}
		return
	}
}

func (c *Controller) findNearestThreat(sh *ship.Ship, allShips map[string]*ship.Ship, maxRange float64) *ship.Ship {
	var nearest *ship.Ship
	minDist := math.MaxFloat64

	selfPos := sh.GetPosition()
	for _, other := range allShips {
		if other.ID == sh.ID {
			continue
		}
		if !c.isHostileTo(sh, other) {
			continue
		}
		dist := distance(selfPos, other.GetPosition())
		if dist < minDist && dist <= maxRange {
			minDist = dist
			nearest = other
		}
	}

	return nearest
}

// isHostileTo reports whether other is a valid target for sh. The faction
// yaml is authoritative through the Hostile hook; the legacy tag comparison
// only applies when no hook is wired (tests) and IsPlayer only when a
// faction was never assigned.
func (c *Controller) isHostileTo(sh, other *ship.Ship) bool {
	fa := sh.GetFaction()
	fb := other.GetFaction()
	if c.Hostile != nil {
		return c.Hostile(fa, fb)
	}
	if fa != "" && fb != "" {
		return fa != fb
	}
	return sh.IsPlayer != other.IsPlayer
}

// steerToward turns the ship toward a world-space point. Gain scales the
// demanded turn rate, so callers pass smaller gains for gentle intercepts.
func (c *Controller) steerToward(sh *ship.Ship, point ship.Vector3, gain float64) {
	selfPos := sh.GetPosition()
	toTarget := normalize(ship.Vector3{
		X: point.X - selfPos.X,
		Y: point.Y - selfPos.Y,
		Z: point.Z - selfPos.Z,
	})

	forward := sh.Forward()
	right := sh.Right()
	up := sh.Up()
	dot := toTarget.X*forward.X + toTarget.Y*forward.Y + toTarget.Z*forward.Z

	yawErr := 0.0
	pitchErr := 0.0
	if dot < 0.98 {
		// Positive yaw demand spins about +up, which carries forward away
		// from +right, so yaw runs opposite the right-dot. Positive pitch
		// demand spins about +right, which carries forward toward +up.
		yawErr = clamp(-(toTarget.X*right.X+toTarget.Y*right.Y+toTarget.Z*right.Z), -1, 1)
		pitchErr = clamp(toTarget.X*up.X+toTarget.Y*up.Y+toTarget.Z*up.Z, -1, 1)
	}
	sh.SetLocalRotationRate(pitchErr*gain, yawErr*gain, 0)
}

func (c *Controller) calculateHullHealth(sh *ship.Ship) float64 {
	total, max := 0.0, 0.0
	for _, section := range sh.HullSnapshot() {
		total += section.Health
		max += section.MaxHealth
	}
	if max == 0 {
		return 1.0
	}
	return total / max
}

func (c *Controller) calculateShieldHealth(sh *ship.Ship) float64 {
	total, max := 0.0, 0.0
	for _, emitter := range sh.ShieldsSnapshot() {
		total += emitter.Strength
		max += emitter.MaxStrength
	}
	if max == 0 {
		return 1.0
	}
	return total / max
}

func (c *Controller) SetDifficulty(diff float64) {
	c.Difficulty = diff
}

func (c *Controller) SetTacticalMode(mode string) {
	c.TacticalMode = mode
	switch mode {
	case "aggressive":
		c.AggressionLevel = 1.0
	case "defensive":
		c.AggressionLevel = 0.2
	case "balanced":
		c.AggressionLevel = 0.5
	}
}

// SetSpawner wires the projectile spawner used for AI torpedo fire.
func (c *Controller) SetSpawner(s ProjectileSpawner) {
	c.spawner = s
}

func distance(a, b ship.Vector3) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func normalize(v ship.Vector3) ship.Vector3 {
	mag := math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
	if mag < 0.0001 {
		return ship.Vector3{X: 0, Y: 0, Z: -1}
	}
	return ship.Vector3{X: v.X / mag, Y: v.Y / mag, Z: v.Z / mag}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
