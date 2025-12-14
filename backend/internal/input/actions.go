package input

import (
	"celestial/internal/ship"
	"celestial/internal/simulation"
	"fmt"
	"log"
)

type Action struct {
	Role   string
	System string
	Action string
	Value  interface{}
}

type ActionRouter struct {
	simulator *simulation.Simulator
	handlers  map[string]ActionHandler
}

type ActionHandler func(action *Action) error

func NewActionRouter(sim *simulation.Simulator) *ActionRouter {
	router := &ActionRouter{
		simulator: sim,
		handlers:  make(map[string]ActionHandler),
	}

	router.registerHandlers()
	return router
}

func (ar *ActionRouter) registerHandlers() {
	ar.handlers["engineer.power.toggle_breaker"] = ar.handleToggleBreaker
	ar.handlers["engineer.damage.repair"] = ar.handleRepair
	ar.handlers["engineer.damage.extinguish_fire"] = ar.handleExtinguishFire
	ar.handlers["engineer.damage.seal_breach"] = ar.handleSealBreach

	ar.handlers["flight.thrust.set"] = ar.handleSetThrust
	ar.handlers["flight.rotation.set"] = ar.handleSetRotation
	ar.handlers["flight.docking.release"] = ar.handleReleaseDocking

	ar.handlers["weapons.torpedo.arm"] = ar.handleArmTorpedo
	ar.handlers["weapons.torpedo.load"] = ar.handleLoadTorpedo
	ar.handlers["weapons.torpedo.lock"] = ar.handleLockTorpedo
	ar.handlers["weapons.torpedo.fire"] = ar.handleFireTorpedo
	ar.handlers["weapons.phaser.fire"] = ar.handleFirePhaser
	ar.handlers["weapons.target.set"] = ar.handleSetTarget

	ar.handlers["captain.alert.set"] = ar.handleSetAlert
	ar.handlers["captain.order.issue"] = ar.handleIssueOrder

	ar.handlers["comms.hail.send"] = ar.handleSendHail
	ar.handlers["comms.message.send"] = ar.handleSendMessage

	ar.handlers["operations.power.route"] = ar.handleRoutePower
	ar.handlers["operations.shields.toggle"] = ar.handleToggleShields

	ar.handlers["relay.scan.initiate"] = ar.handleInitiateScan
	ar.handlers["relay.sensors.set_mode"] = ar.handleSetSensorMode

	ar.handlers["first_officer.system.toggle"] = ar.handleToggleSystem
}

func (ar *ActionRouter) RouteAction(action *Action) error {
	key := fmt.Sprintf("%s.%s.%s", action.Role, action.System, action.Action)
	handler, ok := ar.handlers[key]
	if !ok {
		return fmt.Errorf("no handler for action: %s", key)
	}

	log.Printf("Routing action: %s (value: %v)", key, action.Value)
	return handler(action)
}

func (ar *ActionRouter) handleToggleBreaker(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	enabled, ok := action.Value.(bool)
	if !ok {
		return fmt.Errorf("invalid value type for toggle_breaker")
	}

	breakerID := action.System
	breaker, ok := playerShip.Power.Breakers[breakerID]
	if !ok {
		return fmt.Errorf("breaker not found: %s", breakerID)
	}

	breaker.Enabled = enabled
	log.Printf("Breaker %s set to %v", breakerID, enabled)
	return nil
}

func (ar *ActionRouter) handleRepair(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	log.Printf("Repair initiated on system: %s", action.System)
	return nil
}

func (ar *ActionRouter) handleExtinguishFire(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	location := action.System
	section, ok := playerShip.Hull.Sections[location]
	if ok {
		section.OnFire = false
	}

	comp, ok := playerShip.LifeSupport.Compartments[location]
	if ok {
		comp.OnFire = false
	}

	log.Printf("Fire extinguished in: %s", location)
	return nil
}

func (ar *ActionRouter) handleSealBreach(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	location := action.System
	comp, ok := playerShip.LifeSupport.Compartments[location]
	if ok {
		comp.Breached = false
	}

	section, ok := playerShip.Hull.Sections[location]
	if ok {
		section.Breached = false
	}

	log.Printf("Breach sealed in: %s", location)
	return nil
}

func (ar *ActionRouter) handleSetThrust(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	thrust, ok := action.Value.(float64)
	if !ok {
		return fmt.Errorf("invalid thrust value")
	}

	playerShip.ApplyThrust(0, 0, thrust)
	return nil
}

func (ar *ActionRouter) handleSetRotation(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	rotData, ok := action.Value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid rotation value")
	}

	pitch, _ := rotData["pitch"].(float64)
	yaw, _ := rotData["yaw"].(float64)
	roll, _ := rotData["roll"].(float64)

	playerShip.ApplyRotation(pitch, yaw, roll)
	return nil
}

func (ar *ActionRouter) handleReleaseDocking(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	playerShip.Docked = false
	log.Println("Docking clamps released")
	return nil
}

func (ar *ActionRouter) handleArmTorpedo(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	weaponID := action.System
	weapon, ok := playerShip.Weapons[weaponID]
	if !ok {
		return fmt.Errorf("weapon not found: %s", weaponID)
	}

	armed, ok := action.Value.(bool)
	if !ok {
		return fmt.Errorf("invalid arm value")
	}

	weapon.Armed = armed
	log.Printf("Torpedo %s armed: %v", weaponID, armed)
	return nil
}

func (ar *ActionRouter) handleLoadTorpedo(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	weaponID := action.System
	weapon, ok := playerShip.Weapons[weaponID]
	if !ok {
		return fmt.Errorf("weapon not found: %s", weaponID)
	}

	if weapon.AmmoCount <= 0 {
		return fmt.Errorf("no torpedoes remaining")
	}

	weapon.Loaded = true
	log.Printf("Torpedo %s loaded", weaponID)
	return nil
}

func (ar *ActionRouter) handleLockTorpedo(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	weaponID := action.System
	weapon, ok := playerShip.Weapons[weaponID]
	if !ok {
		return fmt.Errorf("weapon not found: %s", weaponID)
	}

	locked, ok := action.Value.(bool)
	if !ok {
		return fmt.Errorf("invalid lock value")
	}

	weapon.Locked = locked
	log.Printf("Torpedo %s locked: %v", weaponID, locked)
	return nil
}

func (ar *ActionRouter) handleFireTorpedo(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	weaponID := action.System
	weapon, ok := playerShip.Weapons[weaponID]
	if !ok {
		return fmt.Errorf("weapon not found: %s", weaponID)
	}

	if !weapon.Armed || !weapon.Loaded || !weapon.Locked {
		return fmt.Errorf("torpedo not ready to fire")
	}

	if weapon.Cooldown > 0 {
		return fmt.Errorf("torpedo on cooldown")
	}

	targetID := playerShip.TargetID
	if targetID == "" {
		return fmt.Errorf("no target set")
	}

	if playerShip.FireWeapon(weaponID, targetID) {
		forward := playerShip.Position
		velocity := playerShip.Velocity
		velocity.X += 500
		velocity.Y += 0
		velocity.Z += 0

		ar.simulator.SpawnProjectile(
			fmt.Sprintf("torpedo_%s_%.0f", weaponID, ar.simulator.CurrentTime),
			"torpedo",
			playerShip.ID,
			targetID,
			forward,
			velocity,
			weapon.Damage,
		)

		log.Printf("Fired torpedo %s at target %s", weaponID, targetID)
		return nil
	}

	return fmt.Errorf("failed to fire torpedo")
}

func (ar *ActionRouter) handleFirePhaser(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	weaponID := action.System
	weapon, ok := playerShip.Weapons[weaponID]
	if !ok {
		return fmt.Errorf("weapon not found: %s", weaponID)
	}

	if weapon.Cooldown > 0 {
		return fmt.Errorf("phaser on cooldown")
	}

	targetID := playerShip.TargetID
	if targetID == "" {
		return fmt.Errorf("no target set")
	}

	target := ar.simulator.GetShip(targetID)
	if target == nil {
		return fmt.Errorf("target not found")
	}

	if playerShip.FireWeapon(weaponID, targetID) {
		target.TakeDamage(weapon.Damage, "forward")
		log.Printf("Fired phaser %s at target %s for %.1f damage", weaponID, targetID, weapon.Damage)
		return nil
	}

	return fmt.Errorf("failed to fire phaser")
}

func (ar *ActionRouter) handleSetTarget(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	targetID, ok := action.Value.(string)
	if !ok {
		return fmt.Errorf("invalid target ID")
	}

	playerShip.TargetID = targetID
	log.Printf("Target set to: %s", targetID)
	return nil
}

func (ar *ActionRouter) handleSetAlert(action *Action) error {
	alertLevel, ok := action.Value.(string)
	if !ok {
		return fmt.Errorf("invalid alert level")
	}

	log.Printf("Alert level set to: %s", alertLevel)
	return nil
}

func (ar *ActionRouter) handleIssueOrder(action *Action) error {
	order, ok := action.Value.(string)
	if !ok {
		return fmt.Errorf("invalid order")
	}

	log.Printf("Captain issued order: %s", order)
	return nil
}

func (ar *ActionRouter) handleSendHail(action *Action) error {
	targetID, ok := action.Value.(string)
	if !ok {
		return fmt.Errorf("invalid target ID")
	}

	log.Printf("Hailing target: %s", targetID)
	return nil
}

func (ar *ActionRouter) handleSendMessage(action *Action) error {
	message, ok := action.Value.(string)
	if !ok {
		return fmt.Errorf("invalid message")
	}

	log.Printf("Sending message: %s", message)
	return nil
}

func (ar *ActionRouter) handleRoutePower(action *Action) error {
	log.Printf("Routing power to: %s", action.System)
	return nil
}

func (ar *ActionRouter) handleToggleShields(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	enabled, ok := action.Value.(bool)
	if !ok {
		return fmt.Errorf("invalid shield toggle value")
	}

	playerShip.Shields.Enabled = enabled
	log.Printf("Shields set to: %v", enabled)
	return nil
}

func (ar *ActionRouter) handleInitiateScan(action *Action) error {
	targetID, ok := action.Value.(string)
	if !ok {
		return fmt.Errorf("invalid scan target")
	}

	log.Printf("Initiating scan of: %s", targetID)
	return nil
}

func (ar *ActionRouter) handleSetSensorMode(action *Action) error {
	mode, ok := action.Value.(string)
	if !ok {
		return fmt.Errorf("invalid sensor mode")
	}

	log.Printf("Sensor mode set to: %s", mode)
	return nil
}

func (ar *ActionRouter) handleToggleSystem(action *Action) error {
	playerShip := ar.getPlayerShip()
	if playerShip == nil {
		return fmt.Errorf("no player ship found")
	}

	enabled, ok := action.Value.(bool)
	if !ok {
		return fmt.Errorf("invalid toggle value")
	}

	systemID := action.System
	subsystem, ok := playerShip.Subsystems[systemID]
	if ok {
		subsystem.Enabled = enabled
		log.Printf("Subsystem %s set to: %v", systemID, enabled)
	}

	return nil
}

func (ar *ActionRouter) getPlayerShip() *ship.Ship {
	ships := ar.simulator.GetAllShips()
	for _, sh := range ships {
		if sh.IsPlayer {
			return sh
		}
	}
	return nil
}
