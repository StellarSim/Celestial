package gm

import (
	"celestial/internal/mission"
	"celestial/internal/ship"
	"celestial/internal/simulation"
	"log"
	"time"
)

type Controller struct {
	simulator      *simulation.Simulator
	missionEngine  *mission.Engine
	snapshotTicker *time.Ticker
	stopChan       chan struct{}
}

func NewController(sim *simulation.Simulator, missionEng *mission.Engine) *Controller {
	ctrl := &Controller{
		simulator:     sim,
		missionEngine: missionEng,
		stopChan:      make(chan struct{}),
	}

	ctrl.startSnapshotLoop()
	return ctrl
}

func (c *Controller) startSnapshotLoop() {
	c.snapshotTicker = time.NewTicker(20 * time.Second)
	go func() {
		for {
			select {
			case <-c.stopChan:
				return
			case <-c.snapshotTicker.C:
				c.simulator.CreateSnapshot()
			}
		}
	}()
}

func (c *Controller) Stop() {
	if c.snapshotTicker != nil {
		c.snapshotTicker.Stop()
	}
	close(c.stopChan)
}

func (c *Controller) Pause() {
	c.simulator.Pause()
	log.Println("GM: Simulation paused")
}

func (c *Controller) Resume() {
	c.simulator.Resume()
	log.Println("GM: Simulation resumed")
}

func (c *Controller) CreateSnapshot() {
	c.simulator.CreateSnapshot()
	log.Println("GM: Manual snapshot created")
}

func (c *Controller) RestoreSnapshot(index int) error {
	err := c.simulator.RestoreSnapshot(index)
	if err != nil {
		log.Printf("GM: Failed to restore snapshot: %v", err)
		return err
	}
	log.Printf("GM: Restored snapshot %d", index)
	return nil
}

func (c *Controller) GetSnapshots() []*simulation.Snapshot {
	return c.simulator.Snapshots
}

func (c *Controller) SpawnShip(id, classID, name string, isPlayer bool, position ship.Vector3) error {
	err := c.simulator.SpawnShip(id, classID, name, isPlayer, position)
	if err != nil {
		log.Printf("GM: Failed to spawn ship: %v", err)
		return err
	}
	log.Printf("GM: Spawned ship %s (%s)", name, classID)
	return nil
}

func (c *Controller) RemoveShip(id string) {
	c.simulator.RemoveShip(id)
	log.Printf("GM: Removed ship %s", id)
}

func (c *Controller) ModifyShipSystem(shipID, systemType, systemID string, property string, value interface{}) {
	sh := c.simulator.GetShip(shipID)
	if sh == nil {
		log.Printf("GM: Ship not found: %s", shipID)
		return
	}

	switch systemType {
	case "engine":
		if engine, ok := sh.Engines[systemID]; ok {
			c.applyProperty(engine, property, value)
		}
	case "weapon":
		if weapon, ok := sh.Weapons[systemID]; ok {
			c.applyProperty(weapon, property, value)
		}
	case "shield":
		if emitter, ok := sh.Shields.Emitters[systemID]; ok {
			c.applyProperty(emitter, property, value)
		}
	case "hull":
		if section, ok := sh.Hull.Sections[systemID]; ok {
			c.applyProperty(section, property, value)
		}
	}

	log.Printf("GM: Modified %s.%s.%s = %v on ship %s", systemType, systemID, property, value, shipID)
}

func (c *Controller) applyProperty(obj interface{}, property string, value interface{}) {
	// Simplified property setting
	log.Printf("Applying property %s = %v", property, value)
}

func (c *Controller) DamageShip(shipID string, amount float64, location string) {
	sh := c.simulator.GetShip(shipID)
	if sh == nil {
		log.Printf("GM: Ship not found: %s", shipID)
		return
	}

	sh.TakeDamage(amount, location)
	log.Printf("GM: Applied %.1f damage to %s at location %s", amount, shipID, location)
}

func (c *Controller) StartMission(missionID string) error {
	err := c.missionEngine.StartMission(missionID)
	if err != nil {
		log.Printf("GM: Failed to start mission: %v", err)
		return err
	}
	log.Printf("GM: Started mission %s", missionID)
	return nil
}

func (c *Controller) StopMission() {
	c.missionEngine.StopMission()
	log.Println("GM: Stopped current mission")
}

func (c *Controller) TriggerEvent(eventName string, params map[string]interface{}) {
	c.missionEngine.TriggerEvent(eventName, params)
	log.Printf("GM: Triggered event %s", eventName)
}

func (c *Controller) GetSimulationState() map[string]interface{} {
	ships := c.simulator.GetAllShips()
	shipData := make(map[string]interface{})

	for id, sh := range ships {
		shipData[id] = map[string]interface{}{
			"id":       sh.ID,
			"name":     sh.Name,
			"class":    sh.ClassID,
			"position": sh.Position,
			"velocity": sh.Velocity,
			"health":   c.getShipHealth(sh),
		}
	}

	activeMission := c.missionEngine.GetActiveMission()
	var missionData interface{}
	if activeMission != nil {
		missionData = map[string]interface{}{
			"id":         activeMission.ID,
			"name":       activeMission.Name,
			"objectives": activeMission.Objectives,
		}
	}

	return map[string]interface{}{
		"time":           c.simulator.CurrentTime,
		"ships":          shipData,
		"active_mission": missionData,
		"snapshot_count": len(c.simulator.Snapshots),
	}
}

func (c *Controller) getShipHealth(sh *ship.Ship) map[string]float64 {
	totalHull := 0.0
	maxHull := 0.0
	for _, section := range sh.Hull.Sections {
		totalHull += section.Health
		maxHull += section.MaxHealth
	}

	totalShields := 0.0
	maxShields := 0.0
	for _, emitter := range sh.Shields.Emitters {
		totalShields += emitter.Strength
		maxShields += emitter.MaxStrength
	}

	hullPercent := 0.0
	if maxHull > 0 {
		hullPercent = (totalHull / maxHull) * 100
	}

	shieldPercent := 0.0
	if maxShields > 0 {
		shieldPercent = (totalShields / maxShields) * 100
	}

	return map[string]float64{
		"hull":    hullPercent,
		"shields": shieldPercent,
	}
}

func (c *Controller) SetAIDifficulty(shipID string, difficulty float64) {
	if controller, ok := c.simulator.AIControllers[shipID]; ok {
		controller.SetDifficulty(difficulty)
		log.Printf("GM: Set AI difficulty for %s to %.2f", shipID, difficulty)
	}
}

func (c *Controller) SetAITacticalMode(shipID string, mode string) {
	if controller, ok := c.simulator.AIControllers[shipID]; ok {
		controller.SetTacticalMode(mode)
		log.Printf("GM: Set AI tactical mode for %s to %s", shipID, mode)
	}
}
