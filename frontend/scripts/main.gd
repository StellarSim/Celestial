extends Control
## Main menu and client configuration screen.
## Handles server connection and role selection before launching the appropriate client.

const STATION_ROLES := [
	{"id": "engineer", "name": "Engineer", "desc": "Power & Damage Control"},
	{"id": "flight", "name": "Flight", "desc": "Helm & Navigation"},
	{"id": "weapons", "name": "Weapons", "desc": "Tactical Systems"},
	{"id": "captain", "name": "Captain", "desc": "Command"},
	{"id": "communications", "name": "Comms", "desc": "Communications"},
	{"id": "operations", "name": "Operations", "desc": "Shields & Resources"},
	{"id": "relay", "name": "Relay", "desc": "Sensors & Science"},
	{"id": "first_officer", "name": "First Officer", "desc": "Tactical Analysis"},
]

const DISPLAY_ROLES := [
	{"id": "viewscreen", "name": "Main Viewscreen", "desc": "3D Bridge Display"},
	{"id": "tactical_display", "name": "Tactical Display", "desc": "Rear Overview"},
	{"id": "gm", "name": "Game Master", "desc": "Mission Control"},
]

@onready var server_input: LineEdit = %ServerInput
@onready var port_input: LineEdit = %PortInput
@onready var connection_status: Label = %ConnectionStatus
@onready var connect_button: Button = %ConnectButton
@onready var role_grid: GridContainer = %RoleGrid
@onready var debug_overlay: PanelContainer = %DebugOverlay
@onready var connection_overlay: ColorRect = %ConnectionOverlay

@onready var graphics_low: Button = %LowBtn
@onready var graphics_med: Button = %MedBtn
@onready var graphics_high: Button = %HighBtn
@onready var fullscreen_check: CheckButton = %FullscreenCheck
@onready var vsync_check: CheckButton = %VsyncCheck
@onready var master_slider: HSlider = %MasterSlider
@onready var sfx_slider: HSlider = %SFXSlider
@onready var music_slider: HSlider = %MusicSlider

@onready var fps_label: Label = %FPSLabel
@onready var latency_label: Label = %LatencyLabel
@onready var packets_label: Label = %PacketsLabel
@onready var state_label: Label = %StateLabel

var _selected_role: String = ""
var _role_buttons: Dictionary = {}


func _ready() -> void:
	_load_settings()
	_setup_role_buttons()
	_connect_signals()
	_update_ui_state()
	_create_star_background()


func _process(_delta: float) -> void:
	if debug_overlay.visible:
		_update_debug_info()


func _input(event: InputEvent) -> void:
	if event.is_action_pressed("debug_toggle"):
		debug_overlay.visible = not debug_overlay.visible


func _load_settings() -> void:
	server_input.text = Config.server_address
	port_input.text = str(Config.server_port)
	
	graphics_low.button_pressed = Config.graphics_quality == 0
	graphics_med.button_pressed = Config.graphics_quality == 1
	graphics_high.button_pressed = Config.graphics_quality == 2
	
	fullscreen_check.button_pressed = Config.fullscreen
	vsync_check.button_pressed = Config.vsync
	
	master_slider.value = Config.master_volume * 100
	sfx_slider.value = Config.sfx_volume * 100
	music_slider.value = Config.music_volume * 100


func _setup_role_buttons() -> void:
	# Clear existing buttons
	for child in role_grid.get_children():
		child.queue_free()
	
	# Create station role buttons
	for role_data in STATION_ROLES:
		var btn := _create_role_button(role_data)
		role_grid.add_child(btn)
		_role_buttons[role_data.id] = btn
	
	# Add separator
	var separator := HSeparator.new()
	separator.custom_minimum_size.x = 350
	role_grid.add_child(separator)
	var separator2 := HSeparator.new()
	separator2.custom_minimum_size.x = 350
	role_grid.add_child(separator2)
	
	# Create display role buttons
	for role_data in DISPLAY_ROLES:
		var btn := _create_role_button(role_data)
		role_grid.add_child(btn)
		_role_buttons[role_data.id] = btn


func _create_role_button(role_data: Dictionary) -> Button:
	var btn := Button.new()
	btn.custom_minimum_size = Vector2(180, 50)
	btn.text = role_data.name
	btn.tooltip_text = role_data.desc
	btn.toggle_mode = true
	btn.pressed.connect(_on_role_selected.bind(role_data.id))
	return btn


func _connect_signals() -> void:
	connect_button.pressed.connect(_on_connect_pressed)
	
	NetworkClient.connected.connect(_on_network_connected)
	NetworkClient.disconnected.connect(_on_network_disconnected)
	NetworkClient.registered.connect(_on_network_registered)
	NetworkClient.connection_error.connect(_on_connection_error)
	
	graphics_low.pressed.connect(func(): _set_graphics_quality(0))
	graphics_med.pressed.connect(func(): _set_graphics_quality(1))
	graphics_high.pressed.connect(func(): _set_graphics_quality(2))
	
	fullscreen_check.toggled.connect(_on_fullscreen_toggled)
	vsync_check.toggled.connect(_on_vsync_toggled)
	
	master_slider.value_changed.connect(_on_master_changed)
	sfx_slider.value_changed.connect(_on_sfx_changed)
	music_slider.value_changed.connect(_on_music_changed)
	
	$ConnectionOverlay/ConnectionContent/CancelButton.pressed.connect(_on_cancel_connection)


func _update_ui_state() -> void:
	var is_connected := NetworkClient.state >= NetworkClient.ConnectionState.CONNECTED
	
	for role_id in _role_buttons:
		_role_buttons[role_id].disabled = not is_connected
	
	match NetworkClient.state:
		NetworkClient.ConnectionState.DISCONNECTED:
			connection_status.text = "Disconnected"
			connection_status.add_theme_color_override("font_color", Color(0.6, 0.6, 0.6))
			connect_button.text = "CONNECT"
			connect_button.disabled = false
		NetworkClient.ConnectionState.CONNECTING:
			connection_status.text = "Connecting..."
			connection_status.add_theme_color_override("font_color", Color(0.9, 0.7, 0.2))
			connect_button.text = "CONNECTING..."
			connect_button.disabled = true
		NetworkClient.ConnectionState.CONNECTED:
			connection_status.text = "Connected"
			connection_status.add_theme_color_override("font_color", Color(0.3, 0.8, 0.4))
			connect_button.text = "DISCONNECT"
			connect_button.disabled = false
		NetworkClient.ConnectionState.REGISTERED:
			connection_status.text = "Registered"
			connection_status.add_theme_color_override("font_color", Color(0.3, 0.9, 0.5))
			connect_button.text = "DISCONNECT"
			connect_button.disabled = false


func _on_connect_pressed() -> void:
	if NetworkClient.state >= NetworkClient.ConnectionState.CONNECTED:
		NetworkClient.disconnect_from_server()
	else:
		Config.set_server(server_input.text, int(port_input.text))
		connection_overlay.visible = true
		NetworkClient.connect_to_server(Config.get_server_url())


func _on_network_connected() -> void:
	connection_overlay.visible = false
	_update_ui_state()


func _on_network_disconnected() -> void:
	connection_overlay.visible = false
	_selected_role = ""
	_update_ui_state()
	
	# Reset role button states
	for role_id in _role_buttons:
		_role_buttons[role_id].button_pressed = false


func _on_network_registered() -> void:
	_update_ui_state()
	_launch_client()


func _on_connection_error(message: String) -> void:
	connection_overlay.visible = false
	connection_status.text = "Error: " + message
	connection_status.add_theme_color_override("font_color", Color(0.9, 0.3, 0.2))
	_update_ui_state()


func _on_cancel_connection() -> void:
	NetworkClient.disconnect_from_server()
	connection_overlay.visible = false


func _on_role_selected(role_id: String) -> void:
	# Deselect other buttons
	for rid in _role_buttons:
		if rid != role_id:
			_role_buttons[rid].button_pressed = false
	
	_selected_role = role_id
	
	# Register with server
	if NetworkClient.state >= NetworkClient.ConnectionState.CONNECTED:
		NetworkClient.register(role_id)


func _launch_client() -> void:
	var scene_path: String
	
	match _selected_role:
		"viewscreen":
			scene_path = "res://scenes/viewscreen.tscn"
		"tactical_display", "rear_display":
			scene_path = "res://scenes/rear_display.tscn"
		"gm":
			scene_path = "res://scenes/gm_interface.tscn"
		_:
			scene_path = "res://scenes/station.tscn"
	
	Config.set_client_role(_selected_role)
	get_tree().change_scene_to_file(scene_path)


func _set_graphics_quality(quality: int) -> void:
	graphics_low.button_pressed = quality == 0
	graphics_med.button_pressed = quality == 1
	graphics_high.button_pressed = quality == 2
	Config.set_graphics_quality(quality)


func _on_fullscreen_toggled(enabled: bool) -> void:
	Config.set_fullscreen(enabled)


func _on_vsync_toggled(enabled: bool) -> void:
	Config.set_vsync(enabled)


func _on_master_changed(value: float) -> void:
	Config.set_master_volume(value / 100.0)


func _on_sfx_changed(value: float) -> void:
	Config.set_sfx_volume(value / 100.0)


func _on_music_changed(value: float) -> void:
	Config.set_music_volume(value / 100.0)


func _update_debug_info() -> void:
	fps_label.text = "FPS: %d" % Engine.get_frames_per_second()
	latency_label.text = "Latency: %.0fms" % NetworkClient.latency_ms
	packets_label.text = "Packets: %d / %d" % [NetworkClient.packets_received, NetworkClient.packets_sent]
	state_label.text = "State: " + NetworkClient.get_connection_status()


func _create_star_background() -> void:
	var stars_container := $Background/Stars
	var viewport_size := get_viewport_rect().size
	
	for i in 150:
		var star := ColorRect.new()
		var size := randf_range(1, 3)
		star.custom_minimum_size = Vector2(size, size)
		star.size = Vector2(size, size)
		star.position = Vector2(
			randf_range(0, viewport_size.x),
			randf_range(0, viewport_size.y)
		)
		var brightness := randf_range(0.3, 1.0)
		star.color = Color(brightness, brightness * 0.95, brightness * 1.05, 1.0)
		stars_container.add_child(star)
		
		# Add subtle twinkling animation
		var tween := create_tween().set_loops()
		tween.tween_property(star, "modulate:a", randf_range(0.4, 0.8), randf_range(1.5, 4.0))
		tween.tween_property(star, "modulate:a", 1.0, randf_range(1.5, 4.0))
