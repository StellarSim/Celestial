package config

import (
	"path/filepath"
	"runtime"
	"testing"
)

func configsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "configs")
}

func TestLoadShippedObjectClasses(t *testing.T) {
	classes, err := LoadObjectClasses(filepath.Join(configsDir(t), "objects"))
	if err != nil {
		t.Fatalf("LoadObjectClasses: %v", err)
	}
	for _, id := range []string{"waypoint", "station", "anomaly", "debris"} {
		class, ok := classes[id]
		if !ok {
			t.Errorf("expected object class %q", id)
			continue
		}
		if class.Category == "" {
			t.Errorf("object class %q needs a category", id)
		}
	}
	if classes["waypoint"].Category != "waypoint" {
		t.Errorf("waypoint class should have waypoint category, got %q", classes["waypoint"].Category)
	}
}

func TestLoadShippedFactions(t *testing.T) {
	factions, err := LoadFactions(filepath.Join(configsDir(t), "factions"))
	if err != nil {
		t.Fatalf("LoadFactions: %v", err)
	}
	for _, id := range []string{"player", "hostile", "neutral", "civilian", "pirate"} {
		if _, ok := factions[id]; !ok {
			t.Errorf("expected faction %q", id)
		}
	}
	if got := factions["player"].HostileTo; !contains(got, "pirate") {
		t.Errorf("player should be hostile to pirate, got %v", got)
	}
	if got := factions["civilian"].FriendlyTo; !contains(got, "player") {
		t.Errorf("civilian should be friendly to player, got %v", got)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func TestLoadShippedShipClasses(t *testing.T) {
	classes, err := LoadShipClasses(filepath.Join(configsDir(t), "ships"))
	if err != nil {
		t.Fatalf("LoadShipClasses: %v", err)
	}
	if len(classes) == 0 {
		t.Error("expected at least one ship class")
	}
}
