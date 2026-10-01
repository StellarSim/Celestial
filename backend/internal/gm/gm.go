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

	// All mutations go through the locked Ship mutators. Never write through
	// raw subsystem pointers: broadcasts read hull state under Ship.Clone().
	amount, hasAmount := toFloatAmount(value)
	switch systemType {
	case "hull", "damage":
		section := systemID
		if section == "" || section == "hull" || section == "damage" {
			section = ship.SectionForward
		}
		if !hasAmount {
			log.Printf("GM: modify hull on %s needs a numeric value, got %v", shipID, value)
			return
		}
		if amount < 0 {
			sh.TakeDamage(-amount, section)
		} else {
			sh.RepairSection(section, amount)
		}
	case "shields":
		if enabled, ok := value.(bool); ok {
			sh.SetShieldsEnabled(enabled)
		} else {
			log.Printf("GM: modify shields on %s needs a bool, got %v", shipID, value)
			return
		}
	case "alert":
		if level, ok := value.(string); ok {
			sh.SetAlertLevel(level)
		} else {
			log.Printf("GM: modify alert on %s needs a level string, got %v", shipID, value)
			return
		}
	default:
		log.Printf("GM: unsupported modify target %s.%s = %v on ship %s", systemType, systemID, value, shipID)
		return
	}

	log.Printf("GM: Modified %s.%s.%s = %v on ship %s", systemType, systemID, property, value, shipID)
}

func toFloatAmount(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
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

func (c *Controller) GetActiveMission() *mission.Mission {
	return c.missionEngine.GetActiveMission()
}

// GetMissionIDs lists loaded missions for the state_update broadcast.
func (c *Controller) GetMissionIDs() []string {
	if c.missionEngine == nil {
		return []string{}
	}
	return c.missionEngine.GetMissionIDs()
}

// GetActiveMissionID returns the running mission id, or "" when idle.
func (c *Controller) GetActiveMissionID() string {
	if c.missionEngine == nil {
		return ""
	}
	return c.missionEngine.GetActiveMissionID()
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
