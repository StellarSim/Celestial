package mission

import (
	"celestial/internal/ship"
	"celestial/internal/simulation"
	"fmt"
	"log"
	"os"
	"path/filepath"

	lua "github.com/yuin/gopher-lua"
)

type Engine struct {
	simulator *simulation.Simulator
	missions  map[string]*Mission
	active    *Mission
	L         *lua.LState
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

	e.active = mission
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
}

func (e *Engine) luaSpawnShip(L *lua.LState) int {
	shipID := L.ToString(1)
	classID := L.ToString(2)
	name := L.ToString(3)
	isPlayer := L.ToBool(4)
	posTable := L.ToTable(5)

	x := posTable.RawGetString("x").(lua.LNumber)
	y := posTable.RawGetString("y").(lua.LNumber)
	z := posTable.RawGetString("z").(lua.LNumber)

	position := ship.Vector3{
		X: float64(x),
		Y: float64(y),
		Z: float64(z),
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

	x := posTable.RawGetString("x").(lua.LNumber)
	y := posTable.RawGetString("y").(lua.LNumber)
	z := posTable.RawGetString("z").(lua.LNumber)

	position := ship.Vector3{
		X: float64(x),
		Y: float64(y),
		Z: float64(z),
	}

	e.simulator.SpawnObject(objectID, objectType, position)
	return 0
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

func (e *Engine) luaSetObjective(L *lua.LState) int {
	objID := L.ToString(1)
	description := L.ToString(2)

	if e.active != nil {
		e.active.Objectives = append(e.active.Objectives, Objective{
			ID:          objID,
			Description: description,
			Completed:   false,
		})
		log.Printf("Objective set: %s - %s", objID, description)
	}

	return 0
}

func (e *Engine) luaCompleteObjective(L *lua.LState) int {
	objID := L.ToString(1)

	if e.active != nil {
		for i := range e.active.Objectives {
			if e.active.Objectives[i].ID == objID {
				e.active.Objectives[i].Completed = true
				log.Printf("Objective completed: %s", objID)
				break
			}
		}
	}

	return 0
}

func (e *Engine) luaMissionWin(L *lua.LState) int {
	log.Println("Mission completed successfully!")
	return 0
}

func (e *Engine) luaMissionLose(L *lua.LState) int {
	reason := L.ToString(1)
	log.Printf("Mission failed: %s", reason)
	return 0
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

func (e *Engine) GetMissions() map[string]*Mission {
	return e.missions
}
