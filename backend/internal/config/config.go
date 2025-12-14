package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	TickRate         int `yaml:"tick_rate"`
	WebSocketPort    int `yaml:"websocket_port"`
	TCPPort          int `yaml:"tcp_port"`
	SnapshotInterval int `yaml:"snapshot_interval"`
}

func LoadConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg ServerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

type ShipClass struct {
	ID           string            `yaml:"id"`
	Name         string            `yaml:"name"`
	Mass         float64           `yaml:"mass"`
	MaxSpeed     float64           `yaml:"max_speed"`
	Acceleration float64           `yaml:"acceleration"`
	TurnRate     float64           `yaml:"turn_rate"`
	Engines      []EngineConfig    `yaml:"engines"`
	Weapons      []WeaponConfig    `yaml:"weapons"`
	Shields      ShieldConfig      `yaml:"shields"`
	Hull         HullConfig        `yaml:"hull"`
	Subsystems   []SubsystemConfig `yaml:"subsystems"`
	LaunchBays   []LaunchBayConfig `yaml:"launch_bays"`
}

type EngineConfig struct {
	ID        string  `yaml:"id"`
	Type      string  `yaml:"type"`
	Thrust    float64 `yaml:"thrust"`
	Health    float64 `yaml:"health"`
	PowerDraw float64 `yaml:"power_draw"`
}

type WeaponConfig struct {
	ID           string  `yaml:"id"`
	Type         string  `yaml:"type"`
	Damage       float64 `yaml:"damage"`
	Range        float64 `yaml:"range"`
	CooldownTime float64 `yaml:"cooldown_time"`
	Health       float64 `yaml:"health"`
	PowerDraw    float64 `yaml:"power_draw"`
	AmmoCapacity int     `yaml:"ammo_capacity"`
}

type ShieldConfig struct {
	Emitters     []EmitterConfig `yaml:"emitters"`
	RechargeRate float64         `yaml:"recharge_rate"`
	PowerDraw    float64         `yaml:"power_draw"`
}

type EmitterConfig struct {
	ID       string  `yaml:"id"`
	Facing   string  `yaml:"facing"`
	Strength float64 `yaml:"strength"`
	Health   float64 `yaml:"health"`
}

type HullConfig struct {
	Sections []HullSectionConfig `yaml:"sections"`
}

type HullSectionConfig struct {
	ID     string  `yaml:"id"`
	Armor  float64 `yaml:"armor"`
	Health float64 `yaml:"health"`
}

type SubsystemConfig struct {
	ID        string  `yaml:"id"`
	Type      string  `yaml:"type"`
	Health    float64 `yaml:"health"`
	PowerDraw float64 `yaml:"power_draw"`
}

type LaunchBayConfig struct {
	ID       string  `yaml:"id"`
	Capacity int     `yaml:"capacity"`
	Health   float64 `yaml:"health"`
}

func LoadShipClasses(dir string) (map[string]*ShipClass, error) {
	classes := make(map[string]*ShipClass)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading ship classes directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading ship class %s: %w", entry.Name(), err)
		}

		var class ShipClass
		if err := yaml.Unmarshal(data, &class); err != nil {
			return nil, fmt.Errorf("parsing ship class %s: %w", entry.Name(), err)
		}

		classes[class.ID] = &class
	}

	return classes, nil
}

type PanelMapping struct {
	Panels map[string]PanelConfig `yaml:"panels"`
}

type PanelConfig struct {
	ID      string               `yaml:"id"`
	Role    string               `yaml:"role"`
	Actions map[string]ActionDef `yaml:"actions"`
}

type ActionDef struct {
	System string `yaml:"system"`
	Action string `yaml:"action"`
}

func LoadPanelMappings(path string) (*PanelMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading panel mappings: %w", err)
	}

	var mappings PanelMapping
	if err := yaml.Unmarshal(data, &mappings); err != nil {
		return nil, fmt.Errorf("parsing panel mappings: %w", err)
	}

	return &mappings, nil
}
