package network

import (
	"celestial/internal/config"
	"celestial/internal/input"
	"testing"
)

// canonicalPanelIDs lists every panel ID defined in backend/configs/panels.yaml.
var canonicalPanelIDs = []string{
	"engineer_power_main", "engineer_damage_main", "engineer_systems",
	"flight_main", "flight_navigation",
	"weapons_torpedos_1", "weapons_torpedos_2", "weapons_phasers",
	"captain_command", "captain_status",
	"comms_main",
	"operations_power", "operations_resources",
	"relay_sensors", "relay_scanning",
	"first_officer_main",
}

func loadPanelMappings(t *testing.T) *config.PanelMapping {
	t.Helper()
	mappings, err := config.LoadPanelMappings("../../configs/panels.yaml")
	if err != nil {
		t.Fatalf("LoadPanelMappings: %v", err)
	}
	return mappings
}

func TestPanelsUseCanonicalIDs(t *testing.T) {
	mappings := loadPanelMappings(t)

	known := make(map[string]bool, len(canonicalPanelIDs))
	for _, id := range canonicalPanelIDs {
		known[id] = true
	}

	for id := range mappings.Panels {
		if !known[id] {
			t.Errorf("panels.yaml declares non-canonical panel id %q", id)
		}
	}

	for _, id := range canonicalPanelIDs {
		if _, ok := mappings.Panels[id]; !ok {
			t.Errorf("panels.yaml is missing canonical panel id %q", id)
		}
	}
}

func TestPanelActionsResolveInCatalog(t *testing.T) {
	mappings := loadPanelMappings(t)
	sim := newTestSim()
	router := input.NewActionRouter(sim)

	for id, panel := range mappings.Panels {
		if panel.ID != "" && panel.ID != id {
			t.Errorf("Panel %s has mismatched id field %q", id, panel.ID)
		}
		if panel.Role == "" {
			t.Errorf("Panel %s has no role", id)
		}
		for action, def := range panel.Actions {
			key := panel.Role + "." + def.System + "." + def.Action
			found := false
			for _, k := range router.CatalogKeys() {
				if k == key {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Panel %s action %q maps to %q which is not in the catalog", id, action, key)
			}
		}
	}
}
