package damage

import (
	"celestial/internal/ship"
	"log"
	"math/rand"
)

type DamageController struct {
	Ship *ship.Ship
}

func NewDamageController(sh *ship.Ship) *DamageController {
	return &DamageController{
		Ship: sh,
	}
}

func (dc *DamageController) ApplyDamage(amount float64, location string, damageType string) {
	switch damageType {
	case "kinetic":
		dc.applyKineticDamage(amount, location)
	case "energy":
		dc.applyEnergyDamage(amount, location)
	case "explosive":
		dc.applyExplosiveDamage(amount, location)
	default:
		dc.Ship.TakeDamage(amount, location)
	}

	dc.checkCascadingFailures(location)
}

func (dc *DamageController) applyKineticDamage(amount float64, location string) {
	dc.Ship.TakeDamage(amount, location)

	if rand.Float64() < 0.3 {
		dc.startFire(location)
	}
}

func (dc *DamageController) applyEnergyDamage(amount float64, location string) {
	dc.Ship.TakeDamage(amount*1.5, location)

	if rand.Float64() < 0.2 {
		dc.causeSystemOverload(location)
	}
}

func (dc *DamageController) applyExplosiveDamage(amount float64, location string) {
	dc.Ship.TakeDamage(amount, location)

	adjacentLocations := dc.getAdjacentLocations(location)
	for _, loc := range adjacentLocations {
		dc.Ship.TakeDamage(amount*0.5, loc)
	}

	if rand.Float64() < 0.5 {
		dc.startFire(location)
	}
}

func (dc *DamageController) startFire(location string) {
	dc.Ship.Hull.Sections[location].OnFire = true
	log.Printf("Fire started in %s on ship %s", location, dc.Ship.ID)

	comp, ok := dc.Ship.LifeSupport.Compartments[location]
	if ok {
		comp.OnFire = true
	}
}

func (dc *DamageController) ExtinguishFire(location string) {
	section, ok := dc.Ship.Hull.Sections[location]
	if ok {
		section.OnFire = false
	}

	comp, ok := dc.Ship.LifeSupport.Compartments[location]
	if ok {
		comp.OnFire = false
	}

	log.Printf("Fire extinguished in %s on ship %s", location, dc.Ship.ID)
}

func (dc *DamageController) RepairSystem(systemType, systemID string, amount float64) {
	switch systemType {
	case "engine":
		if engine, ok := dc.Ship.Engines[systemID]; ok {
			engine.Health += amount
			if engine.Health > engine.MaxHealth {
				engine.Health = engine.MaxHealth
			}
			log.Printf("Repaired engine %s on ship %s (health: %.1f)", systemID, dc.Ship.ID, engine.Health)
		}
	case "weapon":
		if weapon, ok := dc.Ship.Weapons[systemID]; ok {
			weapon.Health += amount
			if weapon.Health > weapon.MaxHealth {
				weapon.Health = weapon.MaxHealth
			}
			log.Printf("Repaired weapon %s on ship %s (health: %.1f)", systemID, dc.Ship.ID, weapon.Health)
		}
	case "shield":
		if emitter, ok := dc.Ship.Shields.Emitters[systemID]; ok {
			emitter.Health += amount
			if emitter.Health > emitter.MaxHealth {
				emitter.Health = emitter.MaxHealth
			}
			log.Printf("Repaired shield emitter %s on ship %s (health: %.1f)", systemID, dc.Ship.ID, emitter.Health)
		}
	case "hull":
		if section, ok := dc.Ship.Hull.Sections[systemID]; ok {
			section.Health += amount
			if section.Health > section.MaxHealth {
				section.Health = section.MaxHealth
			}
			if section.Health > 0 {
				section.Breached = false
			}
			log.Printf("Repaired hull section %s on ship %s (health: %.1f)", systemID, dc.Ship.ID, section.Health)
		}
	case "subsystem":
		if subsystem, ok := dc.Ship.Subsystems[systemID]; ok {
			subsystem.Health += amount
			if subsystem.Health > subsystem.MaxHealth {
				subsystem.Health = subsystem.MaxHealth
			}
			log.Printf("Repaired subsystem %s on ship %s (health: %.1f)", systemID, dc.Ship.ID, subsystem.Health)
		}
	}
}

func (dc *DamageController) causeSystemOverload(location string) {
	for _, subsystem := range dc.Ship.Subsystems {
		if rand.Float64() < 0.1 {
			subsystem.Health -= 20.0
			if subsystem.Health < 0 {
				subsystem.Health = 0
				subsystem.Enabled = false
			}
			log.Printf("System overload damaged %s on ship %s", subsystem.ID, dc.Ship.ID)
		}
	}
}

func (dc *DamageController) checkCascadingFailures(location string) {
	section, ok := dc.Ship.Hull.Sections[location]
	if !ok {
		return
	}

	if section.Health <= 0 && !section.Breached {
		section.Breached = true
		log.Printf("Hull breach in %s on ship %s", location, dc.Ship.ID)

		comp, ok := dc.Ship.LifeSupport.Compartments[location]
		if ok {
			comp.Breached = true
		}

		for _, adjacentLoc := range dc.getAdjacentLocations(location) {
			if adjComp, ok := dc.Ship.LifeSupport.Compartments[adjacentLoc]; ok {
				if rand.Float64() < 0.3 {
					adjComp.Pressure -= 20.0
					log.Printf("Pressure drop in adjacent compartment %s", adjacentLoc)
				}
			}
		}
	}

	if section.OnFire {
		for _, adjacentLoc := range dc.getAdjacentLocations(location) {
			if rand.Float64() < 0.1 {
				dc.startFire(adjacentLoc)
			}
		}
	}
}

func (dc *DamageController) getAdjacentLocations(location string) []string {
	adjacencyMap := map[string][]string{
		"forward":     {"port", "starboard", "bridge"},
		"aft":         {"port", "starboard", "engineering"},
		"port":        {"forward", "aft"},
		"starboard":   {"forward", "aft"},
		"dorsal":      {"forward", "aft"},
		"ventral":     {"forward", "aft"},
		"bridge":      {"forward"},
		"engineering": {"aft"},
		"weapons_bay": {"forward"},
	}

	adjacent, ok := adjacencyMap[location]
	if !ok {
		return []string{}
	}
	return adjacent
}

func (dc *DamageController) SealBreach(location string) {
	comp, ok := dc.Ship.LifeSupport.Compartments[location]
	if ok {
		comp.Breached = false
		log.Printf("Sealed breach in %s on ship %s", location, dc.Ship.ID)
	}

	section, ok := dc.Ship.Hull.Sections[location]
	if ok {
		section.Breached = false
	}
}

func (dc *DamageController) RestorePressure(location string) {
	comp, ok := dc.Ship.LifeSupport.Compartments[location]
	if !ok {
		return
	}

	if !comp.Breached {
		comp.Pressure = comp.MaxPressure
		comp.Oxygen = comp.MaxOxygen
		log.Printf("Restored pressure in %s on ship %s", location, dc.Ship.ID)
	}
}
