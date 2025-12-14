extends RefCounted
class_name Colors
## Centralized color definitions for the Celestial UI.
## Use these constants throughout the project for consistent theming.

# Primary palette - Celestial blue theme
const PRIMARY := Color(0.2, 0.6, 0.9, 1.0)
const PRIMARY_LIGHT := Color(0.4, 0.75, 1.0, 1.0)
const PRIMARY_DARK := Color(0.1, 0.35, 0.55, 1.0)

# Secondary colors
const SECONDARY := Color(0.15, 0.4, 0.6, 1.0)
const ACCENT := Color(0.3, 0.8, 0.95, 1.0)

# Background colors
const BG_DARK := Color(0.02, 0.04, 0.06, 1.0)
const BG_MEDIUM := Color(0.05, 0.08, 0.12, 1.0)
const BG_LIGHT := Color(0.1, 0.15, 0.2, 1.0)
const BG_PANEL := Color(0.05, 0.08, 0.12, 0.95)

# Text colors
const TEXT_PRIMARY := Color(0.9, 0.95, 1.0, 1.0)
const TEXT_SECONDARY := Color(0.7, 0.75, 0.8, 1.0)
const TEXT_MUTED := Color(0.5, 0.55, 0.6, 1.0)
const TEXT_HIGHLIGHT := Color(0.4, 0.85, 1.0, 1.0)
const TEXT_NORMAL := Color(0.85, 0.9, 0.95, 1.0)

# Alert levels
const ALERT_GREEN := Color(0.2, 0.8, 0.4, 1.0)
const ALERT_YELLOW := Color(0.9, 0.75, 0.2, 1.0)
const ALERT_ORANGE := Color(0.95, 0.5, 0.15, 1.0)
const ALERT_RED := Color(0.9, 0.2, 0.15, 1.0)

# Status colors
const STATUS_ONLINE := Color(0.2, 0.85, 0.4, 1.0)
const STATUS_OFFLINE := Color(0.5, 0.5, 0.5, 1.0)
const STATUS_WARNING := Color(0.95, 0.7, 0.2, 1.0)
const STATUS_CRITICAL := Color(0.95, 0.25, 0.2, 1.0)
const STATUS_CHARGING := Color(0.3, 0.7, 0.95, 1.0)

# System colors
const POWER_ON := Color(0.3, 0.85, 0.4, 1.0)
const POWER_OFF := Color(0.4, 0.1, 0.1, 1.0)
const SHIELDS := Color(0.3, 0.7, 0.95, 1.0)
const SHIELDS_LOW := Color(0.95, 0.6, 0.2, 1.0)
const HULL := Color(0.6, 0.6, 0.65, 1.0)
const HULL_DAMAGED := Color(0.85, 0.3, 0.2, 1.0)

# Weapon colors
const PHASER := Color(0.95, 0.6, 0.2, 1.0)
const TORPEDO := Color(0.3, 0.85, 0.4, 1.0)
const TORPEDO_ARMED := Color(0.95, 0.3, 0.2, 1.0)
const TORPEDO_LOCKED := Color(0.95, 0.85, 0.2, 1.0)

# Faction colors for ships
const FACTION_PLAYER := Color(0.3, 0.7, 0.95, 1.0)
const FACTION_FRIENDLY := Color(0.3, 0.85, 0.4, 1.0)
const FACTION_NEUTRAL := Color(0.7, 0.7, 0.2, 1.0)
const FACTION_HOSTILE := Color(0.95, 0.25, 0.2, 1.0)
const FACTION_UNKNOWN := Color(0.6, 0.6, 0.6, 1.0)

# Damage indicators
const FIRE := Color(0.95, 0.4, 0.1, 1.0)
const BREACH := Color(0.6, 0.2, 0.8, 1.0)
const DAMAGE_NONE := Color(0.2, 0.7, 0.3, 1.0)
const DAMAGE_LIGHT := Color(0.8, 0.75, 0.2, 1.0)
const DAMAGE_MODERATE := Color(0.95, 0.5, 0.15, 1.0)
const DAMAGE_HEAVY := Color(0.9, 0.25, 0.15, 1.0)
const DAMAGE_CRITICAL := Color(0.7, 0.1, 0.1, 1.0)

# UI interactive elements
const BUTTON_GLOW := Color(0.4, 0.8, 1.0, 0.3)
const BORDER_NORMAL := Color(0.2, 0.4, 0.6, 0.8)
const BORDER_HIGHLIGHT := Color(0.4, 0.7, 1.0, 1.0)
const SELECTION := Color(0.3, 0.6, 0.9, 0.3)

# Environment/3D
const ENGINE_GLOW := Color(0.3, 0.6, 1.0, 1.0)
const ENGINE_THRUST := Color(0.5, 0.7, 1.0, 1.0)
const EXPLOSION_CORE := Color(1.0, 0.9, 0.6, 1.0)
const EXPLOSION_OUTER := Color(1.0, 0.4, 0.1, 1.0)
const STAR := Color(1.0, 0.98, 0.95, 1.0)
const NEBULA := Color(0.4, 0.2, 0.6, 0.3)


# Utility functions for dynamic coloring
static func get_health_color(health_percent: float) -> Color:
	if health_percent > 0.75:
		return ALERT_GREEN
	elif health_percent > 0.5:
		return ALERT_YELLOW
	elif health_percent > 0.25:
		return ALERT_ORANGE
	else:
		return ALERT_RED


static func get_power_color(power_percent: float) -> Color:
	if power_percent > 0.5:
		return PRIMARY_LIGHT
	elif power_percent > 0.25:
		return ALERT_YELLOW
	else:
		return ALERT_RED


static func get_shield_color(shield_percent: float) -> Color:
	if shield_percent > 0.6:
		return SHIELDS
	elif shield_percent > 0.3:
		return SHIELDS_LOW
	else:
		return ALERT_RED


static func get_damage_section_color(health_percent: float) -> Color:
	if health_percent > 0.9:
		return DAMAGE_NONE
	elif health_percent > 0.7:
		return DAMAGE_LIGHT
	elif health_percent > 0.4:
		return DAMAGE_MODERATE
	elif health_percent > 0.15:
		return DAMAGE_HEAVY
	else:
		return DAMAGE_CRITICAL


static func get_faction_color(faction: String) -> Color:
	match faction.to_lower():
		"player", "federation":
			return FACTION_PLAYER
		"friendly", "ally", "allied":
			return FACTION_FRIENDLY
		"neutral", "civilian":
			return FACTION_NEUTRAL
		"hostile", "enemy", "klingon", "romulan":
			return FACTION_HOSTILE
		_:
			return FACTION_UNKNOWN
