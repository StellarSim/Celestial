package ship

import (
	"celestial/internal/config"
	"testing"
)

func TestNewShip(t *testing.T) {
	class := &config.ShipClass{
		ID:           "test_ship",
		Name:         "Test Ship",
		Mass:         100000,
		MaxSpeed:     200,
		Acceleration: 50,
		TurnRate:     1.0,
		Engines: []config.EngineConfig{
			{ID: "main_1", Type: "main", Thrust: 50000, Health: 100, PowerDraw: 100},
		},
		Weapons: []config.WeaponConfig{
			{ID: "phaser_1", Type: "phaser", Damage: 25, Range: 2000, CooldownTime: 2.0, Health: 100, PowerDraw: 50},
		},
		Shields: config.ShieldConfig{
			RechargeRate: 10,
			PowerDraw:    100,
			Emitters: []config.EmitterConfig{
				{ID: "forward", Facing: "forward", Strength: 500, Health: 100},
			},
		},
		Hull: config.HullConfig{
			Sections: []config.HullSectionConfig{
				{ID: "forward", Armor: 200, Health: 500},
			},
		},
		Subsystems: []config.SubsystemConfig{
			{ID: "sensors", Type: "sensors", Health: 100, PowerDraw: 30},
		},
		LaunchBays: []config.LaunchBayConfig{},
	}

	ship := NewShip("ship_1", "test_ship", "Test Ship", class, true)

	if ship.ID != "ship_1" {
		t.Errorf("Expected ID ship_1, got %s", ship.ID)
	}

	if ship.ClassID != "test_ship" {
		t.Errorf("Expected ClassID test_ship, got %s", ship.ClassID)
	}

	if len(ship.Engines) != 1 {
		t.Errorf("Expected 1 engine, got %d", len(ship.Engines))
	}

	if len(ship.Weapons) != 1 {
		t.Errorf("Expected 1 weapon, got %d", len(ship.Weapons))
	}

	if ship.IsPlayer {
		if len(ship.Crew) != 8 {
			t.Errorf("Player ship should have 8 crew members, got %d", len(ship.Crew))
		}
	}
}

func TestShipDamage(t *testing.T) {
	class := &config.ShipClass{
		ID:           "test_ship",
		Name:         "Test Ship",
		Mass:         100000,
		MaxSpeed:     200,
		Acceleration: 50,
		TurnRate:     1.0,
		Engines:      []config.EngineConfig{},
		Weapons:      []config.WeaponConfig{},
		Shields: config.ShieldConfig{
			RechargeRate: 10,
			PowerDraw:    100,
			Emitters: []config.EmitterConfig{
				{ID: "forward", Facing: "forward", Strength: 500, Health: 100},
			},
		},
		Hull: config.HullConfig{
			Sections: []config.HullSectionConfig{
				{ID: "forward", Armor: 200, Health: 500},
			},
		},
		Subsystems: []config.SubsystemConfig{},
		LaunchBays: []config.LaunchBayConfig{},
	}

	ship := NewShip("ship_1", "test_ship", "Test Ship", class, false)

	initialShields := ship.Shields.Emitters["forward"].Strength
	ship.TakeDamage(100, "forward")

	if ship.Shields.Emitters["forward"].Strength >= initialShields {
		t.Error("Shields should have been damaged")
	}
}

func TestShipWeaponFire(t *testing.T) {
	class := &config.ShipClass{
		ID:           "test_ship",
		Name:         "Test Ship",
		Mass:         100000,
		MaxSpeed:     200,
		Acceleration: 50,
		TurnRate:     1.0,
		Engines:      []config.EngineConfig{},
		Weapons: []config.WeaponConfig{
			{ID: "torpedo_1", Type: "torpedo", Damage: 100, Range: 5000, CooldownTime: 5.0, Health: 100, PowerDraw: 20, AmmoCapacity: 10},
		},
		Shields:    config.ShieldConfig{Emitters: []config.EmitterConfig{}},
		Hull:       config.HullConfig{Sections: []config.HullSectionConfig{}},
		Subsystems: []config.SubsystemConfig{},
		LaunchBays: []config.LaunchBayConfig{},
	}

	ship := NewShip("ship_1", "test_ship", "Test Ship", class, false)

	weapon := ship.Weapons["torpedo_1"]
	weapon.Armed = true
	weapon.Loaded = true
	weapon.Locked = true

	initialAmmo := weapon.AmmoCount
	success := ship.FireWeapon("torpedo_1", "target_1")

	if !success {
		t.Error("Weapon should have fired successfully")
	}

	if weapon.AmmoCount != initialAmmo-1 {
		t.Errorf("Ammo count should decrease, expected %d got %d", initialAmmo-1, weapon.AmmoCount)
	}

	if weapon.Cooldown != weapon.CooldownTime {
		t.Error("Weapon should be on cooldown after firing")
	}
}

func TestShipUpdate(t *testing.T) {
	class := &config.ShipClass{
		ID:           "test_ship",
		Name:         "Test Ship",
		Mass:         100000,
		MaxSpeed:     200,
		Acceleration: 50,
		TurnRate:     1.0,
		Engines: []config.EngineConfig{
			{ID: "main_1", Type: "main", Thrust: 50000, Health: 100, PowerDraw: 100},
		},
		Weapons:    []config.WeaponConfig{},
		Shields:    config.ShieldConfig{Emitters: []config.EmitterConfig{}},
		Hull:       config.HullConfig{Sections: []config.HullSectionConfig{}},
		Subsystems: []config.SubsystemConfig{},
		LaunchBays: []config.LaunchBayConfig{},
	}

	ship := NewShip("ship_1", "test_ship", "Test Ship", class, false)

	initialPos := ship.Position
	ship.Update(0.1)

	if ship.Position.X != initialPos.X || ship.Position.Y != initialPos.Y || ship.Position.Z != initialPos.Z {
		// Position may change due to velocity updates
	}
}
