package simulation

import (
	"celestial/internal/ai"
	"celestial/internal/config"
	"celestial/internal/ship"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

const projectileHitRadius = 50.0

type Simulator struct {
	mu sync.RWMutex

	tickRate int
	dt       float64
	running  bool
	paused   bool
	stopChan chan struct{}

	Ships       map[string]*ship.Ship
	Projectiles map[string]*Projectile
	Objects     map[string]*Object

	ShipClasses map[string]*config.ShipClass

	AIControllers map[string]*ai.Controller

	CurrentTime   float64
	Snapshots     []*Snapshot
	SnapshotIndex int

	AlertLevel string

	// Event bookkeeping so notifications fire once per occurrence.
	criticalReported map[string]bool
	waypointHits     map[string]bool

	// OnTick runs after each tick with the sim lock released, so subscribers can
	// read simulator state without deadlocking.
	OnTick func(dt float64)

	// OnEvent emits simulation events (ship_destroyed, waypoint_reached, ...)
	// to subscribers such as the mission engine. It is always called with the
	// sim lock released so handlers can freely use the simulator.
	OnEvent func(event string, data map[string]interface{})

	pendingEvents []pendingEvent
}

type pendingEvent struct {
	name string
	data map[string]interface{}
}

type Projectile struct {
	ID          string
	Type        string
	Position    ship.Vector3
	Velocity    ship.Vector3
	Damage      float64
	SourceID    string
	TargetID    string
	Lifetime    float64
	MaxLifetime float64
}

type Object struct {
	ID       string
	Type     string
	Position ship.Vector3
	Velocity ship.Vector3
	Rotation ship.Quaternion
	Data     map[string]interface{}
}

type Snapshot struct {
	Time          float64
	Ships         map[string]*ship.Ship
	Projectiles   map[string]*Projectile
	Objects       map[string]*Object
	AIControllers map[string]*ai.Controller
}

func NewSimulator(tickRate int, shipClasses map[string]*config.ShipClass) *Simulator {
	return &Simulator{
		tickRate:         tickRate,
		dt:               1.0 / float64(tickRate),
		Ships:            make(map[string]*ship.Ship),
		Projectiles:      make(map[string]*Projectile),
		Objects:          make(map[string]*Object),
		ShipClasses:      shipClasses,
		AIControllers:    make(map[string]*ai.Controller),
		stopChan:         make(chan struct{}),
		Snapshots:        make([]*Snapshot, 0),
		AlertLevel:       "normal",
		criticalReported: make(map[string]bool),
		waypointHits:     make(map[string]bool),
	}
}

func (s *Simulator) Start() {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	ticker := time.NewTicker(time.Duration(1000/s.tickRate) * time.Millisecond)
	defer ticker.Stop()

	log.Println("Simulator started")

	for {
		select {
		case <-s.stopChan:
			log.Println("Simulator stopped")
			return
		case <-ticker.C:
			if s.IsPaused() {
				continue
			}
			s.Tick()
		}
	}
}

func (s *Simulator) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		close(s.stopChan)
		s.running = false
	}
}

// Pause stops the sim without blocking, so GM commands never stall.
func (s *Simulator) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.paused {
		s.paused = true
		log.Println("Simulator paused")
	}
}

// Resume restarts the sim.
func (s *Simulator) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paused {
		s.paused = false
		log.Println("Simulator resumed")
	}
}

func (s *Simulator) Tick() {
	s.mu.Lock()
	s.tickLocked()
	events := s.pendingEvents
	s.pendingEvents = nil
	s.mu.Unlock()

	// Hooks and events run without the sim lock so they may use the simulator.
	if s.OnTick != nil {
		s.OnTick(s.dt)
	}
	for _, ev := range events {
		if s.OnEvent != nil {
			s.OnEvent(ev.name, ev.data)
		}
	}
}

func (s *Simulator) tickLocked() {
	s.CurrentTime += s.dt

	for id, sh := range s.Ships {
		sh.Update(s.dt)
		s.checkShipEvents(id, sh)
	}

	s.updateProjectiles()
	s.updateAI()
	s.checkCollisions()
	s.checkWaypoints()
}

// Hull fraction below which a ship is reported as critically damaged.
const criticalHullFraction = 0.25

func (s *Simulator) checkShipEvents(id string, sh *ship.Ship) {
	total, max := shipTotals(sh)
	if max <= 0 {
		return
	}
	health := total / max
	if health <= 0 {
		delete(s.Ships, id)
		delete(s.AIControllers, id)
		s.emitEvent("ship_destroyed", map[string]interface{}{"ship_id": id})
		return
	}
	if health <= criticalHullFraction && !s.criticalReported[id] {
		s.criticalReported[id] = true
		s.emitEvent("damage_critical", map[string]interface{}{
			"ship_id": id, "health": health,
		})
	}
}

func shipTotals(sh *ship.Ship) (total float64, max float64) {
	for _, section := range sh.HullSnapshot() {
		total += section.Health
		max += section.MaxHealth
	}
	return total, max
}

// waypointArrivalDistance is how close a ship must be to register a waypoint.
const waypointArrivalDistance = 400.0

func (s *Simulator) checkWaypoints() {
	if len(s.Objects) == 0 {
		return
	}
	for id, obj := range s.Objects {
		if obj.Type != "waypoint" {
			continue
		}
		for shipID, sh := range s.Ships {
			if distance(sh.Position, obj.Position) > waypointArrivalDistance {
				continue
			}
			key := id + ":" + shipID
			if s.waypointHits[key] {
				continue
			}
			s.waypointHits[key] = true
			s.emitEvent("waypoint_reached", map[string]interface{}{
				"waypoint": id, "ship_id": shipID,
			})
		}
	}
}

// emitEvent queues an event; it is dispatched with the lock released.
func (s *Simulator) emitEvent(event string, data map[string]interface{}) {
	s.pendingEvents = append(s.pendingEvents, pendingEvent{name: event, data: data})
}

func (s *Simulator) updateProjectiles() {
	toDelete := make([]string, 0)

	for id, proj := range s.Projectiles {
		proj.Position.X += proj.Velocity.X * s.dt
		proj.Position.Y += proj.Velocity.Y * s.dt
		proj.Position.Z += proj.Velocity.Z * s.dt

		proj.Lifetime += s.dt
		if proj.Lifetime > proj.MaxLifetime {
			toDelete = append(toDelete, id)
			continue
		}

		if proj.TargetID != "" {
			target, ok := s.Ships[proj.TargetID]
			if ok {
				dist := distance(proj.Position, target.Position)
				if dist < projectileHitRadius {
					// Phaser beams are visuals only; their damage was
					// resolved instantly at fire time.
					if proj.Type != "phaser" {
						facing := target.FacingFor(proj.Position)
						target.ApplyTypedDamage(proj.Damage, facing, "explosive")
					}
					toDelete = append(toDelete, id)
				}
			}
		}
	}

	for _, id := range toDelete {
		delete(s.Projectiles, id)
	}
}

func (s *Simulator) updateAI() {
	for shipID, controller := range s.AIControllers {
		sh, ok := s.Ships[shipID]
		if !ok {
			continue
		}
		controller.Update(s.dt, sh, s.Ships)
	}
}

const collisionRadius = 60.0

// checkCollisions applies a single separation impulse and one-time damage when
// two ships overlap.
func (s *Simulator) checkCollisions() {
	ships := make([]*ship.Ship, 0, len(s.Ships))
	for _, sh := range s.Ships {
		ships = append(ships, sh)
	}

	for i := 0; i < len(ships); i++ {
		for j := i + 1; j < len(ships); j++ {
			a, b := ships[i], ships[j]
			dist := distance(a.Position, b.Position)
			if dist >= collisionRadius || dist < 0.0001 {
				continue
			}

			// Push apart along the contact normal.
			nx := (b.Position.X - a.Position.X) / dist
			ny := (b.Position.Y - a.Position.Y) / dist
			nz := (b.Position.Z - a.Position.Z) / dist
			overlap := (collisionRadius - dist) * 0.5

			a.Position.X -= nx * overlap
			a.Position.Y -= ny * overlap
			a.Position.Z -= nz * overlap
			b.Position.X += nx * overlap
			b.Position.Y += ny * overlap
			b.Position.Z += nz * overlap

			a.ApplyTypedDamage(10.0, b.FacingFor(a.Position), "kinetic")
			b.ApplyTypedDamage(10.0, a.FacingFor(b.Position), "kinetic")
		}
	}
}

func (s *Simulator) SpawnShip(id, classID, name string, isPlayer bool, position ship.Vector3) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	class, ok := s.ShipClasses[classID]
	if !ok {
		return fmt.Errorf("unknown ship class: %s", classID)
	}

	sh := ship.NewShip(id, classID, name, class, isPlayer)
	sh.Position = position
	s.Ships[id] = sh

	if !isPlayer {
		controller := ai.NewController()
		// AI runs inside the tick, so its spawner must not re-take the sim lock.
		controller.SetSpawner(tickSpawner{s})
		s.AIControllers[id] = controller
	}

	log.Printf("Spawned ship: %s (%s) at position (%.1f, %.1f, %.1f)", name, classID, position.X, position.Y, position.Z)
	return nil
}

func (s *Simulator) RemoveShip(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeShipLocked(id)
}

func (s *Simulator) removeShipLocked(id string) {
	delete(s.Ships, id)
	delete(s.AIControllers, id)
	delete(s.criticalReported, id)
	log.Printf("Removed ship: %s", id)
}

func (s *Simulator) SpawnProjectile(id, projType, sourceID, targetID string, position, velocity ship.Vector3, damage float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	proj := &Projectile{
		ID:          id,
		Type:        projType,
		Position:    position,
		Velocity:    velocity,
		Damage:      damage,
		SourceID:    sourceID,
		TargetID:    targetID,
		Lifetime:    0,
		MaxLifetime: 10.0,
	}

	s.Projectiles[id] = proj
	log.Printf("Spawned projectile: %s from %s to %s", id, sourceID, targetID)
}

// tickSpawner lets the AI launch projectiles from inside the tick, where the
// simulator lock is already held.
type tickSpawner struct {
	sim *Simulator
}

func (t tickSpawner) SpawnTorpedo(shooter *ship.Ship, target *ship.Ship, weaponID string) {
	t.sim.spawnTorpedoLocked(shooter, target, weaponID)
}

func (t tickSpawner) SpawnPhaserBeam(shooter *ship.Ship, target *ship.Ship) {
	t.sim.spawnPhaserLocked(shooter, target)
}

// SpawnTorpedo launches a torpedo projectile from shooter toward target.
func (s *Simulator) SpawnTorpedo(shooter *ship.Ship, target *ship.Ship, weaponID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spawnTorpedoLocked(shooter, target, weaponID)
}

// SpawnPhaserBeam publishes a transient "phaser" projectile so clients can
// render the beam. Phaser damage is resolved instantly at fire time, so the
// projectile carries no damage and the tick never applies any for it.
func (s *Simulator) SpawnPhaserBeam(shooter *ship.Ship, target *ship.Ship) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spawnPhaserLocked(shooter, target)
}

// spawnPhaserLocked must be called with s.mu held.
func (s *Simulator) spawnPhaserLocked(shooter *ship.Ship, target *ship.Ship) {
	fwd := shooter.Forward()
	origin := shooter.GetPosition()
	launchPos := ship.Vector3{
		X: origin.X + fwd.X*50,
		Y: origin.Y + fwd.Y*50,
		Z: origin.Z + fwd.Z*50,
	}

	targetPos := target.GetPosition()
	toTarget := ship.Vector3{
		X: targetPos.X - launchPos.X,
		Y: targetPos.Y - launchPos.Y,
		Z: targetPos.Z - launchPos.Z,
	}
	dist := math.Sqrt(toTarget.X*toTarget.X + toTarget.Y*toTarget.Y + toTarget.Z*toTarget.Z)
	if dist < 0.0001 {
		return
	}
	dir := ship.Vector3{X: toTarget.X / dist, Y: toTarget.Y / dist, Z: toTarget.Z / dist}

	const beamSpeed = 6000.0
	velocity := ship.Vector3{X: dir.X * beamSpeed, Y: dir.Y * beamSpeed, Z: dir.Z * beamSpeed}

	id := fmt.Sprintf("phaser_%s_%.0f", shooter.ID, s.CurrentTime)
	s.Projectiles[id] = &Projectile{
		ID:          id,
		Type:        "phaser",
		Position:    launchPos,
		Velocity:    velocity,
		Damage:      0,
		SourceID:    shooter.ID,
		TargetID:    target.ID,
		MaxLifetime: 1.0,
	}
}

// spawnTorpedoLocked must be called with s.mu held.
func (s *Simulator) spawnTorpedoLocked(shooter *ship.Ship, target *ship.Ship, weaponID string) {
	weapons := shooter.WeaponsSnapshot()
	weapon, ok := weapons[weaponID]
	if !ok {
		return
	}

	fwd := shooter.Forward()
	origin := shooter.GetPosition()
	launchPos := ship.Vector3{
		X: origin.X + fwd.X*50,
		Y: origin.Y + fwd.Y*50,
		Z: origin.Z + fwd.Z*50,
	}

	toTarget := ship.Vector3{
		X: target.Position.X - launchPos.X,
		Y: target.Position.Y - launchPos.Y,
		Z: target.Position.Z - launchPos.Z,
	}
	dist := math.Sqrt(toTarget.X*toTarget.X + toTarget.Y*toTarget.Y + toTarget.Z*toTarget.Z)
	if dist < 0.0001 {
		return
	}
	dir := ship.Vector3{X: toTarget.X / dist, Y: toTarget.Y / dist, Z: toTarget.Z / dist}

	const torpedoSpeed = 400.0
	velocity := ship.Vector3{X: dir.X * torpedoSpeed, Y: dir.Y * torpedoSpeed, Z: dir.Z * torpedoSpeed}

	id := fmt.Sprintf("torpedo_%s_%.0f", weaponID, s.CurrentTime)
	s.Projectiles[id] = &Projectile{
		ID:          id,
		Type:        "torpedo",
		Position:    launchPos,
		Velocity:    velocity,
		Damage:      weapon.Damage,
		SourceID:    shooter.ID,
		TargetID:    target.ID,
		MaxLifetime: 10.0,
	}
}

func (s *Simulator) SpawnObject(id, objType string, position ship.Vector3) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.waypointHits, id+":player")
	obj := &Object{
		ID:       id,
		Type:     objType,
		Position: position,
		Velocity: ship.Vector3{X: 0, Y: 0, Z: 0},
		Rotation: ship.Quaternion{W: 1, X: 0, Y: 0, Z: 0},
		Data:     make(map[string]interface{}),
	}

	s.Objects[id] = obj
	log.Printf("Spawned object: %s (%s) at position (%.1f, %.1f, %.1f)", id, objType, position.X, position.Y, position.Z)
}

func (s *Simulator) RemoveObject(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Objects, id)
	for key := range s.waypointHits {
		if len(key) > len(id) && key[:len(id)+1] == id+":" {
			delete(s.waypointHits, key)
		}
	}
	log.Printf("Removed object: %s", id)
}

// Moves a ship instantly (GM move tool).
func (s *Simulator) TeleportShip(id string, pos ship.Vector3) error {
	sh := s.GetShip(id)
	if sh == nil {
		return fmt.Errorf("ship not found: %s", id)
	}
	sh.SetPosition(pos)
	return nil
}

// Returns a copy of the object map for broadcasts.
func (s *Simulator) GetAllObjects() map[string]*Object {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*Object, len(s.Objects))
	for k, v := range s.Objects {
		out[k] = v
	}
	return out
}

// GetObject returns a copy of an object by id.
func (s *Simulator) GetObject(id string) (Object, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	obj, ok := s.Objects[id]
	if !ok {
		return Object{}, false
	}
	return *obj, true
}

func (s *Simulator) GetShip(id string) *ship.Ship {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Ships[id]
}

func (s *Simulator) GetAllShips() map[string]*ship.Ship {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ships := make(map[string]*ship.Ship)
	for k, v := range s.Ships {
		ships[k] = v
	}
	return ships
}

func (s *Simulator) GetAllProjectiles() map[string]*Projectile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]*Projectile, len(s.Projectiles))
	for k, v := range s.Projectiles {
		out[k] = v
	}
	return out
}

func (s *Simulator) GetCurrentTime() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CurrentTime
}

func (s *Simulator) IsPaused() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.paused
}

func (s *Simulator) GetAlertLevel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.AlertLevel == "" {
		return "normal"
	}
	return s.AlertLevel
}

func (s *Simulator) SetAlertLevel(level string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AlertLevel = level
}

func (s *Simulator) CreateSnapshot() {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := &Snapshot{
		Time:          s.CurrentTime,
		Ships:         s.copyShips(),
		Projectiles:   s.copyProjectiles(),
		Objects:       s.copyObjects(),
		AIControllers: copyAIControllers(s.AIControllers),
	}

	s.Snapshots = append(s.Snapshots, snapshot)
	log.Printf("Created snapshot at time %.2f (total: %d)", s.CurrentTime, len(s.Snapshots))
}

func (s *Simulator) RestoreSnapshot(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(s.Snapshots) {
		return fmt.Errorf("invalid snapshot index: %d", index)
	}

	snapshot := s.Snapshots[index]
	s.CurrentTime = snapshot.Time
	// Deep-copy on restore so post-restore ticks never mutate the stored
	// snapshot. The old code aliased the snapshot maps directly.
	restoredShips := make(map[string]*ship.Ship, len(snapshot.Ships))
	for k, v := range snapshot.Ships {
		restoredShips[k] = v.Clone()
	}
	restoredProjs := make(map[string]*Projectile, len(snapshot.Projectiles))
	for k, v := range snapshot.Projectiles {
		cp := *v
		restoredProjs[k] = &cp
	}
	restoredObjs := make(map[string]*Object, len(snapshot.Objects))
	for k, v := range snapshot.Objects {
		cp := *v
		restoredObjs[k] = &cp
	}
	s.Ships = restoredShips
	s.Projectiles = restoredProjs
	s.Objects = restoredObjs
	s.AIControllers = copyAIControllers(snapshot.AIControllers)

	log.Printf("Restored snapshot from time %.2f", snapshot.Time)
	return nil
}

// SnapshotInfo describes every stored snapshot for the GM client.
func (s *Simulator) SnapshotInfo() []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]map[string]interface{}, 0, len(s.Snapshots))
	for i, snap := range s.Snapshots {
		out = append(out, map[string]interface{}{
			"index": i,
			"time":  snap.Time,
		})
	}
	return out
}

func copyAIControllers(src map[string]*ai.Controller) map[string]*ai.Controller {
	out := make(map[string]*ai.Controller, len(src))
	for id, c := range src {
		out[id] = c.Clone()
	}
	return out
}

func (s *Simulator) copyShips() map[string]*ship.Ship {
	ships := make(map[string]*ship.Ship, len(s.Ships))
	for k, v := range s.Ships {
		ships[k] = v.Clone()
	}
	return ships
}

func (s *Simulator) copyProjectiles() map[string]*Projectile {
	projectiles := make(map[string]*Projectile)
	for k, v := range s.Projectiles {
		projCopy := *v
		projectiles[k] = &projCopy
	}
	return projectiles
}

func (s *Simulator) copyObjects() map[string]*Object {
	objects := make(map[string]*Object)
	for k, v := range s.Objects {
		objCopy := *v
		objects[k] = &objCopy
	}
	return objects
}

// distance returns the linear distance between two points.
func distance(a, b ship.Vector3) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}
