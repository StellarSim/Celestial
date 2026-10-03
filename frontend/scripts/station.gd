extends Control
## Base station controller that loads role-specific UI panels.
## Handles common station functionality like status bars, alerts, and network state.

const STATION_PANELS := {
	"engineer": "res://scenes/ui/engineer_panel.tscn",
	"flight": "res://scenes/ui/flight_panel.tscn",
	"weapons": "res://scenes/ui/weapons_panel.tscn",
	"captain": "res://scenes/ui/captain_panel.tscn",
	"communications": "res://scenes/ui/comms_panel.tscn",
	"operations": "res://scenes/ui/operations_panel.tscn",
	"relay": "res://scenes/ui/relay_panel.tscn",
	"first_officer": "res://scenes/ui/first_officer_panel.tscn",
}

const STATION_NAMES := {
	"engineer": "ENGINEER",
	"flight": "HELM",
	"weapons": "TACTICAL",
	"captain": "COMMAND",
	"communications": "COMMUNICATIONS",
	"operations": "OPERATIONS",
	"relay": "SCIENCE",
	"first_officer": "FIRST OFFICER",
}

@onready var station_label: Label = %StationLabel
@onready var ship_name_label: Label = %ShipName
@onready var ship_class_label: Label = %ShipClass
@onready var hull_bar: ProgressBar = %HullBar
@onready var shields_bar: ProgressBar = %ShieldsBar
@onready var power_bar: ProgressBar = %PowerBar
@onready var time_label: Label = %TimeLabel
@onready var alert_indicator: ColorRect = %AlertIndicator
@onready var station_content: Control = %StationContent
@onready var connection_status_dot: ColorRect = %StatusDot
@onready var connection_status_text: Label = %StatusText

@onready var debug_overlay: PanelContainer = $DebugOverlay
@onready var disconnect_overlay: ColorRect = $DisconnectOverlay
@onready var pause_overlay: ColorRect = $PauseOverlay
@onready var alert_overlay: ColorRect = $AlertOverlay

@onready var fps_label: Label = %FPSLabel
@onready var latency_label: Label = %LatencyLabel
@onready var ships_label: Label = %ShipsLabel
@onready var state_label: Label = %StateLabel
@onready var paused_label: Label = %PausedLabel

var _current_panel: Control = null
var _alert_tween: Tween = null


func _ready() -> void:
	_connect_signals()
	_load_station_panel()
	_update_station_label()


func _process(_delta: float) -> void:
	_update_status_bars()
	_update_time_display()
	
	if debug_overlay.visible:
		_update_debug_info()


func _input(event: InputEvent) -> void:
	if event.is_action_pressed("debug_toggle"):
		debug_overlay.visible = not debug_overlay.visible
	if NavUtils.is_menu_exit(event, get_viewport()):
		NavUtils.exit_to_menu(get_tree())


func _connect_signals() -> void:
	NetworkClient.connected.connect(_on_connected)
	NetworkClient.disconnected.connect(_on_disconnected)
	GameState.state_updated.connect(_on_state_updated)
	GameState.paused_changed.connect(_on_paused_changed)
	GameState.alert_level_changed.connect(_on_alert_changed)

	%RedAlertBtn.pressed.connect(_on_red_alert_pressed)
	%YellowAlertBtn.pressed.connect(_on_yellow_alert_pressed)


func _load_station_panel() -> void:
	var role := GameState.client_role
	
	if not STATION_PANELS.has(role):
		push_error("Unknown station role: %s" % role)
		return
	
	var panel_path: String = STATION_PANELS[role]
	
	if not ResourceLoader.exists(panel_path):
		push_error("Station panel scene missing: %s" % panel_path)
		return
	
	var panel_scene := load(panel_path) as PackedScene
	if panel_scene == null:
		push_error("Station panel scene failed to load: %s" % panel_path)
		return
	
	_current_panel = panel_scene.instantiate()
	station_content.add_child(_current_panel)
	_current_panel.set_anchors_preset(Control.PRESET_FULL_RECT)


func _update_station_label() -> void:
	var role := GameState.client_role
	station_label.text = STATION_NAMES.get(role, role.to_upper())


func _update_status_bars() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	# Update ship info
	ship_name_label.text = ship.name.to_upper() if not ship.name.is_empty() else "USS UNKNOWN"
	ship_class_label.text = ship.ship_class.replace("_", " ").capitalize()
	
	# Update hull
	var hull_percent: float = (ship.hull_integrity / ship.max_hull) * 100.0 if ship.max_hull > 0 else 0.0
	hull_bar.value = hull_percent
	hull_bar.modulate = Colors.get_health_color(hull_percent / 100.0)
	
	# Update shields
	var shields_percent: float = (ship.shields / ship.max_shields) * 100.0 if ship.max_shields > 0 else 0.0
	shields_bar.value = shields_percent
	shields_bar.modulate = Colors.get_shield_color(shields_percent / 100.0)
	
	# Update power
	var power_percent: float = (ship.power_available / ship.power_total) * 100.0 if ship.power_total > 0 else 0.0
	power_bar.value = power_percent
	power_bar.modulate = Colors.get_power_color(power_percent / 100.0)


func _update_time_display() -> void:
	time_label.text = NavUtils.format_clock(GameState.simulation_time)


func _update_debug_info() -> void:
	fps_label.text = "FPS: %d" % Engine.get_frames_per_second()
	latency_label.text = "Latency: %.0fms" % NetworkClient.latency_ms
	ships_label.text = "Ships: %d" % GameState.ships.size()
	state_label.text = "State: " + NetworkClient.get_connection_status()
	paused_label.text = "Paused: %s" % ("Yes" if GameState.is_paused else "No")


func _on_connected() -> void:
	NavUtils.set_connection_state(disconnect_overlay, true, connection_status_dot, connection_status_text)


func _on_disconnected() -> void:
	NavUtils.set_connection_state(disconnect_overlay, false, connection_status_dot, connection_status_text)


func _on_state_updated() -> void:
	pass  # Individual panels handle their own updates


func _on_paused_changed(is_paused: bool) -> void:
	pause_overlay.visible = is_paused


func _on_alert_changed(level: String) -> void:
	if _alert_tween:
		_alert_tween.kill()
	
	match level:
		"red":
			alert_indicator.color = Colors.ALERT_RED
			_start_alert_flash(Colors.ALERT_RED)
		"yellow":
			alert_indicator.color = Colors.ALERT_YELLOW
			_start_alert_flash(Colors.ALERT_YELLOW)
		_:
			alert_indicator.color = Colors.ALERT_GREEN
			alert_overlay.visible = false


func _start_alert_flash(color: Color) -> void:
	if _alert_tween:
		_alert_tween.kill()
	_alert_tween = NavUtils.start_alert_flash(self, alert_overlay, color, 0.15, 0.5)


func _on_red_alert_pressed() -> void:
	NetworkClient.send_action("alert", "set_level", {"level": "red"})


func _on_yellow_alert_pressed() -> void:
	NetworkClient.send_action("alert", "set_level", {"level": "yellow"})
