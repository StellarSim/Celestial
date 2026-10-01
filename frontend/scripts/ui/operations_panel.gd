extends StationPanel
## Operations station panel - sensors, shields, and ship systems monitoring.

const SYSTEMS := ["reactor", "engines", "weapons", "shields", "sensors", "life_support"]

@onready var short_range_value: Label = $MainSplit/LeftSection/SensorSection/SensorContent/SensorStatus/ShortRangeValue
@onready var long_range_value: Label = $MainSplit/LeftSection/SensorSection/SensorContent/SensorStatus/LongRangeValue
@onready var resolution_value: Label = $MainSplit/LeftSection/SensorSection/SensorContent/SensorStatus/ResolutionValue

@onready var passive_btn: Button = $MainSplit/LeftSection/SensorSection/SensorContent/SensorModes/PassiveBtn
@onready var active_btn: Button = $MainSplit/LeftSection/SensorSection/SensorContent/SensorModes/ActiveBtn
@onready var deep_scan_btn: Button = $MainSplit/LeftSection/SensorSection/SensorContent/SensorModes/DeepScanBtn
@onready var scan_progress: ProgressBar = $MainSplit/LeftSection/SensorSection/SensorContent/ScanProgress
@onready var contact_list: ItemList = $MainSplit/LeftSection/SensorSection/SensorContent/ContactList

@onready var shield_diagram: Control = $MainSplit/LeftSection/ShieldSection/ShieldContent/ShieldDiagram
@onready var shields_up_btn: Button = $MainSplit/LeftSection/ShieldSection/ShieldContent/ShieldControls/ShieldsUp
@onready var shields_down_btn: Button = $MainSplit/LeftSection/ShieldSection/ShieldContent/ShieldControls/ShieldsDown
@onready var shield_freq_slider: HSlider = $MainSplit/LeftSection/ShieldSection/ShieldContent/ShieldFrequency/FreqSlider
@onready var rotate_btn: Button = $MainSplit/LeftSection/ShieldSection/ShieldContent/ShieldFrequency/RotateBtn

@onready var systems_list: VBoxContainer = $MainSplit/RightSection/SystemsSection/SystemsContent/SystemsList

@onready var transporter_status: Label = $MainSplit/RightSection/TransporterSection/TransporterContent/TransporterStatus/StatusValue
@onready var beam_up_btn: Button = $MainSplit/RightSection/TransporterSection/TransporterContent/TransporterControls/BeamUpBtn
@onready var beam_down_btn: Button = $MainSplit/RightSection/TransporterSection/TransporterContent/TransporterControls/BeamDownBtn
@onready var emergency_btn: Button = $MainSplit/RightSection/TransporterSection/TransporterContent/TransporterControls/EmergencyBtn

var _sensor_mode: String = "passive"
var _shields_enabled: bool = true
var _system_bars: Dictionary = {}


func _ready() -> void:
	super._ready()
	_setup_system_bars()
	_connect_signals()
	passive_btn.button_pressed = true


func _process(_delta: float) -> void:
	_update_display()
	shield_diagram.queue_redraw()


func _setup_system_bars() -> void:
	# Map system names to their UI elements
	for i in range(systems_list.get_child_count()):
		var system_row := systems_list.get_child(i) as HBoxContainer
		var label := system_row.get_child(0) as Label
		var bar := system_row.get_child(1) as ProgressBar
		var value := system_row.get_child(2) as Label
		
		var system_name := label.text.to_lower().replace(" ", "_")
		_system_bars[system_name] = {
			"bar": bar,
			"value": value
		}


func _connect_signals() -> void:
	
	passive_btn.pressed.connect(func(): _set_sensor_mode("passive"))
	active_btn.pressed.connect(func(): _set_sensor_mode("active"))
	deep_scan_btn.pressed.connect(func(): _set_sensor_mode("deep_scan"))
	
	shields_up_btn.pressed.connect(_on_shields_up)
	shields_down_btn.pressed.connect(_on_shields_down)
	shield_freq_slider.value_changed.connect(_on_shield_freq_changed)
	rotate_btn.pressed.connect(_on_rotate_shields)
	
	beam_up_btn.pressed.connect(_on_beam_up)
	beam_down_btn.pressed.connect(_on_beam_down)
	emergency_btn.pressed.connect(_on_emergency_transport)
	
	contact_list.item_selected.connect(_on_contact_selected)
	shield_diagram.draw.connect(_draw_shield_diagram)


func _update_display() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	_update_sensor_status(ship)
	_update_contact_list()
	_update_systems(ship)
	_update_transporter_status(ship)
	_update_scan_progress(ship)


func _update_sensor_status(ship: GameState.ShipState) -> void:
	if ship.sensors.enabled and ship.power_breakers.get("sensors", true):
		short_range_value.text = "ONLINE"
		short_range_value.add_theme_color_override("font_color", Colors.STATUS_ONLINE)
		long_range_value.text = "ONLINE"
		long_range_value.add_theme_color_override("font_color", Colors.STATUS_ONLINE)
		resolution_value.text = "100%"
	else:
		short_range_value.text = "OFFLINE"
		short_range_value.add_theme_color_override("font_color", Colors.STATUS_OFFLINE)
		long_range_value.text = "OFFLINE"
		long_range_value.add_theme_color_override("font_color", Colors.STATUS_OFFLINE)
		resolution_value.text = "0%"


func _update_contact_list() -> void:
	var player_ship := GameState.get_player_ship()
	if player_ship == null:
		return

	contact_list.clear()
	var player_pos := player_ship.position.to_vector3()

	for entry in NavUtils.contacts_by_distance():
		var ship: GameState.ShipState = entry.ship

		var faction_color: Color = Colors.get_faction_color(ship.faction)
		var bearing := NavUtils.bearing_to(player_pos, ship.position.to_vector3())

		var display := "%s | %.1f km | %03.0f°" % [ship.name, entry.distance / 1000.0, bearing]
		var idx := contact_list.add_item(display)
		contact_list.set_item_custom_fg_color(idx, faction_color)
		contact_list.set_item_metadata(idx, entry.id)


func _update_scan_progress(ship: GameState.ShipState) -> void:
	# Scan progress is server owned; the client only renders it.
	scan_progress.value = ship.sensors.scan_progress * 100.0
	scan_progress.modulate = Colors.get_health_color(ship.sensors.scan_progress)


func _update_systems(ship: GameState.ShipState) -> void:
	for system_name in _system_bars:
		var bar: ProgressBar = _system_bars[system_name].bar
		var value: Label = _system_bars[system_name].value
		
		var health := _system_health(ship, system_name)
		var enabled: bool = ship.power_breakers.get(system_name, true)
		
		if not enabled:
			health = 0
		
		bar.value = health
		bar.modulate = Colors.get_health_color(health / 100.0)
		value.text = "%.0f%%" % health
		
		if not enabled:
			value.text = "OFF"
			value.add_theme_color_override("font_color", Colors.STATUS_OFFLINE)
		else:
			value.add_theme_color_override("font_color", Colors.get_health_color(health / 100.0))


func _system_health(ship: GameState.ShipState, system_name: String) -> float:
	match system_name:
		"sensors":
			return ship.sensors.health
		"shields":
			var total := 0.0
			for facing in ship.shield_facings:
				total += float(ship.shield_facings[facing])
			return total / max(ship.shield_facings.size(), 1) / 2.0
		"life_support":
			return ship.life_support.oxygen_level
		"reactor":
			return ship.power_total
		"weapons", "engines":
			var worst := 100.0
			if system_name == "weapons":
				for bay in ship.weapons.torpedo_bays:
					worst = minf(worst, 100.0)
			else:
				for engine in ship.engines_list:
					worst = minf(worst, engine.health)
			return worst
	# Anything without a dedicated reading falls back to the worst hull section.
	var worst_section := 100.0
	for section in ship.damage_sections:
		worst_section = minf(worst_section, ship.damage_sections[section].health)
	return worst_section


func _update_transporter_status(ship: GameState.ShipState) -> void:
	var power_ok: bool = ship.power_breakers.get("life_support", true)
	var shields_up := ship.shields_enabled
	
	if not power_ok:
		transporter_status.text = "NO POWER"
		transporter_status.add_theme_color_override("font_color", Colors.STATUS_OFFLINE)
		beam_up_btn.disabled = true
		beam_down_btn.disabled = true
	elif ship.transporter_emergency:
		transporter_status.text = "EMERGENCY"
		transporter_status.add_theme_color_override("font_color", Colors.ALERT_RED)
		beam_up_btn.disabled = true
		beam_down_btn.disabled = false
	elif ship.transporter_active:
		transporter_status.text = "BEAM ACTIVE"
		transporter_status.add_theme_color_override("font_color", Colors.ALERT_YELLOW)
		beam_up_btn.disabled = true
		beam_down_btn.disabled = false
	elif shields_up:
		transporter_status.text = "SHIELDS UP"
		transporter_status.add_theme_color_override("font_color", Colors.ALERT_YELLOW)
		beam_up_btn.disabled = true
		beam_down_btn.disabled = true
	else:
		transporter_status.text = "READY"
		transporter_status.add_theme_color_override("font_color", Colors.STATUS_ONLINE)
		beam_up_btn.disabled = false
		beam_down_btn.disabled = false


func _draw_shield_diagram() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	var center := shield_diagram.size / 2
	var radius: float = min(center.x, center.y) - 20
	
	# Draw ship representation
	var ship_points := PackedVector2Array([
		center + Vector2(0, -radius * 0.4),
		center + Vector2(-radius * 0.2, radius * 0.3),
		center + Vector2(radius * 0.2, radius * 0.3)
	])
	shield_diagram.draw_colored_polygon(ship_points, Color(0.3, 0.3, 0.3))
	shield_diagram.draw_polyline(ship_points + PackedVector2Array([ship_points[0]]), Colors.PRIMARY, 2.0)
	
	# Draw shield facings
	var facings := ["fore", "aft", "port", "starboard"]
	var angles := [PI * 1.5, PI * 0.5, PI, 0.0]  # Top, bottom, left, right
	var arc_span := PI * 0.4
	
	for i in range(facings.size()):
		var facing: String = facings[i]
		var angle: float = angles[i]
		var shield_val: float = ship.shield_facings.get(facing, 100.0)
		var max_per_facing := ship.max_shields / 4.0 if ship.max_shields > 0 else 250.0
		var strength: float = clampf(shield_val / max_per_facing, 0.0, 1.0)
		
		var color := Colors.get_shield_color(strength)
		if not ship.shields_enabled:
			color = Color(0.3, 0.3, 0.3, 0.5)
		
		shield_diagram.draw_arc(center, radius, angle - arc_span/2, angle + arc_span/2, 16, color, 8.0 * strength + 2.0)


func _set_sensor_mode(mode: String) -> void:
	_sensor_mode = mode
	
	passive_btn.button_pressed = mode == "passive"
	active_btn.button_pressed = mode == "active"
	deep_scan_btn.button_pressed = mode == "deep_scan"
	
	NetworkClient.send_action("sensors", "set_mode", {"mode": mode})


func _on_contact_selected(idx: int) -> void:
	var target_id: String = contact_list.get_item_metadata(idx)
	if _sensor_mode == "deep_scan":
		NetworkClient.send_action("sensors", "deep_scan", {"target_id": target_id})
	else:
		NetworkClient.send_action("sensors", "initiate_scan", {"target_id": target_id})


func _on_shields_up() -> void:
	NetworkClient.send_action("shields", "raise", {})


func _on_shields_down() -> void:
	NetworkClient.send_action("shields", "lower", {})


func _on_shield_freq_changed(value: float) -> void:
	NetworkClient.send_action("shields", "set_frequency", {"frequency": value})


func _on_rotate_shields() -> void:
	NetworkClient.send_action("shields", "rotate_frequency", {})


func _on_beam_up() -> void:
	NetworkClient.send_action("transporter", "beam_up", {})


func _on_beam_down() -> void:
	NetworkClient.send_action("transporter", "beam_down", {})


func _on_emergency_transport() -> void:
	NetworkClient.send_action("transporter", "emergency", {})

