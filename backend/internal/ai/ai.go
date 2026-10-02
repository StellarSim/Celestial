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
func (c *Controller) Clone() *Controller {
	return &Controller{
		State:           c.State,
		TargetID:        c.TargetID,
		Difficulty:      c.Difficulty,
		AggressionLevel: c.AggressionLevel,
		TacticalMode:    c.TacticalMode,
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
	// Drift on a steady heading, wings level.
	sh.ApplyThrust(0, 0, 0.3)
	sh.SetLocalRotationRate(0, 0, 0)

	threat := c.findNearestThreat(sh, allShips)
	if threat != nil {
		dist := distance(sh.Position, threat.Position)
		if dist < engagementRange {
			c.State = "combat"
			c.TargetID = threat.ID
			log.Printf("AI ship %s entering combat with %s", sh.ID, threat.ID)
		}
	}
}

func (c *Controller) updateCombat(sh *ship.Ship, allShips map[string]*ship.Ship) {
	target := allShips[c.TargetID]
	if target == nil {
		c.State = "patrol"
		c.TargetID = ""
		return
	}

	dist := distance(sh.Position, target.Position)

	toTarget := ship.Vector3{
		X: target.Position.X - sh.Position.X,
		Y: target.Position.Y - sh.Position.Y,
		Z: target.Position.Z - sh.Position.Z,
	}
	toTarget = normalize(toTarget)

	// Use the ship's orientation, not its position.
	forward := sh.Forward()
	right := sh.Right()
	up := sh.Up()
	dot := toTarget.X*forward.X + toTarget.Y*forward.Y + toTarget.Z*forward.Z

	// Proportional steering toward the target in local space, commanded as a
	// rate so it settles on the target instead of overshooting forever.
	yawErr := 0.0
	pitchErr := 0.0
	if dot < 0.98 {
		yawErr = clamp(toTarget.X*right.X+toTarget.Y*right.Y+toTarget.Z*right.Z, -1, 1)
		pitchErr = clamp(-(toTarget.X*up.X+toTarget.Y*up.Y+toTarget.Z*up.Z), -1, 1)
	}
	sh.SetLocalRotationRate(pitchErr*c.Difficulty*0.5, yawErr*c.Difficulty*0.5, 0)

	switch {
	case dist > optimalRange*1.5:
		sh.ApplyThrust(0, 0, 0.8)
	case dist < optimalRange*0.5:
		sh.ApplyThrust(0, 0, -0.5)
	default:
		sh.ApplyThrust(0, 0, 0.3)
	}

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

	away := ship.Vector3{
		X: sh.Position.X - target.Position.X,
		Y: sh.Position.Y - target.Position.Y,
		Z: sh.Position.Z - target.Position.Z,
	}
	away = normalize(away)

	sh.ApplyThrust(0, 0, 1.0)
	fwd := sh.Forward()
	rgt := sh.Right()
	upV := sh.Up()
	dot := away.X*fwd.X + away.Y*fwd.Y + away.Z*fwd.Z
	yawErr := 0.0
	pitchErr := 0.0
	if dot < 0.98 {
		yawErr = clamp(away.X*rgt.X+away.Y*rgt.Y+away.Z*rgt.Z, -1, 1)
		pitchErr = clamp(-(away.X*upV.X+away.Y*upV.Y+away.Z*upV.Z), -1, 1)
	}
	sh.SetLocalRotationRate(pitchErr*0.5, yawErr*0.5, 0)

	if distance(sh.Position, target.Position) > optimalRange {
		c.State = "combat"
	}
}

func (c *Controller) updateRetreat(sh *ship.Ship, allShips map[string]*ship.Ship) {
	sh.ApplyThrust(0, 0, 1.0)
	sh.SetLocalRotationRate(0, 0, 0)

	dist := math.MaxFloat64
	if c.TargetID != "" {
		if target := allShips[c.TargetID]; target != nil {
			dist = distance(sh.Position, target.Position)
		}
	}

	if dist > retreatTrigger {
		c.State = "patrol"
		c.TargetID = ""
		log.Printf("AI ship %s ending retreat", sh.ID)
	}
}

func (c *Controller) evaluateState(sh *ship.Ship, allShips map[string]*ship.Ship) {
	hullHealth := c.calculateHullHealth(sh)
	shieldHealth := c.calculateShieldHealth(sh)

	if hullHealth < 0.3 || shieldHealth < 0.2 {
		if c.State != "retreat" {
			c.State = "retreat"
		}
		return
	}

	if hullHealth < 0.6 && shieldHealth < 0.5 && c.State == "combat" {
		c.State = "evade"
	}
}

// attemptPhaserFire fires a phaser within range and applies damage.
func (c *Controller) attemptPhaserFire(sh *ship.Ship, target *ship.Ship) {
	for id, weapon := range sh.WeaponsSnapshot() {
		if weapon.Type != "phaser" || weapon.Health <= 0 || weapon.Cooldown > 0 {
			continue
		}
		if weapon.Range > 0 && distance(sh.Position, target.Position) > weapon.Range {
			continue
		}
		if !sh.FireWeapon(id, target.ID) {
			return
		}
		facing := target.FacingFor(sh.Position)
		target.ApplyTypedDamage(weapon.Damage*c.Difficulty, facing, "energy")
		if c.spawner != nil {
			c.spawner.SpawnPhaserBeam(sh, target)
		}
		return
	}
}

// attemptTorpedoFire consumes torpedo ammo and spawns a real projectile.
func (c *Controller) attemptTorpedoFire(sh *ship.Ship, target *ship.Ship) {
	for id, weapon := range sh.WeaponsSnapshot() {
		if weapon.Type != "torpedo" || weapon.Health <= 0 || weapon.Cooldown > 0 || weapon.AmmoCount <= 0 {
			continue
		}
		if weapon.Range > 0 && distance(sh.Position, target.Position) > weapon.Range {
			continue
		}
		// AI crews keep their tubes armed, loaded and locked on the target.
		_ = sh.MutateWeapon(id, func(w *ship.Weapon) error {
			w.Armed = true
			if w.AmmoCount > 0 {
				w.Loaded = true
			}
			w.Locked = true
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

func (c *Controller) findNearestThreat(sh *ship.Ship, allShips map[string]*ship.Ship) *ship.Ship {
	var nearest *ship.Ship
	minDist := math.MaxFloat64

	for _, other := range allShips {
		if other.ID == sh.ID {
			continue
		}
		if sh.IsPlayer == other.IsPlayer {
			continue
		}
		dist := distance(sh.Position, other.Position)
		if dist < minDist {
			minDist = dist
			nearest = other
		}
	}

	return nearest
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
		return 0.0
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
