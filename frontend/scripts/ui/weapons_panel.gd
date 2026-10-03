extends StationPanel
## Weapons station panel for targeting and firing weapons.
##
## Torpedo flow per tube is LOAD then ARM then FIRE. ARM is the latching
## safety catch, so a tube must be armed before it will launch.

@onready var target_list: ItemList = %TargetList
@onready var lock_btn: Button = %LockTarget
@onready var clear_btn: Button = %ClearTarget
@onready var next_btn: Button = %NextTarget

@onready var target_name: Label = %TargetName
@onready var range_value: Label = %RangeValue
@onready var bearing_value: Label = %BearingValue
@onready var shield_value: Label = %ShieldValue
@onready var hull_value: Label = %HullValue
@onready var in_range_value: Label = %InRangeValue

@onready var feedback_label: Label = %FeedbackLabel
@onready var beam_banks: VBoxContainer = %BeamBanks
@onready var torpedo_tubes: VBoxContainer = %TorpedoTubes

@onready var total_count: Label = %TotalCount

@onready var fire_all_btn: Button = %FireAll

var _locked_target_id: String = ""
var _pending_selection: String = ""
var _beam_rows: Array[Dictionary] = []
var _tube_rows: Array[Dictionary] = []
var _target_refresh: float = 0.0
var _contacts_sig: String = ""


func _ready() -> void:
	super._ready()
	_connect_signals()
	_set_feedback("Select a target, then LOAD + ARM a tube and press FIRE.", false)


func _process(delta: float) -> void:
	_target_refresh -= delta
	if _target_refresh <= 0.0:
		_target_refresh = 0.25
		_refresh_target_list()
	_update_display()


func _connect_signals() -> void:
	GameState.ship_removed.connect(_on_ship_removed)
	NetworkClient.message_received.connect(_on_network_message)
	target_list.item_selected.connect(_on_target_selected)
	# Double-click, or Enter on a highlighted contact, locks it outright.
	target_list.item_activated.connect(_on_target_activated)
	lock_btn.pressed.connect(_on_lock_target)
	clear_btn.pressed.connect(_on_clear_target)
	next_btn.pressed.connect(_on_next_target)
	fire_all_btn.pressed.connect(_on_fire_all_bays)


func _on_network_message(data: Dictionary) -> void:
	if data.get("type") == "error":
		_set_feedback(str(data.get("message", "Server rejected the action")), true)


func _set_feedback(text: String, is_error: bool) -> void:
	feedback_label.text = text
	feedback_label.add_theme_color_override(
		"font_color", Colors.STATUS_CRITICAL if is_error else Colors.STATUS_ONLINE
	)


# Targeting

func _refresh_target_list() -> void:
	if GameState.get_player_ship() == null:
		return
	var contacts := NavUtils.contacts_by_distance()
	var sig_parts: PackedStringArray = []
	for entry in contacts:
		sig_parts.append("%s@%d" % [entry.id, int(entry.distance / 200.0)])
	var sig := "|".join(sig_parts)
	var selected_id := _pending_selection
	var selected := target_list.get_selected_items()
	if not selected.is_empty():
		var meta = target_list.get_item_metadata(selected[0])
		if meta != null:
			selected_id = str(meta)
	lock_btn.disabled = selected.is_empty() and _pending_selection.is_empty()
	if sig == _contacts_sig and target_list.item_count == contacts.size():
		return
	_contacts_sig = sig
	target_list.clear()
	for entry in contacts:
		var ship: GameState.ShipState = entry.ship
		var ship_id: String = entry.id
		var idx := target_list.add_item("%s (%s)" % [ship.name, NavUtils.format_range(entry.distance)])
		target_list.set_item_custom_fg_color(idx, GameState.get_faction_color(ship.faction))
		target_list.set_item_metadata(idx, ship_id)
		# Selection follows the locked target, or the last one clicked.
		if ship_id == _locked_target_id or ship_id == selected_id:
			target_list.select(idx)


func _update_target_info() -> void:
	if _locked_target_id.is_empty():
		target_name.text = "---  (no lock)"
		range_value.text = "---"
		bearing_value.text = "---"
		shield_value.text = "---"
		hull_value.text = "---"
		in_range_value.text = "---"
		return
	var target: GameState.ShipState = GameState.ships.get(_locked_target_id)
	if target == null:
		_locked_target_id = ""
		_pending_selection = ""
		return
	var player_ship := GameState.get_player_ship()
	if player_ship == null:
		return
	var player_pos := player_ship.position.to_vector3()
	var target_pos := target.position.to_vector3()
	var distance := player_pos.distance_to(target_pos)
	var bearing := NavUtils.bearing_to(player_pos, target_pos)
	target_name.text = target.name
	target_name.add_theme_color_override("font_color", GameState.get_faction_color(target.faction))
	range_value.text = NavUtils.format_range(distance)
	bearing_value.text = NavUtils.format_bearing(bearing)
	# Facings and hull are absolute totals from the server, so scale them
	# against the reported maxima rather than reading them as percentages.
	var shield_pct: float = (target.shields / target.max_shields * 100.0) if target.max_shields > 0.0 else 0.0
	shield_value.text = "%.0f%%" % shield_pct
	shield_value.add_theme_color_override("font_color", Colors.get_shield_color(shield_pct / 100.0))
	var hull_pct: float = (target.hull_integrity / target.max_hull * 100.0) if target.max_hull > 0.0 else 0.0
	hull_value.text = "%.0f%%" % hull_pct
	hull_value.add_theme_color_override("font_color", Colors.get_health_color(hull_pct / 100.0))
	var torp_ok := not _out_of_range(distance, _best_range(player_ship.weapons.torpedo_bays))
	var beam_ok := not _out_of_range(distance, _best_range(player_ship.weapons.phaser_arrays))
	in_range_value.text = "TORP %s / BEAM %s" % ["OK" if torp_ok else "OUT", "OK" if beam_ok else "OUT"]
	in_range_value.add_theme_color_override(
		"font_color", Colors.STATUS_ONLINE if (torp_ok or beam_ok) else Colors.ALERT_YELLOW
	)


func _best_range(weapons: Array) -> float:
	var best := 0.0
	for weapon in weapons:
		best = maxf(best, weapon.weapon_range)
	return best


# A negative distance means there is no locked target to be out of range of.
func _out_of_range(distance: float, weapon_range: float) -> bool:
	return distance >= 0.0 and weapon_range > 0.0 and distance > weapon_range


func _on_target_selected(idx: int) -> void:
	_pending_selection = str(target_list.get_item_metadata(idx))
	lock_btn.disabled = false


func _on_target_activated(idx: int) -> void:
	_on_target_selected(idx)
	_on_lock_target()


func _selected_or_pending() -> String:
	if not _pending_selection.is_empty():
		return _pending_selection
	var selected := target_list.get_selected_items()
	if selected.is_empty():
		return ""
	return str(target_list.get_item_metadata(selected[0]))


func _on_lock_target() -> void:
	var want := _selected_or_pending()
	if want.is_empty():
		_set_feedback("Select a contact first, then press LOCK TARGET.", true)
		return
	_pending_selection = want
	NetworkClient.send_action("weapons", "set_target", {"target_id": want})
	var target: GameState.ShipState = GameState.ships.get(want)
	var label := target.name if target != null else want
	_set_feedback("Target locked: %s. LOAD + ARM a tube, then FIRE." % label, false)


func _on_clear_target() -> void:
	_locked_target_id = ""
	_pending_selection = ""
	NetworkClient.send_action("weapons", "clear_target", {})
	_set_feedback("Target cleared.", false)


func _on_next_target() -> void:
	if target_list.item_count == 0:
		return
	var current_idx := -1
	var selected := target_list.get_selected_items()
	if not selected.is_empty():
		current_idx = selected[0]
	var next_idx := (current_idx + 1) % target_list.item_count
	target_list.select(next_idx)
	target_list.ensure_current_is_visible()
	_on_lock_target()


# Weapon banks

func _update_display() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	_update_target_info()
	_sync_rows(_beam_rows, ship.weapons.phaser_arrays.size(), _make_beam_row, _beam_row)
	_sync_rows(_tube_rows, ship.weapons.torpedo_bays.size(), _make_tube_row, _tube_row)
	_update_beams(ship)
	_update_tubes(ship)
	_update_inventory(ship)
	_update_fire_all(ship)


# Rows follow the weapon list, so a class with more banks than the previous
# one gets a row for each and a shrink drops the extras. `card_of` names the
# node to free when a row goes away.
func _sync_rows(rows: Array[Dictionary], count: int, make_row: Callable, card_of: Callable) -> void:
	while rows.size() < count:
		rows.append(make_row.call(rows.size()))
	while rows.size() > count:
		(card_of.call(rows.pop_back()) as Node).queue_free()


func _beam_row(row: Dictionary) -> Node:
	return (row["toggle"] as Node).get_parent()


func _tube_row(row: Dictionary) -> Node:
	return row["card"]


func _make_beam_row(idx: int) -> Dictionary:
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 8)
	var toggle := CheckButton.new()
	toggle.text = "ON"
	toggle.tooltip_text = "Enable / disable this beam array"
	var beam_name := Label.new()
	beam_name.text = "Beam %d" % (idx + 1)
	beam_name.custom_minimum_size = Vector2(140, 0)
	var charge := ProgressBar.new()
	charge.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	charge.max_value = 100.0
	charge.show_percentage = false
	charge.tooltip_text = "Charge level"
	var fire_btn := Button.new()
	fire_btn.text = "FIRE"
	fire_btn.custom_minimum_size = Vector2(90, 0)
	row.add_child(toggle)
	row.add_child(beam_name)
	row.add_child(charge)
	row.add_child(fire_btn)
	beam_banks.add_child(row)
	toggle.toggled.connect(_on_beam_toggle.bind(idx))
	fire_btn.pressed.connect(_on_beam_fire.bind(idx))
	return {"toggle": toggle, "name": beam_name, "charge": charge, "fire": fire_btn}


func _make_tube_row(idx: int) -> Dictionary:
	var card := PanelContainer.new()
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 4)
	card.add_child(box)

	var top := HBoxContainer.new()
	top.add_theme_constant_override("separation", 8)
	var tube_name := Label.new()
	tube_name.text = "Tube %d" % (idx + 1)
	tube_name.custom_minimum_size = Vector2(70, 0)
	var status := Label.new()
	status.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	var ammo := Label.new()
	ammo.custom_minimum_size = Vector2(52, 0)
	ammo.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
	top.add_child(tube_name)
	top.add_child(status)
	top.add_child(ammo)

	# One row in firing order: load the round, arm the tube, then fire.
	var bottom := HBoxContainer.new()
	bottom.add_theme_constant_override("separation", 8)
	var load_btn := Button.new()
	load_btn.text = "LOAD"
	load_btn.tooltip_text = "Momentary action: load one torpedo into the tube"
	var arm := CheckButton.new()
	arm.text = "ARM"
	arm.tooltip_text = "Latching safety: the tube must be armed to fire"
	var fire_btn := Button.new()
	fire_btn.text = "FIRE"
	fire_btn.custom_minimum_size = Vector2(0, 44)
	for btn in [load_btn, arm, fire_btn]:
		btn.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		bottom.add_child(btn)

	box.add_child(top)
	box.add_child(bottom)
	torpedo_tubes.add_child(card)
	arm.toggled.connect(_on_tube_arm.bind(idx))
	load_btn.pressed.connect(_on_tube_load.bind(idx))
	fire_btn.pressed.connect(_on_tube_fire.bind(idx))
	return {
		"card": card, "arm": arm, "name": tube_name, "status": status, "ammo": ammo,
		"load": load_btn, "fire": fire_btn,
	}


# Rows are positional, the server addresses bays by their configured id.
func _bay_id(idx: int) -> int:
	var ship := GameState.get_player_ship()
	if ship != null and idx >= 0 and idx < ship.weapons.torpedo_bays.size():
		var bay_id: int = ship.weapons.torpedo_bays[idx].bay_id
		if bay_id > 0:
			return bay_id
	return idx + 1


# Keeps a fire button in step with its state and always says why it is dead.
func _gate_fire(btn: Button, reason: String, ready_hint: String) -> void:
	btn.disabled = not reason.is_empty()
	btn.tooltip_text = reason if not reason.is_empty() else ready_hint


func _update_beams(ship: GameState.ShipState) -> void:
	var weapons_online: bool = ship.power_breakers.get("weapons", true)
	var target_dist := _locked_target_distance(ship)
	for i in range(_beam_rows.size()):
		var row: Dictionary = _beam_rows[i]
		var toggle := row["toggle"] as CheckButton
		var beam_name := row["name"] as Label
		var charge := row["charge"] as ProgressBar
		var fire_btn := row["fire"] as Button
		if i >= ship.weapons.phaser_arrays.size():
			continue
		var beam = ship.weapons.phaser_arrays[i]
		beam_name.text = beam.array_id if not beam.array_id.is_empty() else ("Beam %d" % (i + 1))
		toggle.set_pressed_no_signal(beam.enabled and beam.health > 0.0)
		var frac := 1.0
		if beam.cooldown_time > 0.0:
			frac = clampf(1.0 - beam.cooldown / beam.cooldown_time, 0.0, 1.0)
		charge.value = frac * 100.0
		var reason := ""
		if not weapons_online:
			reason = "Weapons power off"
		elif beam.health <= 0.0:
			reason = "Array destroyed"
		elif not beam.enabled:
			reason = "Array disabled"
		elif _locked_target_id.is_empty():
			reason = "No target locked"
		elif beam.cooldown > 0.0:
			reason = "Recharging %.1fs" % beam.cooldown
		elif _out_of_range(target_dist, beam.weapon_range):
			reason = "Target out of range"
		_gate_fire(fire_btn, reason, "Fire %s at locked target" % beam_name.text)


# A tube's readiness phrase plus the colour it reads in.
func _tube_status(bay: GameState.TorpedoBayState, weapons_online: bool) -> Array:
	if not weapons_online:
		return ["NO POWER", Colors.STATUS_OFFLINE]
	if not bay.loaded:
		if int(bay.ammo) <= 0:
			return ["EMPTY - NO AMMO", Colors.STATUS_OFFLINE]
		return ["EMPTY - PRESS LOAD", Colors.ALERT_YELLOW]
	if bay.cooldown > 0.0:
		return ["RELOADING %.1fs" % bay.cooldown, Colors.ALERT_YELLOW]
	if not bay.armed:
		return ["LOADED - TOGGLE ARM", Colors.ALERT_YELLOW]
	return ["ARMED - READY", Colors.STATUS_ONLINE]


func _update_tubes(ship: GameState.ShipState) -> void:
	var weapons_online: bool = ship.power_breakers.get("weapons", true)
	var target_dist := _locked_target_distance(ship)
	for i in range(_tube_rows.size()):
		var row: Dictionary = _tube_rows[i]
		var arm := row["arm"] as CheckButton
		var tube_name := row["name"] as Label
		var status := row["status"] as Label
		var ammo := row["ammo"] as Label
		var load_btn := row["load"] as Button
		var fire_btn := row["fire"] as Button
		if i >= ship.weapons.torpedo_bays.size():
			continue
		var bay = ship.weapons.torpedo_bays[i]
		var bay_id := _bay_id(i)
		tube_name.text = "Tube %d" % bay_id
		arm.set_pressed_no_signal(bay.armed)
		ammo.text = "x%d" % int(bay.ammo)
		var readout := _tube_status(bay, weapons_online)
		status.text = readout[0]
		status.add_theme_color_override("font_color", readout[1])
		load_btn.disabled = bay.loaded or int(bay.ammo) <= 0
		load_btn.tooltip_text = "Load one torpedo" if not load_btn.disabled else ("Already loaded" if bay.loaded else "No torpedoes left")
		var reason := ""
		if not weapons_online:
			reason = "Weapons power off"
		elif not bay.loaded:
			reason = "Tube empty: press LOAD"
		elif not bay.armed:
			reason = "Toggle ARM first"
		elif _locked_target_id.is_empty():
			reason = "No target locked"
		elif bay.cooldown > 0.0:
			reason = "Reloading"
		elif _out_of_range(target_dist, bay.weapon_range):
			reason = "Target out of range"
		_gate_fire(fire_btn, reason, "Fire tube %d at the locked target" % bay_id)


func _locked_target_distance(ship: GameState.ShipState) -> float:
	if _locked_target_id.is_empty():
		return -1.0
	var target: GameState.ShipState = GameState.ships.get(_locked_target_id)
	if target == null:
		return -1.0
	return ship.position.to_vector3().distance_to(target.position.to_vector3())


func _update_inventory(ship: GameState.ShipState) -> void:
	var reserve := 0
	var loaded := 0
	for bay in ship.weapons.torpedo_bays:
		reserve += int(bay.ammo)
		if bay.loaded:
			loaded += 1
	total_count.text = "%d loaded, %d in reserve" % [loaded, reserve]


func _update_fire_all(ship: GameState.ShipState) -> void:
	var ready := 0
	for bay in ship.weapons.torpedo_bays:
		if bay.loaded and bay.armed and bay.cooldown <= 0.0:
			ready += 1
	fire_all_btn.disabled = ready == 0 or _locked_target_id.is_empty()
	fire_all_btn.text = "FIRE ALL READY TUBES (%d)" % ready


# Control actions

func _beam_id(bank_idx: int) -> String:
	var ship := GameState.get_player_ship()
	if ship != null and bank_idx < ship.weapons.phaser_arrays.size():
		var array_id: String = ship.weapons.phaser_arrays[bank_idx].array_id
		if not array_id.is_empty():
			return array_id
	return "phaser_array_%d" % (bank_idx + 1)


func _on_beam_toggle(enabled: bool, bank_idx: int) -> void:
	var array_id := _beam_id(bank_idx)
	NetworkClient.send_action("phaser", "set_enabled", {"array_id": array_id, "enabled": enabled})
	_set_feedback("Beam %s %s." % [array_id, "enabled" if enabled else "disabled"], false)


func _on_beam_fire(bank_idx: int) -> void:
	if _locked_target_id.is_empty():
		_set_feedback("Lock a target before firing beams.", true)
		return
	var array_id := _beam_id(bank_idx)
	NetworkClient.send_action("phaser", "fire", {"array_id": array_id, "target_id": _locked_target_id})
	_set_feedback("Firing %s." % array_id, false)


func _on_tube_arm(enabled: bool, tube_idx: int) -> void:
	var bay_id := _bay_id(tube_idx)
	NetworkClient.send_action("torpedo", "arm", {"bay_id": bay_id, "armed": enabled})
	_set_feedback("Tube %d %s." % [bay_id, "ARMED" if enabled else "SAFE"], false)


func _on_tube_load(tube_idx: int) -> void:
	var bay_id := _bay_id(tube_idx)
	NetworkClient.send_action("torpedo", "load", {"bay_id": bay_id})
	_set_feedback("Loading tube %d." % bay_id, false)


func _on_tube_fire(tube_idx: int) -> void:
	if _locked_target_id.is_empty():
		_set_feedback("Lock a target before firing torpedoes.", true)
		return
	var ship := GameState.get_player_ship()
	if ship == null or tube_idx >= ship.weapons.torpedo_bays.size():
		return
	var bay = ship.weapons.torpedo_bays[tube_idx]
	var bay_id := _bay_id(tube_idx)
	if not bay.loaded:
		_set_feedback("Tube %d is empty: press LOAD first." % bay_id, true)
		return
	if not bay.armed:
		_set_feedback("Tube %d is not armed: toggle ARM first." % bay_id, true)
		return
	NetworkClient.send_action("torpedo", "fire", {"bay_id": bay_id, "target_id": _locked_target_id})
	_set_feedback("Torpedo away from tube %d." % bay_id, false)


func _on_fire_all_bays() -> void:
	if _locked_target_id.is_empty():
		_set_feedback("Lock a target before firing.", true)
		return
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	var fired := 0
	for i in range(ship.weapons.torpedo_bays.size()):
		var bay = ship.weapons.torpedo_bays[i]
		if not bay.loaded or not bay.armed or bay.cooldown > 0.0:
			continue
		NetworkClient.send_action("torpedo", "fire", {"bay_id": _bay_id(i), "target_id": _locked_target_id})
		fired += 1
	if fired == 0:
		_set_feedback("No ready tubes: ARM + LOAD at least one tube first.", true)
	else:
		_set_feedback("Firing %d tube(s)." % fired, false)


func _on_state_updated() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	var server_target: String = ship.target_id
	if server_target != _locked_target_id:
		_locked_target_id = server_target
		if not _locked_target_id.is_empty():
			_pending_selection = _locked_target_id


func _on_ship_removed(ship_id: String) -> void:
	if ship_id == _locked_target_id:
		_locked_target_id = ""
		_pending_selection = ""
		_contacts_sig = ""
