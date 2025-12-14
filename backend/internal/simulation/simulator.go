package simulation

import (
	"celestial/internal/ai"
	"celestial/internal/config"
	"celestial/internal/ship"
	"log"
	"sync"
	"time"
)

type Simulator struct {
	mu sync.RWMutex

	tickRate  int
	dt        float64
	running   bool
	paused    bool
	stopChan  chan struct{}
	pauseChan chan bool

	Ships       map[string]*ship.Ship
	Projectiles map[string]*Projectile
	Objects     map[string]*Object

	ShipClasses map[string]*config.ShipClass

	AIControllers map[string]*ai.Controller

	CurrentTime   float64
	Snapshots     []*Snapshot
	SnapshotIndex int
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
	Time        float64
	Ships       map[string]*ship.Ship
	Projectiles map[string]*Projectile
	Objects     map[string]*Object
}

func NewSimulator(tickRate int, shipClasses map[string]*config.ShipClass) *Simulator {
	return &Simulator{
		tickRate:      tickRate,
		dt:            1.0 / float64(tickRate),
		Ships:         make(map[string]*ship.Ship),
		Projectiles:   make(map[string]*Projectile),
		Objects:       make(map[string]*Object),
		ShipClasses:   shipClasses,
		AIControllers: make(map[string]*ai.Controller),
		stopChan:      make(chan struct{}),
		pauseChan:     make(chan bool),
		Snapshots:     make([]*Snapshot, 0),
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
		case pauseState := <-s.pauseChan:
			s.mu.Lock()
			s.paused = pauseState
			s.mu.Unlock()
			if pauseState {
				log.Println("Simulator paused")
			} else {
				log.Println("Simulator resumed")
			}
		case <-ticker.C:
			s.mu.RLock()
			paused := s.paused
			s.mu.RUnlock()

			if !paused {
				s.Tick()
			}
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

func (s *Simulator) Pause() {
	s.pauseChan <- true
}

func (s *Simulator) Resume() {
	s.pauseChan <- false
}

func (s *Simulator) Tick() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.CurrentTime += s.dt

	for _, sh := range s.Ships {
		sh.Update(s.dt)
	}

	s.updateProjectiles()
	s.updateAI()
	s.checkCollisions()
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
				if dist < 50.0 {
					target.TakeDamage(proj.Damage, "forward")
					log.Printf("Projectile %s hit ship %s for %.1f damage", id, proj.TargetID, proj.Damage)
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

func (s *Simulator) checkCollisions() {
	// Basic collision detection for ships
	ships := make([]*ship.Ship, 0, len(s.Ships))
	for _, sh := range s.Ships {
		ships = append(ships, sh)
	}

	for i := 0; i < len(ships); i++ {
		for j := i + 1; j < len(ships); j++ {
			dist := distance(ships[i].Position, ships[j].Position)
			if dist < 100.0 {
				ships[i].TakeDamage(10.0, "forward")
				ships[j].TakeDamage(10.0, "forward")
				log.Printf("Collision between %s and %s", ships[i].ID, ships[j].ID)
			}
		}
	}
}

func (s *Simulator) SpawnShip(id, classID, name string, isPlayer bool, position ship.Vector3) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	class, ok := s.ShipClasses[classID]
	if !ok {
		log.Printf("Unknown ship class: %s", classID)
		return nil
	}

	sh := ship.NewShip(id, classID, name, class, isPlayer)
	sh.Position = position
	s.Ships[id] = sh

	if !isPlayer {
		s.AIControllers[id] = ai.NewController()
	}

	log.Printf("Spawned ship: %s (%s) at position (%.1f, %.1f, %.1f)", name, classID, position.X, position.Y, position.Z)
	return nil
}

func (s *Simulator) RemoveShip(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Ships, id)
	delete(s.AIControllers, id)
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

func (s *Simulator) SpawnObject(id, objType string, position ship.Vector3) {
	s.mu.Lock()
	defer s.mu.Unlock()

	obj := &Object{
		ID:       id,
		Type:     objType,
		Position: position,
		Velocity: ship.Vector3{0, 0, 0},
		Rotation: ship.Quaternion{1, 0, 0, 0},
		Data:     make(map[string]interface{}),
	}

	s.Objects[id] = obj
	log.Printf("Spawned object: %s (%s) at position (%.1f, %.1f, %.1f)", id, objType, position.X, position.Y, position.Z)
}

func (s *Simulator) RemoveObject(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Objects, id)
	log.Printf("Removed object: %s", id)
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

func (s *Simulator) CreateSnapshot() {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := &Snapshot{
		Time:        s.CurrentTime,
		Ships:       s.copyShips(),
		Projectiles: s.copyProjectiles(),
		Objects:     s.copyObjects(),
	}

	s.Snapshots = append(s.Snapshots, snapshot)
	log.Printf("Created snapshot at time %.2f (total: %d)", s.CurrentTime, len(s.Snapshots))
}

func (s *Simulator) RestoreSnapshot(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(s.Snapshots) {
		log.Printf("Invalid snapshot index: %d", index)
		return nil
	}

	snapshot := s.Snapshots[index]
	s.CurrentTime = snapshot.Time
	s.Ships = snapshot.Ships
	s.Projectiles = snapshot.Projectiles
	s.Objects = snapshot.Objects

	log.Printf("Restored snapshot from time %.2f", snapshot.Time)
	return nil
}

func (s *Simulator) copyShips() map[string]*ship.Ship {
	ships := make(map[string]*ship.Ship)
	for k, v := range s.Ships {
		ships[k] = v
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

func distance(a, b ship.Vector3) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return (dx*dx + dy*dy + dz*dz)
}
