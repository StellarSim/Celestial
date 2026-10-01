extends Control
class_name StationPanel
## Base class for the eight role panels.
## Owns the shared `state_updated` subscription; panels implement their own
## refresh by overriding `_on_state_updated`.
## Shared math and formatting live in NavUtils (scripts/utils/nav_utils.gd).

func _ready() -> void:
	GameState.state_updated.connect(_on_state_updated)


func _on_state_updated() -> void:
	pass
