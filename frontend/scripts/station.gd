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

@onready var station_label: Label = $MainLayout/TopBar/TopBarContent/StationLabel
@onready var ship_name_label: Label = $MainLayout/TopBar/TopBarContent/ShipInfo/ShipName
@onready var ship_class_label: Label = $MainLayout/TopBar/TopBarContent/ShipInfo/ShipClass
@onready var hull_bar: ProgressBar = $MainLayout/TopBar/TopBarContent/StatusPanel/HullStatus/HullBar
@onready var shields_bar: ProgressBar = $MainLayout/TopBar/TopBarContent/StatusPanel/ShieldsStatus/ShieldsBar
@onready var power_bar: ProgressBar = $MainLayout/TopBar/TopBarContent/StatusPanel/PowerStatus/PowerBar
@onready var time_label: Label = $MainLayout/TopBar/TopBarContent/TimeLabel
@onready var alert_indicator: ColorRect = $MainLayout/TopBar/TopBarContent/AlertIndicator
@onready var station_content: Control = $MainLayout/ContentArea/StationContent
@onready var connection_status_dot: ColorRect = $MainLayout/BottomBar/BottomBarContent/ConnectionStatus/StatusDot
@onready var connection_status_text: Label = $MainLayout/BottomBar/BottomBarContent/ConnectionStatus/StatusText

@onready var debug_overlay: PanelContainer = $DebugOverlay
@onready var disconnect_overlay: ColorRect = $DisconnectOverlay
@onready var pause_overlay: ColorRect = $PauseOverlay
@onready var alert_overlay: ColorRect = $AlertOverlay
@onready var damage_effects: Control = $DamageEffects

@onready var fps_label: Label = $DebugOverlay/DebugContent/FPSLabel
@onready var latency_label: Label = $DebugOverlay/DebugContent/LatencyLabel
@onready var ships_label: Label = $DebugOverlay/DebugContent/ShipsLabel
@onready var state_label: Label = $DebugOverlay/DebugContent/StateLabel
@onready var paused_label: Label = $DebugOverlay/DebugContent/PausedLabel

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


func _connect_signals() -> void:
	NetworkClient.connected.connect(_on_connected)
	NetworkClient.disconnected.connect(_on_disconnected)
	GameState.state_updated.connect(_on_state_updated)
	GameState.paused_changed.connect(_on_paused_changed)
	GameState.alert_level_changed.connect(_on_alert_changed)
	
	$MainLayout/BottomBar/BottomBarContent/BackButton.pressed.connect(_on_back_pressed)
	$MainLayout/BottomBar/BottomBarContent/AlertButtons/RedAlertBtn.pressed.connect(_on_red_alert_pressed)
	$MainLayout/BottomBar/BottomBarContent/AlertButtons/YellowAlertBtn.pressed.connect(_on_yellow_alert_pressed)
	$DisconnectOverlay/DisconnectContent/MenuButton.pressed.connect(_on_back_pressed)


func _load_station_panel() -> void:
	var role := GameState.client_role
	
	if not STATION_PANELS.has(role):
		push_warning("Unknown station role: ", role)
		return
	
	var panel_path: String = STATION_PANELS[role]
	
	if not ResourceLoader.exists(panel_path):
		push_warning("Station panel not found: ", panel_path)
		_create_placeholder_panel(role)
		return
	
	var panel_scene := load(panel_path) as PackedScene
	if panel_scene:
		_current_panel = panel_scene.instantiate()
		station_content.add_child(_current_panel)
		_current_panel.set_anchors_preset(Control.PRESET_FULL_RECT)


func _create_placeholder_panel(role: String) -> void:
	var placeholder := Label.new()
	placeholder.text = "Station panel for '%s' not yet implemented" % role
	placeholder.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	placeholder.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	station_content.add_child(placeholder)
	placeholder.set_anchors_preset(Control.PRESET_FULL_RECT)


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
	var total_seconds := int(GameState.simulation_time)
	var hours := total_seconds / 3600
	var minutes := (total_seconds % 3600) / 60
	var seconds := total_seconds % 60
	time_label.text = "%02d:%02d:%02d" % [hours, minutes, seconds]


func _update_debug_info() -> void:
	fps_label.text = "FPS: %d" % Engine.get_frames_per_second()
	latency_label.text = "Latency: %.0fms" % NetworkClient.latency_ms
	ships_label.text = "Ships: %d" % GameState.ships.size()
	state_label.text = "State: " + NetworkClient.get_connection_status()
	paused_label.text = "Paused: %s" % ("Yes" if GameState.is_paused else "No")


func _on_connected() -> void:
	disconnect_overlay.visible = false
	connection_status_dot.color = Colors.STATUS_ONLINE
	connection_status_text.text = "Connected"


func _on_disconnected() -> void:
	disconnect_overlay.visible = true
	connection_status_dot.color = Colors.STATUS_OFFLINE
	connection_status_text.text = "Disconnected"


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
	alert_overlay.visible = true
	alert_overlay.color = Color(color.r, color.g, color.b, 0.0)
	
	_alert_tween = create_tween().set_loops()
	_alert_tween.tween_property(alert_overlay, "color:a", 0.15, 0.5)
	_alert_tween.tween_property(alert_overlay, "color:a", 0.0, 0.5)


func _on_back_pressed() -> void:
	NetworkClient.disconnect_from_server()
	get_tree().change_scene_to_file("res://scenes/main.tscn")


func _on_red_alert_pressed() -> void:
	NetworkClient.send_action("alert", "set_level", "red")


func _on_yellow_alert_pressed() -> void:
	NetworkClient.send_action("alert", "set_level", "yellow")
