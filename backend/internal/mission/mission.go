package mission

import (
	"celestial/internal/ship"
	"celestial/internal/simulation"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"

	lua "github.com/yuin/gopher-lua"
)

type Engine struct {
	simulator *simulation.Simulator
	missions  map[string]*Mission
	active    *Mission
	L         *lua.LState
	OnEvent   func(event string, data map[string]interface{})
}

type Mission struct {
	ID          string
	Name        string
	Description string
	Script      string
	Objectives  []Objective
	State       map[string]interface{}
}

type Objective struct {
	ID          string
	Description string
	Completed   bool
}

func NewEngine(sim *simulation.Simulator) *Engine {
	return &Engine{
		simulator: sim,
		missions:  make(map[string]*Mission),
	}
}

func (e *Engine) LoadMissions(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading missions directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".lua" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		script, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading mission %s: %w", entry.Name(), err)
		}

		missionID := entry.Name()[:len(entry.Name())-4]
		mission := &Mission{
			ID:         missionID,
			Script:     string(script),
			Objectives: make([]Objective, 0),
			State:      make(map[string]interface{}),
		}

		e.missions[missionID] = mission
		log.Printf("Loaded mission: %s", missionID)
	}

	return nil
}

func (e *Engine) StartMission(missionID string) error {
	mission, ok := e.missions[missionID]
	if !ok {
		return fmt.Errorf("mission not found: %s", missionID)
	}

	// Close any previous Lua state before starting a new one.
	if e.L != nil {
		e.L.Close()
		e.L = nil
	}
	e.active = mission
	// A restart begins from a clean objective list.
	mission.Objectives = make([]Objective, 0)
	mission.State = make(map[string]interface{})
	e.L = lua.NewState()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Mission panic: %v", r)
		}
	}()

	e.registerAPI()

	if err := e.L.DoString(mission.Script); err != nil {
		return fmt.Errorf("executing mission script: %w", err)
	}

	e.readMissionTable(mission)

	if err := e.L.CallByParam(lua.P{
		Fn:      e.L.GetGlobal("on_start"),
		NRet:    0,
		Protect: true,
	}); err != nil {
		log.Printf("Mission on_start error: %v", err)
	}

	log.Printf("Started mission: %s", missionID)
	return nil
}

func (e *Engine) StopMission() {
	if e.active == nil {
		return
	}

	if e.L != nil {
		e.L.Close()
		e.L = nil
	}

	log.Printf("Stopped mission: %s", e.active.ID)
	e.active = nil
}

func (e *Engine) TriggerEvent(eventName string, params map[string]interface{}) {
	if e.active == nil || e.L == nil {
		return
	}

	fn := e.L.GetGlobal("on_event")
	if fn.Type() != lua.LTFunction {
		return
	}

	e.L.Push(fn)
	e.L.Push(lua.LString(eventName))

	table := e.L.NewTable()
	for k, v := range params {
		e.L.SetField(table, k, e.goToLua(v))
	}
	e.L.Push(table)

	if err := e.L.PCall(2, 0, nil); err != nil {
		log.Printf("Event trigger error: %v", err)
	}
}

func (e *Engine) registerAPI() {
	e.L.SetGlobal("spawn_ship", e.L.NewFunction(e.luaSpawnShip))
	e.L.SetGlobal("remove_ship", e.L.NewFunction(e.luaRemoveShip))
	e.L.SetGlobal("spawn_object", e.L.NewFunction(e.luaSpawnObject))
	e.L.SetGlobal("remove_object", e.L.NewFunction(e.luaRemoveObject))
	e.L.SetGlobal("damage_ship", e.L.NewFunction(e.luaDamageShip))
	e.L.SetGlobal("set_objective", e.L.NewFunction(e.luaSetObjective))
	e.L.SetGlobal("complete_objective", e.L.NewFunction(e.luaCompleteObjective))
	e.L.SetGlobal("mission_win", e.L.NewFunction(e.luaMissionWin))
	e.L.SetGlobal("mission_lose", e.L.NewFunction(e.luaMissionLose))
	e.L.SetGlobal("log", e.L.NewFunction(e.luaLog))
	e.L.SetGlobal("ship_exists", e.L.NewFunction(e.luaShipExists))
	e.L.SetGlobal("ship_health", e.L.NewFunction(e.luaShipHealth))
	e.L.SetGlobal("ship_distance", e.L.NewFunction(e.luaShipDistance))
	e.L.SetGlobal("set_faction", e.L.NewFunction(e.luaSetFaction))
	e.L.SetGlobal("order_attack", e.L.NewFunction(e.luaOrderAttack))
	e.L.SetGlobal("player_ship", e.L.NewFunction(e.luaPlayerShip))
	e.L.SetGlobal("get_object", e.L.NewFunction(e.luaGetObject))
}

// luaShipExists reports whether a ship is still in the simulation.
func (e *Engine) luaShipExists(L *lua.LState) int {
	L.Push(lua.LBool(e.simulator.GetShip(L.ToString(1)) != nil))
	return 1
}

// luaShipHealth returns a ship's remaining hull as a 0..1 fraction.
func (e *Engine) luaShipHealth(L *lua.LState) int {
	sh := e.simulator.GetShip(L.ToString(1))
	if sh == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	total, max := 0.0, 0.0
	for _, section := range sh.HullSnapshot() {
		total += section.Health
		max += section.MaxHealth
	}
	if max <= 0 {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(total / max))
	return 1
}

// luaShipDistance returns the distance between two ships in sim meters.
func (e *Engine) luaShipDistance(L *lua.LState) int {
	a := e.simulator.GetShip(L.ToString(1))
	b := e.simulator.GetShip(L.ToString(2))
	if a == nil || b == nil {
		L.Push(lua.LNumber(-1))
		return 1
	}
	L.Push(lua.LNumber(distance(a.GetPosition(), b.GetPosition())))
	return 1
}

// luaSetFaction re-tags a ship so AI hostility follows the mission script
// (for example a merchant sailing as "civilian" while pirates stay hostile).
func (e *Engine) luaSetFaction(L *lua.LState) int {
	shipID := L.ToString(1)
	faction := L.ToString(2)
	L.Push(lua.LBool(e.simulator.SetShipFaction(shipID, faction)))
	return 1
}

// luaOrderAttack forces an AI ship onto a target immediately, regardless of
// range. Used for scripted attacks such as a pirate ordered onto a merchant.
func (e *Engine) luaOrderAttack(L *lua.LState) int {
	shipID := L.ToString(1)
	targetID := L.ToString(2)
	L.Push(lua.LBool(e.simulator.OrderShipAttack(shipID, targetID)))
	return 1
}

// luaPlayerShip returns the id of the player ship, if any.
func (e *Engine) luaPlayerShip(L *lua.LState) int {
	for _, sh := range e.simulator.GetAllShips() {
		if sh.IsPlayer {
			L.Push(lua.LString(sh.ID))
			return 1
		}
	}
	L.Push(lua.LNil)
	return 1
}

// luaGetObject returns {x, y, z} for a spawned object.
func (e *Engine) luaGetObject(L *lua.LState) int {
	obj, ok := e.simulator.GetObject(L.ToString(1))
	if !ok {
		L.Push(lua.LNil)
		return 1
	}
	tbl := L.NewTable()
	L.SetField(tbl, "x", lua.LNumber(obj.Position.X))
	L.SetField(tbl, "y", lua.LNumber(obj.Position.Y))
	L.SetField(tbl, "z", lua.LNumber(obj.Position.Z))
	L.Push(tbl)
	return 1
}

func distance(a, b ship.Vector3) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func (e *Engine) luaSpawnShip(L *lua.LState) int {
	shipID := L.ToString(1)
	classID := L.ToString(2)
	name := L.ToString(3)
	isPlayer := L.ToBool(4)
	posTable := L.ToTable(5)

	x, ok := checkedNumber(posTable, "x")
	if !ok {
		L.Push(lua.LBool(false))
		return 1
	}
	y, ok := checkedNumber(posTable, "y")
	if !ok {
		L.Push(lua.LBool(false))
		return 1
	}
	z, ok := checkedNumber(posTable, "z")
	if !ok {
		L.Push(lua.LBool(false))
		return 1
	}

	position := ship.Vector3{
		X: x,
		Y: y,
		Z: z,
	}

	err := e.simulator.SpawnShip(shipID, classID, name, isPlayer, position)
	if err != nil {
		log.Printf("Lua spawn_ship error: %v", err)
		L.Push(lua.LBool(false))
	} else {
		L.Push(lua.LBool(true))
	}
	return 1
}

func (e *Engine) luaRemoveShip(L *lua.LState) int {
	shipID := L.ToString(1)
	e.simulator.RemoveShip(shipID)
	return 0
}

func (e *Engine) luaSpawnObject(L *lua.LState) int {
	objectID := L.ToString(1)
	objectType := L.ToString(2)
	posTable := L.ToTable(3)

	x, ok := checkedNumber(posTable, "x")
	if !ok {
		return 0
	}
	y, ok := checkedNumber(posTable, "y")
	if !ok {
		return 0
	}
	z, ok := checkedNumber(posTable, "z")
	if !ok {
		return 0
	}

	position := ship.Vector3{
		X: x,
		Y: y,
		Z: z,
	}

	if err := e.simulator.SpawnObject(objectID, objectType, position); err != nil {
		log.Printf("Lua spawn_object error: %v", err)
		L.Push(lua.LBool(false))
	} else {
		L.Push(lua.LBool(true))
	}
	return 1
}

func (e *Engine) luaRemoveObject(L *lua.LState) int {
	objectID := L.ToString(1)
	e.simulator.RemoveObject(objectID)
	return 0
}

func (e *Engine) luaDamageShip(L *lua.LState) int {
	shipID := L.ToString(1)
	damage := L.ToNumber(2)
	location := L.ToString(3)

	ship := e.simulator.GetShip(shipID)
	if ship != nil {
		ship.TakeDamage(float64(damage), location)
	}

	return 0
}

// luaSetObjective adds an objective, or updates the description of an existing
// one with the same id.
func (e *Engine) luaSetObjective(L *lua.LState) int {
	objID := L.ToString(1)
	description := L.ToString(2)

	if e.active == nil {
		return 0
	}

	for i := range e.active.Objectives {
		if e.active.Objectives[i].ID == objID {
			e.active.Objectives[i].Description = description
			log.Printf("Objective updated: %s - %s", objID, description)
			e.emitEvent("objective_set", map[string]interface{}{"objective_id": objID})
			return 0
		}
	}

	e.active.Objectives = append(e.active.Objectives, Objective{
		ID:          objID,
		Description: description,
		Completed:   false,
	})
	log.Printf("Objective set: %s - %s", objID, description)
	e.emitEvent("objective_set", map[string]interface{}{"objective_id": objID})
	return 0
}

func (e *Engine) luaCompleteObjective(L *lua.LState) int {
	objID := L.ToString(1)

	if e.active != nil {
		for i := range e.active.Objectives {
			if e.active.Objectives[i].ID == objID {
				e.active.Objectives[i].Completed = true
				log.Printf("Objective completed: %s", objID)
				e.emitEvent("objective_complete", map[string]interface{}{"objective_id": objID})
				break
			}
		}
	}

	return 0
}

func (e *Engine) luaMissionWin(L *lua.LState) int {
	log.Println("Mission completed successfully!")
	e.emitEvent("mission_win", map[string]interface{}{})
	return 0
}

func (e *Engine) luaMissionLose(L *lua.LState) int {
	reason := L.ToString(1)
	log.Printf("Mission failed: %s", reason)
	e.emitEvent("mission_lose", map[string]interface{}{"reason": reason})
	return 0
}

// Completes all objectives and emits a win event for GM tooling.
// It does not stop the mission; the GM stops or restarts explicitly.
func (e *Engine) MissionWin() {
	if e.active != nil {
		for i := range e.active.Objectives {
			e.active.Objectives[i].Completed = true
		}
	}
	log.Println("Mission completed successfully (GM)!")
	e.emitEvent("mission_win", map[string]interface{}{"source": "gm"})
}

// Emits a lose event for GM tooling.
func (e *Engine) MissionLose(reason string) {
	if reason == "" {
		reason = "Mission failed by GM"
	}
	log.Printf("Mission failed (GM): %s", reason)
	e.emitEvent("mission_lose", map[string]interface{}{"reason": reason, "source": "gm"})
}

func (e *Engine) luaLog(L *lua.LState) int {
	message := L.ToString(1)
	log.Printf("Mission log: %s", message)
	return 0
}

func (e *Engine) goToLua(v interface{}) lua.LValue {
	switch val := v.(type) {
	case string:
		return lua.LString(val)
	case float64:
		return lua.LNumber(val)
	case int:
		return lua.LNumber(val)
	case bool:
		return lua.LBool(val)
	default:
		return lua.LNil
	}
}

func (e *Engine) GetActiveMission() *Mission {
	return e.active
}

// GetActiveMissionID returns the active mission id, or "" when none runs.
func (e *Engine) GetActiveMissionID() string {
	if e.active == nil {
		return ""
	}
	return e.active.ID
}

// GetMissionIDs returns the loaded mission ids in sorted order.
func (e *Engine) GetMissionIDs() []string {
	ids := make([]string, 0, len(e.missions))
	for id := range e.missions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (e *Engine) GetMissions() map[string]*Mission {
	return e.missions
}

func (e *Engine) emitEvent(event string, data map[string]interface{}) {
	if data == nil {
		data = map[string]interface{}{}
	}
	if e.OnEvent != nil {
		e.OnEvent(event, data)
	}
}

func (e *Engine) readMissionTable(m *Mission) {
	if e.L == nil {
		return
	}
	tbl := e.L.GetGlobal("mission")
	mt, ok := tbl.(*lua.LTable)
	if !ok {
		return
	}
	if v := mt.RawGetString("name"); v != lua.LNil {
		if s, ok := v.(lua.LString); ok {
			m.Name = string(s)
		}
	}
	if m.Name == "" {
		m.Name = m.ID
	}
	if v := mt.RawGetString("description"); v != lua.LNil {
		if s, ok := v.(lua.LString); ok {
			m.Description = string(s)
		}
	}
}

func checkedNumber(tbl *lua.LTable, key string) (float64, bool) {
	v := tbl.RawGetString(key)
	n, ok := v.(lua.LNumber)
	if !ok {
		log.Printf("Mission: expected number for %q, got %s", key, v.Type().String())
		return 0, false
	}
	return float64(n), true
}
