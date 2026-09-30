package network

import (
	"celestial/internal/config"
	"celestial/internal/ship"
	"celestial/internal/simulation"
)

func testClasses() map[string]*config.ShipClass {
	return map[string]*config.ShipClass{
		"cruiser": {
			ID: "cruiser", Name: "Cruiser",
			Mass: 500000, MaxSpeed: 250, Acceleration: 50, TurnRate: 0.8,
			Engines: []config.EngineConfig{
				{ID: "main_1", Type: "main", Thrust: 100000, Health: 100, PowerDraw: 150},
			},
			Weapons: []config.WeaponConfig{
				{ID: "phaser_1", Type: "phaser", Damage: 25, Range: 2000, CooldownTime: 2, Health: 100, PowerDraw: 50},
				{ID: "torpedo_bay_1", Type: "torpedo", Damage: 100, Range: 5000, CooldownTime: 5, Health: 100, PowerDraw: 20, AmmoCapacity: 20},
			},
			Shields: config.ShieldConfig{
				RechargeRate: 10, PowerDraw: 100,
				Emitters: []config.EmitterConfig{
					{ID: "forward", Facing: "forward", Strength: 500, Health: 100},
					{ID: "aft", Facing: "aft", Strength: 500, Health: 100},
				},
			},
			Hull: config.HullConfig{Sections: []config.HullSectionConfig{
				{ID: "forward", Armor: 200, Health: 500},
				{ID: "aft", Armor: 200, Health: 500},
				{ID: "port", Armor: 150, Health: 400},
				{ID: "starboard", Armor: 150, Health: 400},
			}},
			Subsystems: []config.SubsystemConfig{
				{ID: "sensors", Type: "sensors", Health: 100, PowerDraw: 30},
			},
			LaunchBays: []config.LaunchBayConfig{},
		},
	}
}

func newTestSim() *simulation.Simulator {
	sim := simulation.NewSimulator(60, testClasses())
	//nolint:errcheck // test fixture; class exists
	sim.SpawnShip("player", "cruiser", "Endeavour", true, ship.Vector3{})
	return sim
}

func ship0() ship.Vector3 {
	return ship.Vector3{}
}
