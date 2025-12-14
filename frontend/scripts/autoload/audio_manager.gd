extends Node
## Audio manager for handling all game sounds.
## Provides pooled audio players and bus management.

const BUS_MASTER := "Master"
const BUS_SFX := "SFX"
const BUS_MUSIC := "Music"
const BUS_UI := "UI"
const BUS_AMBIENT := "Ambient"

const POOL_SIZE := 16

var _sfx_pool: Array[AudioStreamPlayer] = []
var _sfx_3d_pool: Array[AudioStreamPlayer3D] = []
var _music_player: AudioStreamPlayer
var _ambient_player: AudioStreamPlayer
var _current_sfx_index: int = 0
var _current_sfx_3d_index: int = 0

# Preloaded common sounds (will be populated as assets are created)
var sounds: Dictionary = {}


func _ready() -> void:
	_setup_audio_buses()
	_create_audio_pools()


func _setup_audio_buses() -> void:
	# Create audio buses if they don't exist
	var bus_layout := AudioServer.get_bus_count()
	
	# The buses should be defined in project settings, but we ensure they exist
	_ensure_bus_exists(BUS_SFX, BUS_MASTER)
	_ensure_bus_exists(BUS_MUSIC, BUS_MASTER)
	_ensure_bus_exists(BUS_UI, BUS_MASTER)
	_ensure_bus_exists(BUS_AMBIENT, BUS_MASTER)


func _ensure_bus_exists(bus_name: String, send_to: String) -> void:
	var idx := AudioServer.get_bus_index(bus_name)
	if idx == -1:
		var new_idx := AudioServer.bus_count
		AudioServer.add_bus()
		AudioServer.set_bus_name(new_idx, bus_name)
		AudioServer.set_bus_send(new_idx, send_to)


func _create_audio_pools() -> void:
	# 2D SFX pool
	for i in POOL_SIZE:
		var player := AudioStreamPlayer.new()
		player.bus = BUS_SFX
		add_child(player)
		_sfx_pool.append(player)
	
	# 3D SFX pool
	for i in POOL_SIZE:
		var player := AudioStreamPlayer3D.new()
		player.bus = BUS_SFX
		add_child(player)
		_sfx_3d_pool.append(player)
	
	# Music player
	_music_player = AudioStreamPlayer.new()
	_music_player.bus = BUS_MUSIC
	add_child(_music_player)
	
	# Ambient player
	_ambient_player = AudioStreamPlayer.new()
	_ambient_player.bus = BUS_AMBIENT
	add_child(_ambient_player)


func play_sfx(sound: AudioStream, volume_db: float = 0.0, pitch: float = 1.0) -> void:
	if sound == null:
		return
	
	var player := _get_next_sfx_player()
	player.stream = sound
	player.volume_db = volume_db
	player.pitch_scale = pitch
	player.play()


func play_sfx_3d(sound: AudioStream, position: Vector3, volume_db: float = 0.0, pitch: float = 1.0) -> void:
	if sound == null:
		return
	
	var player := _get_next_sfx_3d_player()
	player.stream = sound
	player.global_position = position
	player.volume_db = volume_db
	player.pitch_scale = pitch
	player.play()


func play_ui_sound(sound: AudioStream, volume_db: float = 0.0) -> void:
	if sound == null:
		return
	
	var player := _get_next_sfx_player()
	player.bus = BUS_UI
	player.stream = sound
	player.volume_db = volume_db
	player.pitch_scale = 1.0
	player.play()
	
	# Reset bus after playing
	player.finished.connect(func(): player.bus = BUS_SFX, CONNECT_ONE_SHOT)


func play_music(music: AudioStream, fade_time: float = 1.0) -> void:
	if music == null:
		stop_music(fade_time)
		return
	
	if _music_player.playing:
		var tween := create_tween()
		tween.tween_property(_music_player, "volume_db", -40.0, fade_time)
		await tween.finished
	
	_music_player.stream = music
	_music_player.volume_db = -40.0
	_music_player.play()
	
	var tween := create_tween()
	tween.tween_property(_music_player, "volume_db", 0.0, fade_time)


func stop_music(fade_time: float = 1.0) -> void:
	if not _music_player.playing:
		return
	
	var tween := create_tween()
	tween.tween_property(_music_player, "volume_db", -40.0, fade_time)
	await tween.finished
	_music_player.stop()


func play_ambient(ambient: AudioStream, fade_time: float = 2.0) -> void:
	if ambient == null:
		stop_ambient(fade_time)
		return
	
	_ambient_player.stream = ambient
	_ambient_player.volume_db = -40.0
	_ambient_player.play()
	
	var tween := create_tween()
	tween.tween_property(_ambient_player, "volume_db", -10.0, fade_time)


func stop_ambient(fade_time: float = 2.0) -> void:
	if not _ambient_player.playing:
		return
	
	var tween := create_tween()
	tween.tween_property(_ambient_player, "volume_db", -40.0, fade_time)
	await tween.finished
	_ambient_player.stop()


func set_master_volume(value: float) -> void:
	var bus_idx := AudioServer.get_bus_index(BUS_MASTER)
	AudioServer.set_bus_volume_db(bus_idx, linear_to_db(value))


func set_sfx_volume(value: float) -> void:
	var bus_idx := AudioServer.get_bus_index(BUS_SFX)
	AudioServer.set_bus_volume_db(bus_idx, linear_to_db(value))


func set_music_volume(value: float) -> void:
	var bus_idx := AudioServer.get_bus_index(BUS_MUSIC)
	AudioServer.set_bus_volume_db(bus_idx, linear_to_db(value))


func set_ui_volume(value: float) -> void:
	var bus_idx := AudioServer.get_bus_index(BUS_UI)
	AudioServer.set_bus_volume_db(bus_idx, linear_to_db(value))


func _get_next_sfx_player() -> AudioStreamPlayer:
	var player := _sfx_pool[_current_sfx_index]
	_current_sfx_index = (_current_sfx_index + 1) % POOL_SIZE
	return player


func _get_next_sfx_3d_player() -> AudioStreamPlayer3D:
	var player := _sfx_3d_pool[_current_sfx_3d_index]
	_current_sfx_3d_index = (_current_sfx_3d_index + 1) % POOL_SIZE
	return player


# Convenience methods for common sounds
func play_button_click() -> void:
	# Will use actual sound when assets are added
	pass


func play_button_hover() -> void:
	pass


func play_alert(level: String) -> void:
	pass


func play_damage_hit() -> void:
	pass


func play_weapon_fire(weapon_type: String, position: Vector3) -> void:
	pass


func play_explosion(position: Vector3, size: float = 1.0) -> void:
	pass


func play_shield_impact(position: Vector3) -> void:
	pass
