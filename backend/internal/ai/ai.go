package ai

import (
	"celestial/internal/ship"
	"log"
	"math"
	"math/rand"
)

type Controller struct {
	State           string
	TargetID        string
	Difficulty      float64
	AggressionLevel float64
	TacticalMode    string
}

func NewController() *Controller {
	return &Controller{
		State:           "patrol",
		Difficulty:      1.0,
		AggressionLevel: 0.5,
		TacticalMode:    "balanced",
	}
}

func (c *Controller) Update(dt float64, sh *ship.Ship, allShips map[string]*ship.Ship) {
	switch c.State {
	case "patrol":
		c.updatePatrol(dt, sh, allShips)
	case "combat":
		c.updateCombat(dt, sh, allShips)
	case "evade":
		c.updateEvade(dt, sh, allShips)
	case "retreat":
		c.updateRetreat(dt, sh, allShips)
	}

	c.evaluateState(sh, allShips)
}

func (c *Controller) updatePatrol(dt float64, sh *ship.Ship, allShips map[string]*ship.Ship) {
	sh.ApplyThrust(0, 0, sh.MaxSpeed*0.3)

	yawRate := 0.1 * c.Difficulty
	sh.ApplyRotation(0, yawRate, 0)

	threat := c.findNearestThreat(sh, allShips)
	if threat != nil {
		dist := distance(sh.Position, threat.Position)
		if dist < 5000.0 {
			c.State = "combat"
			c.TargetID = threat.ID
			log.Printf("AI ship %s entering combat with %s", sh.ID, threat.ID)
		}
	}
}

func (c *Controller) updateCombat(dt float64, sh *ship.Ship, allShips map[string]*ship.Ship) {
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

	forward := sh.Position
	dot := toTarget.X*forward.X + toTarget.Y*forward.Y + toTarget.Z*forward.Z

	turnRate := c.Difficulty * 0.5
	if dot < 0.9 {
		sh.ApplyRotation(toTarget.Y*turnRate, toTarget.X*turnRate, 0)
	}

	optimalRange := 1000.0
	if dist > optimalRange*1.5 {
		sh.ApplyThrust(0, 0, sh.MaxSpeed*0.8)
	} else if dist < optimalRange*0.5 {
		sh.ApplyThrust(0, 0, -sh.MaxSpeed*0.5)
	} else {
		sh.ApplyThrust(0, 0, sh.MaxSpeed*0.3)
	}

	if dot > 0.95 && dist < 2000.0 {
		c.attemptFire(sh, target)
	}

	if rand.Float64() < 0.1*c.AggressionLevel {
		c.attemptMissilefire(sh, target)
	}
}

func (c *Controller) updateEvade(dt float64, sh *ship.Ship, allShips map[string]*ship.Ship) {
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

	sh.ApplyThrust(0, 0, sh.MaxSpeed)
	sh.ApplyRotation(away.Y*0.5, away.X*0.5, rand.Float64()*0.2-0.1)

	if rand.Float64() < 0.3 {
		c.State = "combat"
	}
}

func (c *Controller) updateRetreat(dt float64, sh *ship.Ship, allShips map[string]*ship.Ship) {
	sh.ApplyThrust(0, 0, sh.MaxSpeed)

	dist := 10000.0
	if c.TargetID != "" {
		if target := allShips[c.TargetID]; target != nil {
			dist = distance(sh.Position, target.Position)
		}
	}

	if dist > 8000.0 {
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
			log.Printf("AI ship %s retreating (hull: %.1f%%, shields: %.1f%%)", sh.ID, hullHealth*100, shieldHealth*100)
		}
		return
	}

	if hullHealth < 0.6 && shieldHealth < 0.5 {
		if c.State == "combat" && rand.Float64() < 0.3 {
			c.State = "evade"
			log.Printf("AI ship %s evading", sh.ID)
		}
	}
}

func (c *Controller) attemptFire(sh *ship.Ship, target *ship.Ship) {
	for id, weapon := range sh.Weapons {
		if weapon.Type == "phaser" && weapon.Health > 0 && weapon.Cooldown <= 0 {
			if sh.FireWeapon(id, target.ID) {
				target.TakeDamage(weapon.Damage*c.Difficulty, "forward")
				log.Printf("AI ship %s fired %s at %s for %.1f damage", sh.ID, id, target.ID, weapon.Damage*c.Difficulty)
				return
			}
		}
	}
}

func (c *Controller) attemptMissilefire(sh *ship.Ship, target *ship.Ship) {
	for id, weapon := range sh.Weapons {
		if weapon.Type == "torpedo" && weapon.Health > 0 && weapon.Cooldown <= 0 && weapon.AmmoCount > 0 {
			weapon.Armed = true
			weapon.Loaded = true
			weapon.Locked = true
			if sh.FireWeapon(id, target.ID) {
				log.Printf("AI ship %s fired torpedo %s at %s", sh.ID, id, target.ID)
				return
			}
		}
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
	total := 0.0
	max := 0.0
	for _, section := range sh.Hull.Sections {
		total += section.Health
		max += section.MaxHealth
	}
	if max == 0 {
		return 1.0
	}
	return total / max
}

func (c *Controller) calculateShieldHealth(sh *ship.Ship) float64 {
	total := 0.0
	max := 0.0
	for _, emitter := range sh.Shields.Emitters {
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
	log.Printf("AI difficulty set to %.2f", diff)
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
	log.Printf("AI tactical mode set to %s", mode)
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
		return ship.Vector3{0, 0, 1}
	}
	return ship.Vector3{
		X: v.X / mag,
		Y: v.Y / mag,
		Z: v.Z / mag,
	}
}
