package ai

import (
	"celestial/internal/ship"
	"testing"
)

func hostilePair(a, b string) bool {
	return a == "pirate" && b == "civilian"
}

func TestHostileHookOverridesLegacyRule(t *testing.T) {
	c := NewController()
	c.Hostile = hostilePair

	pirate := &ship.Ship{ID: "p"}
	pirate.SetFaction("pirate")
	civilian := &ship.Ship{ID: "c"}
	civilian.SetFaction("civilian")

	if !c.isHostileTo(pirate, civilian) {
		t.Error("hooked pairing should be hostile")
	}
	if c.isHostileTo(civilian, pirate) {
		t.Error("reverse pairing should not be hostile")
	}
}

func TestLegacyRuleWithoutHook(t *testing.T) {
	c := NewController()

	a := &ship.Ship{ID: "a"}
	a.SetFaction("pirate")
	b := &ship.Ship{ID: "b"}
	b.SetFaction("civilian")

	if !c.isHostileTo(a, b) {
		t.Error("different factions should be hostile without a hook")
	}
	if c.isHostileTo(a, a) {
		t.Error("same faction should not be hostile without a hook")
	}
}
