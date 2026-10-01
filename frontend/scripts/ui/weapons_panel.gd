extends StationPanel
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
	super._ready()
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
	GameState.ship_removed.connect(_on_ship_removed)
	
	lock_btn.pressed.connect(_on_lock_target)
	clear_btn.pressed.connect(_on_clear_target)
	next_btn.pressed.connect(_on_next_target)
	
	auto_fire_toggle.toggled.connect(_on_auto_fire_toggled)
	fire_all_btn.pressed.connect(_on_fire_all_bays)
	
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

	# Add all non-player ships as potential targets
	for entry in NavUtils.contacts_by_distance():
		var ship: GameState.ShipState = entry.ship
		var ship_id: String = entry.id

		# Color-code by faction
		var faction_color: Color = Colors.get_faction_color(ship.faction)

		var display_text := "%s (%.1f km)" % [ship.name, entry.distance / 1000.0]
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
	var bearing := NavUtils.bearing_to(player_pos, target_pos)
	
	target_name.text = target.name
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
	hull_value.text = "%.0f%%" % target.hull_integrity
	hull_value.add_theme_color_override("font_color", Colors.get_health_color(target.hull_integrity / 100.0))


func _update_weapon_status(ship: GameState.ShipState) -> void:
	# Update beam weapons (spec: phaser_arrays)
	for i in range(_beam_controls.size()):
		var ctrl: Dictionary = _beam_controls[i]
		var beam_state = ship.weapons.phaser_arrays[i] if i < ship.weapons.phaser_arrays.size() else null
		
		if beam_state:
			(ctrl.toggle as CheckButton).set_pressed_no_signal(beam_state.health > 0 and beam_state.cooldown <= 0)
			(ctrl.charge as ProgressBar).value = 100.0 - clampf(beam_state.cooldown * 50.0, 0.0, 100.0)
			(ctrl.fire as Button).disabled = beam_state.cooldown > 0 or _locked_target_id.is_empty()
		else:
			(ctrl.fire as Button).disabled = true
	
	# Update torpedo tubes (spec: torpedo_bays)
	for i in range(_tube_controls.size()):
		var ctrl: Dictionary = _tube_controls[i]
		var tube_state = ship.weapons.torpedo_bays[i] if i < ship.weapons.torpedo_bays.size() else null
		
		if tube_state:
			(ctrl.toggle as CheckButton).set_pressed_no_signal(tube_state.armed)
			
			var status_label := ctrl.status as Label
			if not tube_state.loaded:
				status_label.text = "EMPTY"
				status_label.add_theme_color_override("font_color", Colors.STATUS_OFFLINE)
			elif tube_state.cooldown > 0:
				status_label.text = "LOADING..."
				status_label.add_theme_color_override("font_color", Colors.ALERT_YELLOW)
			elif tube_state.loaded and tube_state.locked:
				status_label.text = "READY"
				status_label.add_theme_color_override("font_color", Colors.STATUS_ONLINE)
			else:
				status_label.text = "LOADED"
				status_label.add_theme_color_override("font_color", Colors.ALERT_YELLOW)
			
			(ctrl.fire as Button).disabled = not (tube_state.loaded and tube_state.locked) or _locked_target_id.is_empty()
		else:
			(ctrl.fire as Button).disabled = true


func _update_inventory(ship: GameState.ShipState) -> void:
	var total_ammo := 0
	for bay in ship.weapons.torpedo_bays:
		total_ammo += int(bay.ammo)
	standard_count.text = str(total_ammo)
	emp_count.text = "0"
	nuclear_count.text = "0"
	mine_count.text = "0"


func _on_target_selected(idx: int) -> void:
	lock_btn.disabled = false


func _on_lock_target() -> void:
	var selected := target_list.get_selected_items()
	if selected.is_empty():
		return
	
	_locked_target_id = target_list.get_item_metadata(selected[0])
	NetworkClient.send_action("weapons", "set_target", {"target_id": _locked_target_id})


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
	NetworkClient.send_action("phaser", "set_enabled", {
		"array_id": _phaser_id(bank_idx),
		"enabled": enabled
	})


func _on_beam_fire(bank_idx: int) -> void:
	if _locked_target_id.is_empty():
		return
	NetworkClient.send_action("phaser", "fire", {
		"array_id": _phaser_id(bank_idx),
		"target_id": _locked_target_id
	})


func _on_tube_toggle(enabled: bool, tube_idx: int) -> void:
	NetworkClient.send_action("torpedo", "arm", {
		"bay_id": tube_idx + 1,
		"armed": enabled
	})


func _on_tube_type_changed(_type_idx: int, tube_idx: int) -> void:
	NetworkClient.send_action("torpedo", "load", {"bay_id": tube_idx + 1})


func _on_tube_fire(tube_idx: int) -> void:
	if _locked_target_id.is_empty():
		return
	NetworkClient.send_action("torpedo", "fire", {
		"bay_id": tube_idx + 1,
		"target_id": _locked_target_id
	})


func _on_auto_fire_toggled(enabled: bool) -> void:
	NetworkClient.send_action("torpedo", "set_auto_fire", {"enabled": enabled})


func _on_fire_all_bays() -> void:
	if _locked_target_id.is_empty():
		return
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	for bay in ship.weapons.torpedo_bays:
		NetworkClient.send_action("torpedo", "fire", {
			"bay_id": bay.bay_id,
			"target_id": _locked_target_id
		})


func _phaser_id(index: int) -> String:
	var ship := GameState.get_player_ship()
	if ship and index < ship.weapons.phaser_arrays.size():
		var array_id: String = ship.weapons.phaser_arrays[index].array_id
		if not array_id.is_empty():
			return array_id
	return "phaser_array_%d" % (index + 1)


func _on_state_updated() -> void:
	# Reflect the server's auto-fire state instead of keeping a local flag.
	auto_fire_toggle.set_pressed_no_signal(GameState.auto_fire)


func _on_ship_removed(ship_id: String) -> void:
	if ship_id == _locked_target_id:
		_locked_target_id = ""
