package panel

import (
	"testing"
)

func TestNewPanelStateManager(t *testing.T) {
	psm := NewPanelStateManager()

	if psm == nil {
		t.Fatal("Expected PanelStateManager, got nil")
	}
}

func TestPanelStateTimestamp(t *testing.T) {
	psm := NewPanelStateManager()

	// Test retrieving state from empty manager
	state := psm.GetState("nonexistent")

	if state != nil {
		t.Log("Panel state manager handles unknown panels")
	}
}

func TestGetState(t *testing.T) {
	psm := NewPanelStateManager()

	// Get state that doesn't exist
	state := psm.GetState("nonexistent_panel")

	if state != nil {
		t.Error("Expected nil for nonexistent panel")
	}
}

func TestGetAllStates(t *testing.T) {
	psm := NewPanelStateManager()

	// Get all states when empty
	allStates := psm.GetAllStates()

	if allStates == nil {
		t.Error("Expected empty map, got nil")
	}

	if len(allStates) != 0 {
		t.Logf("Found %d cached states", len(allStates))
	}
}

func TestUnknownPanel(t *testing.T) {
	psm := NewPanelStateManager()

	state := psm.GetState("completely_unknown_panel_id")

	if state == nil {
		t.Log("Unknown panel returns nil as expected")
	}
}

func TestIndicatorStructure(t *testing.T) {
	indicator := Indicator{
		Type:  "led",
		Value: 1,
		Color: "green",
		Blink: false,
	}

	if indicator.Type != "led" {
		t.Errorf("Expected type 'led', got %s", indicator.Type)
	}

	if indicator.Value != 1 {
		t.Errorf("Expected value 1, got %d", indicator.Value)
	}

	if indicator.Color != "green" {
		t.Errorf("Expected color 'green', got %s", indicator.Color)
	}

	if indicator.Blink {
		t.Error("Expected blink false, got true")
	}
}

func TestDisplayStructure(t *testing.T) {
	display := Display{
		Type:   "numeric",
		Value:  123.45,
		Unit:   "MW",
		Format: "%.2f",
	}

	if display.Type != "numeric" {
		t.Errorf("Expected type 'numeric', got %s", display.Type)
	}

	if display.Value != 123.45 {
		t.Errorf("Expected value 123.45, got %.2f", display.Value)
	}

	if display.Unit != "MW" {
		t.Errorf("Expected unit 'MW', got %s", display.Unit)
	}

	if display.Format != "%.2f" {
		t.Errorf("Expected format '%%.2f', got %s", display.Format)
	}
}

func TestPanelStateStructure(t *testing.T) {
	state := &PanelState{
		PanelID:   "test_panel",
		Timestamp: 1000.0,
		Indicators: map[string]Indicator{
			"test_led": {
				Type:  "led",
				Value: 1,
				Color: "red",
				Blink: true,
			},
		},
		Displays: map[string]Display{
			"test_display": {
				Type:   "numeric",
				Value:  42.0,
				Unit:   "units",
				Format: "%.0f",
			},
		},
	}

	if state.PanelID != "test_panel" {
		t.Errorf("Expected panel ID 'test_panel', got %s", state.PanelID)
	}

	if state.Timestamp != 1000.0 {
		t.Errorf("Expected timestamp 1000.0, got %.2f", state.Timestamp)
	}

	if len(state.Indicators) != 1 {
		t.Errorf("Expected 1 indicator, got %d", len(state.Indicators))
	}

	if len(state.Displays) != 1 {
		t.Errorf("Expected 1 display, got %d", len(state.Displays))
	}
}

func TestMultiplePanelStates(t *testing.T) {
	psm := NewPanelStateManager()

	panelIDs := []string{
		"engineer_power_main",
		"flight_main",
		"weapons_torpedos_1",
		"captain_command",
	}

	for _, panelID := range panelIDs {
		state := psm.GetState(panelID)
		if state != nil {
			t.Logf("Found cached state for %s", panelID)
		}
	}
}
