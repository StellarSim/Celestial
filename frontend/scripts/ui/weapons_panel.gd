extends Control
## Weapons station panel for targeting and firing weapons.

const TORPEDO_TYPES := ["Standard", "EMP", "Nuclear", "Mine"]

@onready var target_list: ItemList = $MainSplit/LeftSection/TargetingSection/TargetingContent/TargetList
@onready var lock_btn: Button = $MainSplit/LeftSection/TargetingSection/TargetingContent/TargetActions/LockTarget
@onready var clear_btn: Button = $MainSplit/LeftSection/TargetingSection/TargetingContent/TargetActions/ClearTarget
@onready var next_btn: Button = $MainSplit/LeftSection/TargetingSection/TargetingContent/TargetActions/NextTarget

@onready var target_name: Label = $MainSplit/LeftSection/TargetInfo/TargetInfoContent/TargetName
@onready var range_value: Label = $MainSplit/LeftSection/TargetInfo/TargetInfoContent/TargetDetails/RangeValue
@onready var bearing_value: Label = $MainSplit/LeftSection/TargetInfo/TargetInfoContent/TargetDetails/BearingValue
@onready var shield_value: Label = $MainSplit/LeftSection/TargetInfo/TargetInfoContent/TargetDetails/ShieldValue
@onready var hull_value: Label = $MainSplit/LeftSection/TargetInfo/TargetInfoContent/TargetDetails/HullValue

@onready var beam_banks: VBoxContainer = $MainSplit/RightSection/WeaponBanks/WeaponContent/BeamSection/BeamBanks
@onready var torpedo_tubes: VBoxContainer = $MainSplit/RightSection/WeaponBanks/WeaponContent/TorpedoSection/TorpedoTubes

@onready var standard_count: Label = $MainSplit/RightSection/TorpedoInventory/InventoryContent/InventoryGrid/StandardCount
@onready var emp_count: Label = $MainSplit/RightSection/TorpedoInventory/InventoryContent/InventoryGrid/EMPCount
@onready var nuclear_count: Label = $MainSplit/RightSection/TorpedoInventory/InventoryContent/InventoryGrid/NuclearCount
@onready var mine_count: Label = $MainSplit/RightSection/TorpedoInventory/InventoryContent/InventoryGrid/MineCount

@onready var auto_fire_toggle: CheckButton = $MainSplit/RightSection/FireControls/FireContent/AutoFireToggle
@onready var fire_all_btn: Button = $MainSplit/RightSection/FireControls/FireContent/FireAll

var _locked_target_id: String = ""
var _beam_controls: Array[Dictionary] = []
var _tube_controls: Array[Dictionary] = []


func _ready() -> void:
	_setup_weapon_controls()
	_connect_signals()


func _process(_delta: float) -> void:
	_update_display()


func _setup_weapon_controls() -> void:
	# Setup beam bank controls
	for i in range(beam_banks.get_child_count()):
		var bank := beam_banks.get_child(i) as HBoxContainer
		var toggle := bank.get_child(0) as CheckButton
		var charge := bank.get_child(2) as ProgressBar
		var fire_btn := bank.get_child(3) as Button
		
		_beam_controls.append({
			"toggle": toggle,
			"charge": charge,
			"fire": fire_btn
		})
		
		toggle.toggled.connect(_on_beam_toggle.bind(i))
		fire_btn.pressed.connect(_on_beam_fire.bind(i))
	
	# Setup torpedo tube controls
	for i in range(torpedo_tubes.get_child_count()):
		var tube := torpedo_tubes.get_child(i) as HBoxContainer
		var toggle := tube.get_child(0) as CheckButton
		var type_select := tube.get_child(2) as OptionButton
		var status := tube.get_child(3) as Label
		var fire_btn := tube.get_child(4) as Button
		
		# Populate torpedo types
		type_select.clear()
		for torp_type in TORPEDO_TYPES:
			type_select.add_item(torp_type)
		
		_tube_controls.append({
			"toggle": toggle,
			"type": type_select,
			"status": status,
			"fire": fire_btn
		})
		
		toggle.toggled.connect(_on_tube_toggle.bind(i))
		type_select.item_selected.connect(_on_tube_type_changed.bind(i))
		fire_btn.pressed.connect(_on_tube_fire.bind(i))


func _connect_signals() -> void:
	GameState.state_updated.connect(_on_state_updated)
	GameState.ship_added.connect(_on_ship_added)
	GameState.ship_removed.connect(_on_ship_removed)
	
	lock_btn.pressed.connect(_on_lock_target)
	clear_btn.pressed.connect(_on_clear_target)
	next_btn.pressed.connect(_on_next_target)
	
	auto_fire_toggle.toggled.connect(_on_auto_fire_toggled)
	fire_all_btn.pressed.connect(_on_fire_all)
	
	target_list.item_selected.connect(_on_target_selected)


func _update_display() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	_update_target_list()
	_update_target_info()
	_update_weapon_status(ship)
	_update_inventory(ship)


func _update_target_list() -> void:
	var player_ship := GameState.get_player_ship()
	if player_ship == null:
		return
	
	# Get current selection
	var selected_idx := -1
	var selected_items := target_list.get_selected_items()
	if not selected_items.is_empty():
		selected_idx = selected_items[0]
	
	target_list.clear()
	
	var player_pos := player_ship.position.to_vector3()
	
	# Add all non-player ships as potential targets
	for ship_id in GameState.ships:
		if ship_id == GameState.player_ship_id:
			continue
		
		var ship: GameState.ShipState = GameState.ships[ship_id]
		var ship_pos := ship.position.to_vector3()
		var distance := player_pos.distance_to(ship_pos)
		
		# Color-code by faction
		var faction_color: Color = Colors.get_faction_color(ship.faction)
		
		var display_text := "%s (%.1f km)" % [ship.display_name, distance / 1000.0]
		var idx := target_list.add_item(display_text)
		target_list.set_item_custom_fg_color(idx, faction_color)
		target_list.set_item_metadata(idx, ship_id)
		
		# Highlight locked target
		if ship_id == _locked_target_id:
			target_list.select(idx)
	
	lock_btn.disabled = target_list.get_selected_items().is_empty()


func _update_target_info() -> void:
	if _locked_target_id.is_empty():
		target_name.text = "---"
		range_value.text = "---"
		bearing_value.text = "---"
		shield_value.text = "---"
		hull_value.text = "---"
		return
	
	var target: GameState.ShipState = GameState.ships.get(_locked_target_id)
	if target == null:
		_locked_target_id = ""
		return
	
	var player_ship := GameState.get_player_ship()
	if player_ship == null:
		return
	
	var player_pos := player_ship.position.to_vector3()
	var target_pos := target.position.to_vector3()
	var distance := player_pos.distance_to(target_pos)
	
	# Calculate bearing
	var dir := (target_pos - player_pos).normalized()
	var bearing := rad_to_deg(atan2(dir.x, dir.z))
	bearing = fmod(bearing + 360, 360)
	
	target_name.text = target.display_name
	var target_color: Color = Colors.get_faction_color(target.faction)
	target_name.add_theme_color_override("font_color", target_color)
	
	range_value.text = "%.1f km" % (distance / 1000.0)
	bearing_value.text = "%03.0f°" % bearing
	
	# Shield status - use shield_facings dictionary
	var shield_total: float = 0.0
	for facing in target.shield_facings:
		var facing_val: float = target.shield_facings[facing]
		shield_total += facing_val
	var facing_count: int = target.shield_facings.size()
	var shield_avg: float = shield_total / max(facing_count, 1)
	shield_value.text = "%.0f%%" % shield_avg
	shield_value.add_theme_color_override("font_color", Colors.get_shield_color(shield_avg / 100.0))
	
	# Hull status
	hull_value.text = "%.0f%%" % target.hull
	hull_value.add_theme_color_override("font_color", Colors.get_health_color(target.hull / 100.0))


func _update_weapon_status(ship: GameState.ShipState) -> void:
	# Update beam weapons
	for i in range(_beam_controls.size()):
		var ctrl: Dictionary = _beam_controls[i]
		var beam_state = ship.weapons.beams[i] if i < ship.weapons.beams.size() else null
		
		if beam_state:
			(ctrl.toggle as CheckButton).set_pressed_no_signal(beam_state.enabled)
			(ctrl.charge as ProgressBar).value = beam_state.charge
			(ctrl.fire as Button).disabled = beam_state.charge < 100 or _locked_target_id.is_empty()
		else:
			(ctrl.fire as Button).disabled = true
	
	# Update torpedo tubes
	for i in range(_tube_controls.size()):
		var ctrl: Dictionary = _tube_controls[i]
		var tube_state = ship.weapons.tubes[i] if i < ship.weapons.tubes.size() else null
		
		if tube_state:
			(ctrl.toggle as CheckButton).set_pressed_no_signal(tube_state.enabled)
			
			var status_label := ctrl.status as Label
			match tube_state.state:
				"empty":
					status_label.text = "EMPTY"
					status_label.add_theme_color_override("font_color", Colors.STATUS_OFFLINE)
				"loading":
					status_label.text = "LOADING..."
					status_label.add_theme_color_override("font_color", Colors.ALERT_YELLOW)
				"ready":
					status_label.text = "READY"
					status_label.add_theme_color_override("font_color", Colors.STATUS_ONLINE)
				_:
					status_label.text = tube_state.state.to_upper()
			
			(ctrl.fire as Button).disabled = tube_state.state != "ready" or _locked_target_id.is_empty()
		else:
			(ctrl.fire as Button).disabled = true


func _update_inventory(ship: GameState.ShipState) -> void:
	var inv: Dictionary = ship.weapons.torpedo_inventory
	var standard_val: int = inv.get("standard", 0)
	var emp_val: int = inv.get("emp", 0)
	var nuclear_val: int = inv.get("nuclear", 0)
	var mine_val: int = inv.get("mine", 0)
	standard_count.text = str(standard_val)
	emp_count.text = str(emp_val)
	nuclear_count.text = str(nuclear_val)
	mine_count.text = str(mine_val)


func _on_target_selected(idx: int) -> void:
	lock_btn.disabled = false


func _on_lock_target() -> void:
	var selected := target_list.get_selected_items()
	if selected.is_empty():
		return
	
	_locked_target_id = target_list.get_item_metadata(selected[0])
	NetworkClient.send_action("weapons", "lock_target", {"target_id": _locked_target_id})


func _on_clear_target() -> void:
	_locked_target_id = ""
	NetworkClient.send_action("weapons", "clear_target", {})


func _on_next_target() -> void:
	if target_list.item_count == 0:
		return
	
	var current_idx := -1
	var selected := target_list.get_selected_items()
	if not selected.is_empty():
		current_idx = selected[0]
	
	var next_idx := (current_idx + 1) % target_list.item_count
	target_list.select(next_idx)
	_on_lock_target()


func _on_beam_toggle(enabled: bool, bank_idx: int) -> void:
	NetworkClient.send_action("weapons", "toggle_beam", {
		"bank": bank_idx,
		"enabled": enabled
	})


func _on_beam_fire(bank_idx: int) -> void:
	if _locked_target_id.is_empty():
		return
	NetworkClient.send_action("weapons", "fire_beam", {
		"bank": bank_idx,
		"target_id": _locked_target_id
	})


func _on_tube_toggle(enabled: bool, tube_idx: int) -> void:
	NetworkClient.send_action("weapons", "toggle_tube", {
		"tube": tube_idx,
		"enabled": enabled
	})


func _on_tube_type_changed(type_idx: int, tube_idx: int) -> void:
	NetworkClient.send_action("weapons", "load_tube", {
		"tube": tube_idx,
		"torpedo_type": TORPEDO_TYPES[type_idx].to_lower()
	})


func _on_tube_fire(tube_idx: int) -> void:
	if _locked_target_id.is_empty():
		return
	NetworkClient.send_action("weapons", "fire_torpedo", {
		"tube": tube_idx,
		"target_id": _locked_target_id
	})


func _on_auto_fire_toggled(enabled: bool) -> void:
	NetworkClient.send_action("weapons", "auto_fire", {"enabled": enabled})


func _on_fire_all() -> void:
	if _locked_target_id.is_empty():
		return
	NetworkClient.send_action("weapons", "fire_all", {"target_id": _locked_target_id})


func _on_state_updated() -> void:
	pass  # Updates handled in _process


func _on_ship_added(_ship_id: String) -> void:
	pass  # List updates in _process


func _on_ship_removed(ship_id: String) -> void:
	if ship_id == _locked_target_id:
		_locked_target_id = ""
