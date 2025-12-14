extends Node
## Configuration manager that handles loading and saving settings.

const CONFIG_PATH := "user://celestial_config.cfg"
const DEFAULT_SERVER := "ws://localhost:8080"

var _config := ConfigFile.new()

# Cached values
var server_address: String = "localhost"
var server_port: int = 8080
var client_role: String = ""
var graphics_quality: int = 2  # 0=Low, 1=Medium, 2=High
var master_volume: float = 1.0
var sfx_volume: float = 1.0
var music_volume: float = 1.0
var ui_volume: float = 1.0
var fullscreen: bool = true
var vsync: bool = true


func _ready() -> void:
	load_config()


func load_config() -> void:
	var err := _config.load(CONFIG_PATH)
	if err != OK:
		# Create default config
		save_config()
		return
	
	# Network
	server_address = _config.get_value("network", "server_address", "localhost")
	server_port = _config.get_value("network", "server_port", 8080)
	client_role = _config.get_value("network", "client_role", "")
	
	# Graphics
	graphics_quality = _config.get_value("graphics", "quality", 2)
	fullscreen = _config.get_value("graphics", "fullscreen", true)
	vsync = _config.get_value("graphics", "vsync", true)
	
	# Audio
	master_volume = _config.get_value("audio", "master", 1.0)
	sfx_volume = _config.get_value("audio", "sfx", 1.0)
	music_volume = _config.get_value("audio", "music", 1.0)
	ui_volume = _config.get_value("audio", "ui", 1.0)
	
	_apply_settings()


func save_config() -> void:
	# Network
	_config.set_value("network", "server_address", server_address)
	_config.set_value("network", "server_port", server_port)
	_config.set_value("network", "client_role", client_role)
	
	# Graphics
	_config.set_value("graphics", "quality", graphics_quality)
	_config.set_value("graphics", "fullscreen", fullscreen)
	_config.set_value("graphics", "vsync", vsync)
	
	# Audio
	_config.set_value("audio", "master", master_volume)
	_config.set_value("audio", "sfx", sfx_volume)
	_config.set_value("audio", "music", music_volume)
	_config.set_value("audio", "ui", ui_volume)
	
	_config.save(CONFIG_PATH)


func get_server_url() -> String:
	return "ws://" + server_address + ":" + str(server_port)


func set_server(address: String, port: int) -> void:
	server_address = address
	server_port = port
	save_config()


func set_client_role(role: String) -> void:
	client_role = role
	save_config()


func set_graphics_quality(quality: int) -> void:
	graphics_quality = clampi(quality, 0, 2)
	_apply_graphics_settings()
	save_config()


func set_fullscreen(enabled: bool) -> void:
	fullscreen = enabled
	_apply_display_settings()
	save_config()


func set_vsync(enabled: bool) -> void:
	vsync = enabled
	_apply_display_settings()
	save_config()


func set_master_volume(value: float) -> void:
	master_volume = clampf(value, 0.0, 1.0)
	AudioManager.set_master_volume(master_volume)
	save_config()


func set_sfx_volume(value: float) -> void:
	sfx_volume = clampf(value, 0.0, 1.0)
	AudioManager.set_sfx_volume(sfx_volume)
	save_config()


func set_music_volume(value: float) -> void:
	music_volume = clampf(value, 0.0, 1.0)
	AudioManager.set_music_volume(music_volume)
	save_config()


func _apply_settings() -> void:
	_apply_graphics_settings()
	_apply_display_settings()
	_apply_audio_settings()


func _apply_graphics_settings() -> void:
	var env := get_viewport().world_3d.environment if get_viewport().world_3d else null
	
	match graphics_quality:
		0:  # Low
			RenderingServer.viewport_set_msaa_3d(get_viewport().get_viewport_rid(), RenderingServer.VIEWPORT_MSAA_DISABLED)
			if env:
				env.glow_enabled = false
				env.ssao_enabled = false
				env.ssr_enabled = false
		1:  # Medium
			RenderingServer.viewport_set_msaa_3d(get_viewport().get_viewport_rid(), RenderingServer.VIEWPORT_MSAA_2X)
			if env:
				env.glow_enabled = true
				env.ssao_enabled = false
				env.ssr_enabled = false
		2:  # High
			RenderingServer.viewport_set_msaa_3d(get_viewport().get_viewport_rid(), RenderingServer.VIEWPORT_MSAA_4X)
			if env:
				env.glow_enabled = true
				env.ssao_enabled = true
				env.ssr_enabled = true


func _apply_display_settings() -> void:
	if fullscreen:
		DisplayServer.window_set_mode(DisplayServer.WINDOW_MODE_FULLSCREEN)
	else:
		DisplayServer.window_set_mode(DisplayServer.WINDOW_MODE_WINDOWED)
	
	DisplayServer.window_set_vsync_mode(
		DisplayServer.VSYNC_ENABLED if vsync else DisplayServer.VSYNC_DISABLED
	)


func _apply_audio_settings() -> void:
	AudioManager.set_master_volume(master_volume)
	AudioManager.set_sfx_volume(sfx_volume)
	AudioManager.set_music_volume(music_volume)
