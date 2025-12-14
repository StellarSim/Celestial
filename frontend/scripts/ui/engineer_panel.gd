extends Control
## Engineer station panel for power management and damage control.

const BREAKERS := ["reactor", "engines", "shields", "weapons", "sensors", "comms", "life_support", "navigation"]
const SECTIONS := ["bow", "stern", "port", "starboard"]

@onready var generation_value: Label = $MainSplit/RightSection/PowerSection/PowerContent/PowerOverview/GenerationPanel/GenerationValue
@onready var consumption_value: Label = $MainSplit/RightSection/PowerSection/PowerContent/PowerOverview/ConsumptionPanel/ConsumptionValue
@onready var available_value: Label = $MainSplit/RightSection/PowerSection/PowerContent/PowerOverview/AvailablePanel/AvailableValue

@onready var breaker_grid: GridContainer = $MainSplit/RightSection/PowerSection/PowerContent/BreakerGrid

@onready var bow_section: Button = $MainSplit/LeftSection/DamageSection/DamageContent/ShipDiagram/BowSection
@onready var stern_section: Button = $MainSplit/LeftSection/DamageSection/DamageContent/ShipDiagram/SternSection
@onready var port_section: Button = $MainSplit/LeftSection/DamageSection/DamageContent/ShipDiagram/PortSection
@onready var starboard_section: Button = $MainSplit/LeftSection/DamageSection/DamageContent/ShipDiagram/StarboardSection

@onready var repair_btn: Button = $MainSplit/LeftSection/DamageSection/DamageContent/RepairActions/RepairBtn
@onready var seal_breach_btn: Button = $MainSplit/LeftSection/DamageSection/DamageContent/RepairActions/SealBreachBtn
@onready var extinguish_btn: Button = $MainSplit/LeftSection/DamageSection/DamageContent/RepairActions/ExtinguishBtn

@onready var damage_log: ItemList = $MainSplit/LeftSection/DamageLog/DamageLogContent/LogList

@onready var engine_health_bar: ProgressBar = $MainSplit/RightSection/EngineSection/EngineContent/EngineHealth/EngineHealthBar
@onready var engine_temp_bar: ProgressBar = $MainSplit/RightSection/EngineSection/EngineContent/EngineTemp/EngineTempBar
@onready var thrust_value: Label = $MainSplit/RightSection/EngineSection/EngineContent/ThrustOutput/ThrustValue

var _selected_section: String = ""
var _breaker_toggles: Dictionary = {}
var _breaker_statuses: Dictionary = {}


func _ready() -> void:
	_setup_breakers()
	_connect_signals()


func _process(_delta: float) -> void:
	_update_display()


func _setup_breakers() -> void:
	var breaker_containers := breaker_grid.get_children()
	
	for i in range(min(breaker_containers.size(), BREAKERS.size())):
		var container := breaker_containers[i] as HBoxContainer
		var breaker_name: String = BREAKERS[i]
		
		var toggle := container.get_child(0) as CheckButton
		var status := container.get_child(1) as Label
		
		_breaker_toggles[breaker_name] = toggle
		_breaker_statuses[breaker_name] = status
		
		toggle.toggled.connect(_on_breaker_toggled.bind(breaker_name))


func _connect_signals() -> void:
	GameState.state_updated.connect(_on_state_updated)
	
	bow_section.pressed.connect(func(): _select_section("bow"))
	stern_section.pressed.connect(func(): _select_section("stern"))
	port_section.pressed.connect(func(): _select_section("port"))
	starboard_section.pressed.connect(func(): _select_section("starboard"))
	
	repair_btn.pressed.connect(_on_repair_pressed)
	seal_breach_btn.pressed.connect(_on_seal_breach_pressed)
	extinguish_btn.pressed.connect(_on_extinguish_pressed)


func _update_display() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	_update_power_display(ship)
	_update_breakers(ship)
	_update_damage_sections(ship)
	_update_engine_status(ship)


func _update_power_display(ship: GameState.ShipState) -> void:
	var generation := ship.power_total
	var available := ship.power_available
	var consumption := generation - available
	
	generation_value.text = "%.0f MW" % generation
	consumption_value.text = "%.0f MW" % consumption
	available_value.text = "%.0f MW" % available
	
	# Color code available power
	var power_ratio: float = available / generation if generation > 0 else 0.0
	available_value.add_theme_color_override("font_color", Colors.get_power_color(power_ratio))


func _update_breakers(ship: GameState.ShipState) -> void:
	for breaker_name in _breaker_toggles:
		var toggle: CheckButton = _breaker_toggles[breaker_name]
		var status: Label = _breaker_statuses[breaker_name]
		
		var is_on: bool = ship.power_breakers.get(breaker_name, true)
		
		# Update toggle without triggering signal
		toggle.set_pressed_no_signal(is_on)
		
		if is_on:
			status.text = "ONLINE"
			status.add_theme_color_override("font_color", Colors.STATUS_ONLINE)
		else:
			status.text = "OFFLINE"
			status.add_theme_color_override("font_color", Colors.STATUS_OFFLINE)


func _update_damage_sections(ship: GameState.ShipState) -> void:
	_update_section_button(bow_section, "BOW", ship.damage_sections.get("bow"))
	_update_section_button(stern_section, "STERN", ship.damage_sections.get("stern"))
	_update_section_button(port_section, "PORT", ship.damage_sections.get("port"))
	_update_section_button(starboard_section, "STARBOARD", ship.damage_sections.get("starboard"))


func _update_section_button(button: Button, name: String, section) -> void:
	var health := 100.0
	var fires := 0
	var breaches := 0
	
	if section != null:
		health = section.health
		fires = section.fires
		breaches = section.breaches
	
	var status_text := "%s\n%.0f%%" % [name, health]
	
	if fires > 0:
		status_text += "\n🔥 %d" % fires
	if breaches > 0:
		status_text += "\n⚠ %d" % breaches
	
	button.text = status_text
	
	# Color based on health
	var color := Colors.get_damage_section_color(health / 100.0)
	var style := StyleBoxFlat.new()
	style.bg_color = Color(color.r * 0.3, color.g * 0.3, color.b * 0.3, 0.8)
	style.border_color = color
	style.set_border_width_all(2)
	style.set_corner_radius_all(4)
	button.add_theme_stylebox_override("normal", style)


func _update_engine_status(ship: GameState.ShipState) -> void:
	var engine_health := ship.engines.health
	engine_health_bar.value = engine_health
	engine_health_bar.modulate = Colors.get_health_color(engine_health / 100.0)
	
	# Calculate thrust percentage from velocity
	var thrust := ship.engines.thrust.to_vector3().length()
	var max_thrust := 10000.0  # Approximate max thrust
	var thrust_percent := (thrust / max_thrust) * 100.0
	thrust_value.text = "%.0f%%" % clampf(thrust_percent, 0, 100)
	
	# Temperature (simulated based on thrust)
	engine_temp_bar.value = thrust_percent * 0.6 + 20.0


func _select_section(section: String) -> void:
	_selected_section = section
	
	# Update button states to show selection
	bow_section.button_pressed = section == "bow"
	stern_section.button_pressed = section == "stern"
	port_section.button_pressed = section == "port"
	starboard_section.button_pressed = section == "starboard"
	
	# Enable action buttons based on section state
	var ship := GameState.get_player_ship()
	if ship and ship.damage_sections.has(section):
		var sec = ship.damage_sections[section]
		repair_btn.disabled = sec.health >= 100.0
		seal_breach_btn.disabled = sec.breaches <= 0
		extinguish_btn.disabled = sec.fires <= 0
	else:
		repair_btn.disabled = true
		seal_breach_btn.disabled = true
		extinguish_btn.disabled = true


func _on_breaker_toggled(enabled: bool, breaker_name: String) -> void:
	NetworkClient.send_action("power", "toggle_breaker", {
		"breaker": breaker_name,
		"enabled": enabled
	})


func _on_repair_pressed() -> void:
	if _selected_section.is_empty():
		return
	NetworkClient.send_action("damage", "repair", {"section": _selected_section})
	_add_log_entry("Initiating repairs on %s section" % _selected_section.to_upper())


func _on_seal_breach_pressed() -> void:
	if _selected_section.is_empty():
		return
	NetworkClient.send_action("damage", "seal_breach", {"section": _selected_section})
	_add_log_entry("Sealing breach in %s section" % _selected_section.to_upper())


func _on_extinguish_pressed() -> void:
	if _selected_section.is_empty():
		return
	NetworkClient.send_action("damage", "extinguish", {"section": _selected_section})
	_add_log_entry("Extinguishing fire in %s section" % _selected_section.to_upper())


func _on_state_updated() -> void:
	pass  # Updates handled in _process


func _add_log_entry(message: String) -> void:
	var time_str := Time.get_time_string_from_system()
	damage_log.add_item("[%s] %s" % [time_str, message])
	
	# Auto-scroll to bottom
	if damage_log.item_count > 0:
		damage_log.ensure_current_is_visible()
	
	# Limit log entries
	while damage_log.item_count > 50:
		damage_log.remove_item(0)
