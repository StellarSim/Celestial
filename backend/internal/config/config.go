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

// Reads every .yaml file in dir. Keeps one copy of the directory walk so
// each catalog loader only handles its own unmarshaling.
func readYAMLFiles(dir, plural, singular string) (map[string][]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s directory: %w", plural, err)
	}

	files := make(map[string][]byte)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s %s: %w", singular, entry.Name(), err)
		}

		files[entry.Name()] = data
	}

	return files, nil
}

func baseID(filename string) string {
	return filename[:len(filename)-len(filepath.Ext(filename))]
}

func LoadShipClasses(dir string) (map[string]*ShipClass, error) {
	classes := make(map[string]*ShipClass)

	files, err := readYAMLFiles(dir, "ship classes", "ship class")
	if err != nil {
		return nil, err
	}

	for name, data := range files {
		var class ShipClass
		if err := yaml.Unmarshal(data, &class); err != nil {
			return nil, fmt.Errorf("parsing ship class %s: %w", name, err)
		}
		if class.ID == "" {
			class.ID = baseID(name)
		}

		classes[class.ID] = &class
	}

	return classes, nil
}

// Describes a spawnable universe object: stations, asteroids, cargo,
// anomalies, hazards, mission markers. Users add new types by dropping
// a yaml file into configs/objects, mirroring configs/ships.
type ObjectClass struct {
	ID              string  `yaml:"id"`
	Name            string  `yaml:"name"`
	Category        string  `yaml:"category"`
	Description     string  `yaml:"description"`
	Radius          float64 `yaml:"radius"`
	Solid           bool    `yaml:"solid"`
	Scannable       bool    `yaml:"scannable"`
	SensorSignature float64 `yaml:"sensor_signature"`
	ArrivalRadius   float64 `yaml:"arrival_radius"`
	Color           string  `yaml:"color"`
}

func LoadObjectClasses(dir string) (map[string]*ObjectClass, error) {
	classes := make(map[string]*ObjectClass)

	files, err := readYAMLFiles(dir, "object classes", "object class")
	if err != nil {
		return nil, err
	}

	for name, data := range files {
		var class ObjectClass
		if err := yaml.Unmarshal(data, &class); err != nil {
			return nil, fmt.Errorf("parsing object class %s: %w", name, err)
		}
		if class.ID == "" {
			class.ID = baseID(name)
		}
		if class.Category == "" {
			class.Category = class.ID
		}

		classes[class.ID] = &class
	}

	return classes, nil
}

// Describes a named side. Users add new factions by dropping a yaml
// file into configs/factions.
// Stances are viewer centric and explicit: FriendlyTo and HostileTo list
// how this side regards others. Any unlisted pairing is neutral.
type Faction struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Color       string   `yaml:"color"`
	FriendlyTo  []string `yaml:"friendly_to"`
	HostileTo   []string `yaml:"hostile_to"`
}

func LoadFactions(dir string) (map[string]*Faction, error) {
	factions := make(map[string]*Faction)

	files, err := readYAMLFiles(dir, "factions", "faction")
	if err != nil {
		return nil, err
	}

	for name, data := range files {
		var faction Faction
		if err := yaml.Unmarshal(data, &faction); err != nil {
			return nil, fmt.Errorf("parsing faction %s: %w", name, err)
		}
		if faction.ID == "" {
			faction.ID = baseID(name)
		}

		factions[faction.ID] = &faction
	}

	return factions, nil
}

type PanelMapping struct {
	Panels map[string]PanelConfig `yaml:"panels"`
}

type PanelConfig struct {
	ID      string               `yaml:"id"`
	Role    string               `yaml:"role"`
	Actions map[string]ActionDef `yaml:"actions"`
}

// ActionDef maps a panel-local input name onto the shared action catalog.
// Value supplies any static parts of the target (for example the breaker name).
type ActionDef struct {
	System string                 `yaml:"system"`
	Action string                 `yaml:"action"`
	Value  map[string]interface{} `yaml:"value"`
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
