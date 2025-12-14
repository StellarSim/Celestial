extends Node3D
## Main viewscreen renderer displaying the 3D space view from the player ship.
## Handles ship rendering, projectiles, effects, and camera management.

const SHIP_SCENE := preload("res://scenes/3d/ship_instance.tscn")
const TORPEDO_SCENE := preload("res://scenes/3d/torpedo.tscn")
const EXPLOSION_SCENE := preload("res://scenes/3d/explosion.tscn")

@onready var camera: Camera3D = $PlayerCamera
@onready var camera_shake: Node3D = $PlayerCamera/CameraShake
@onready var ships_container: Node3D = $Ships
@onready var projectiles_container: Node3D = $Projectiles
@onready var effects_container: Node3D = $Effects
@onready var starfield: Node3D = $Starfield

# HUD elements
@onready var ship_name_label: Label = $UI/HUD/TopHUD/LeftInfo/ShipName
@onready var ship_class_label: Label = $UI/HUD/TopHUD/LeftInfo/ShipClass
@onready var alert_status: Label = $UI/HUD/TopHUD/AlertStatus
@onready var heading_label: Label = $UI/HUD/TopHUD/RightInfo/Heading
@onready var speed_label: Label = $UI/HUD/TopHUD/RightInfo/Speed
@onready var fore_shield: ProgressBar = $UI/HUD/BottomHUD/ShieldsDisplay/ShieldGrid/ForeFacing
@onready var aft_shield: ProgressBar = $UI/HUD/BottomHUD/ShieldsDisplay/ShieldGrid/AftFacing
@onready var port_shield: ProgressBar = $UI/HUD/BottomHUD/ShieldsDisplay/ShieldGrid/PortFacing
@onready var starboard_shield: ProgressBar = $UI/HUD/BottomHUD/ShieldsDisplay/ShieldGrid/StarboardFacing
@onready var hull_value: Label = $UI/HUD/BottomHUD/ShieldsDisplay/ShieldGrid/HullDisplay/HullValue
@onready var shield_status: Label = $UI/HUD/BottomHUD/ShieldsDisplay/ShieldStatus
@onready var debug_overlay: PanelContainer = $UI/HUD/DebugOverlay
@onready var disconnect_overlay: ColorRect = $UI/HUD/DisconnectOverlay
@onready var alert_overlay: ColorRect = $UI/HUD/AlertOverlay
@onready var targeting_reticle: Control = $UI/HUD/TargetingReticle
@onready var damage_vignette: ColorRect = $UI/HUD/DamageOverlay/VignetteEffect

# Ship visual instances keyed by ship_id
var _ship_instances: Dictionary = {}
# Projectile visual instances keyed by projectile_id  
var _projectile_instances: Dictionary = {}

# Camera shake
var _shake_trauma: float = 0.0
var _shake_decay: float = 2.0
var _shake_max_offset := Vector3(0.3, 0.2, 0.1)
var _shake_max_rotation := Vector3(0.02, 0.02, 0.01)

# Alert flash
var _alert_tween: Tween = null


func _ready() -> void:
	_connect_signals()
	_create_starfield()
	_preload_effects()


func _process(delta: float) -> void:
	_update_camera(delta)
	_update_ship_visuals(delta)
	_update_projectile_visuals(delta)
	_update_hud()
	_process_shake(delta)
	
	if debug_overlay.visible:
		_update_debug_info()


func _input(event: InputEvent) -> void:
	if event.is_action_pressed("debug_toggle"):
		debug_overlay.visible = not debug_overlay.visible


func _connect_signals() -> void:
	NetworkClient.connected.connect(_on_connected)
	NetworkClient.disconnected.connect(_on_disconnected)
	GameState.ship_added.connect(_on_ship_added)
	GameState.ship_removed.connect(_on_ship_removed)
	GameState.projectile_added.connect(_on_projectile_added)
	GameState.projectile_removed.connect(_on_projectile_removed)
	GameState.alert_level_changed.connect(_on_alert_changed)


func _create_starfield() -> void:
	# Create a simple procedural starfield using MultiMeshInstance3D
	var star_mesh := SphereMesh.new()
	star_mesh.radius = 1.0
	star_mesh.height = 2.0
	star_mesh.radial_segments = 4
	star_mesh.rings = 2
	
	var star_material := StandardMaterial3D.new()
	star_material.albedo_color = Color.WHITE
	star_material.emission_enabled = true
	star_material.emission = Color(1.0, 0.98, 0.95)
	star_material.emission_energy_multiplier = 2.0
	star_mesh.material = star_material
	
	var multi_mesh := MultiMesh.new()
	multi_mesh.transform_format = MultiMesh.TRANSFORM_3D
	multi_mesh.mesh = star_mesh
	multi_mesh.instance_count = 2000
	
	for i in multi_mesh.instance_count:
		var distance := randf_range(5000.0, 30000.0)
		var theta := randf() * TAU
		var phi := acos(2.0 * randf() - 1.0)
		
		var pos := Vector3(
			distance * sin(phi) * cos(theta),
			distance * sin(phi) * sin(theta),
			distance * cos(phi)
		)
		
		var scale := randf_range(3.0, 12.0)
		var transform := Transform3D().scaled(Vector3(scale, scale, scale))
		transform.origin = pos
		multi_mesh.set_instance_transform(i, transform)
	
	var multi_mesh_instance := MultiMeshInstance3D.new()
	multi_mesh_instance.multimesh = multi_mesh
	starfield.add_child(multi_mesh_instance)


func _preload_effects() -> void:
	# Preload effect scenes to avoid hitches during gameplay
	pass


func _update_camera(delta: float) -> void:
	var player_ship := GameState.get_player_ship()
	if player_ship == null:
		return
	
	var t := GameState.get_interpolation_factor()
	var target_pos := player_ship.get_interpolated_position(t)
	var target_rot := player_ship.get_interpolated_rotation(t)
	
	# Position camera behind and above the ship
	var offset := target_rot * Vector3(0, 5, 25)
	var camera_target := target_pos + offset
	
	camera.global_position = camera.global_position.lerp(camera_target, delta * 5.0)
	
	# Look at the ship
	var look_target := target_pos + target_rot * Vector3(0, 0, -50)
	camera.look_at(look_target, Vector3.UP)


func _update_ship_visuals(delta: float) -> void:
	var t := GameState.get_interpolation_factor()
	
	for ship_id in _ship_instances:
		var instance: Node3D = _ship_instances[ship_id]
		var ship := GameState.get_ship(ship_id)
		
		if ship == null:
			continue
		
		# Interpolate position and rotation
		instance.global_position = ship.get_interpolated_position(t)
		instance.quaternion = ship.get_interpolated_rotation(t)
		
		# Update visual state (damage, engine glow, etc.)
		if instance.has_method("update_visual_state"):
			instance.update_visual_state(ship)


func _update_projectile_visuals(delta: float) -> void:
	var t := GameState.get_interpolation_factor()
	
	for proj_id in _projectile_instances:
		var instance: Node3D = _projectile_instances[proj_id]
		var proj: GameState.ProjectileState = GameState.projectiles.get(proj_id)
		
		if proj == null:
			continue
		
		instance.global_position = proj.get_interpolated_position(t)
		
		# Orient projectile in direction of travel
		var velocity: Vector3 = proj.velocity.to_vector3()
		if velocity.length_squared() > 0.1:
			instance.look_at(instance.global_position + velocity.normalized(), Vector3.UP)


func _update_hud() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	# Ship info
	ship_name_label.text = ship.name.to_upper() if not ship.name.is_empty() else "USS UNKNOWN"
	ship_class_label.text = ship.ship_class.replace("_", " ").capitalize()
	
	# Velocity and heading
	var velocity: Vector3 = ship.velocity.to_vector3()
	var speed := velocity.length()
	speed_label.text = "SPD: %.0f m/s" % speed
	
	var rotation := ship.rotation.to_quaternion()
	var forward := rotation * Vector3.FORWARD
	var heading := rad_to_deg(atan2(forward.x, -forward.z))
	if heading < 0:
		heading += 360.0
	heading_label.text = "HDG: %03d°" % int(heading)
	
	# Shields
	var max_per_facing := ship.max_shields / 4.0 if ship.max_shields > 0 else 250.0
	var fore_val: float = ship.shield_facings.get("fore", 0.0)
	var aft_val: float = ship.shield_facings.get("aft", 0.0)
	var port_val: float = ship.shield_facings.get("port", 0.0)
	var starboard_val: float = ship.shield_facings.get("starboard", 0.0)
	fore_shield.value = (fore_val / max_per_facing) * 100.0
	aft_shield.value = (aft_val / max_per_facing) * 100.0
	port_shield.value = (port_val / max_per_facing) * 100.0
	starboard_shield.value = (starboard_val / max_per_facing) * 100.0
	
	# Shield status
	if ship.shields_enabled:
		var shield_percent := ship.shields / ship.max_shields if ship.max_shields > 0 else 0
		shield_status.text = "ONLINE"
		shield_status.add_theme_color_override("font_color", Colors.get_shield_color(shield_percent))
	else:
		shield_status.text = "OFFLINE"
		shield_status.add_theme_color_override("font_color", Colors.STATUS_OFFLINE)
	
	# Hull
	var hull_percent := (ship.hull_integrity / ship.max_hull) * 100.0 if ship.max_hull > 0 else 0
	hull_value.text = "%d%%" % int(hull_percent)
	hull_value.add_theme_color_override("font_color", Colors.get_health_color(hull_percent / 100.0))
	
	# Damage vignette based on hull
	if hull_percent < 50:
		damage_vignette.visible = true
		damage_vignette.color.a = (50.0 - hull_percent) / 100.0
	else:
		damage_vignette.visible = false


func _process_shake(delta: float) -> void:
	_shake_trauma = maxf(0.0, _shake_trauma - _shake_decay * delta)
	
	if _shake_trauma > 0:
		var shake := _shake_trauma * _shake_trauma  # Quadratic falloff
		
		camera_shake.position = Vector3(
			randf_range(-1, 1) * _shake_max_offset.x * shake,
			randf_range(-1, 1) * _shake_max_offset.y * shake,
			randf_range(-1, 1) * _shake_max_offset.z * shake
		)
		
		camera_shake.rotation = Vector3(
			randf_range(-1, 1) * _shake_max_rotation.x * shake,
			randf_range(-1, 1) * _shake_max_rotation.y * shake,
			randf_range(-1, 1) * _shake_max_rotation.z * shake
		)
	else:
		camera_shake.position = Vector3.ZERO
		camera_shake.rotation = Vector3.ZERO


func add_shake(amount: float) -> void:
	_shake_trauma = minf(1.0, _shake_trauma + amount)


func _on_connected() -> void:
	disconnect_overlay.visible = false


func _on_disconnected() -> void:
	disconnect_overlay.visible = true


func _on_ship_added(ship_id: String) -> void:
	if _ship_instances.has(ship_id):
		return
	
	var ship := GameState.get_ship(ship_id)
	if ship == null:
		return
	
	# Don't render player ship (we're looking from it)
	if ship.is_player:
		return
	
	var instance: Node3D
	if ResourceLoader.exists("res://scenes/3d/ship_instance.tscn"):
		instance = SHIP_SCENE.instantiate()
	else:
		# Fallback to simple mesh
		instance = _create_placeholder_ship(ship)
	
	ships_container.add_child(instance)
	instance.global_position = ship.position.to_vector3()
	_ship_instances[ship_id] = instance


func _on_ship_removed(ship_id: String) -> void:
	if not _ship_instances.has(ship_id):
		return
	
	var instance: Node3D = _ship_instances[ship_id]
	instance.queue_free()
	_ship_instances.erase(ship_id)


func _on_projectile_added(projectile_id: String) -> void:
	if _projectile_instances.has(projectile_id):
		return
	
	var proj = GameState.projectiles.get(projectile_id)
	if proj == null:
		return
	
	var instance: Node3D
	if ResourceLoader.exists("res://scenes/3d/torpedo.tscn"):
		instance = TORPEDO_SCENE.instantiate()
	else:
		instance = _create_placeholder_projectile(proj)
	
	projectiles_container.add_child(instance)
	instance.global_position = proj.position.to_vector3()
	_projectile_instances[projectile_id] = instance


func _on_projectile_removed(projectile_id: String) -> void:
	if not _projectile_instances.has(projectile_id):
		return
	
	var instance: Node3D = _projectile_instances[projectile_id]
	
	# Spawn explosion at projectile location
	_spawn_explosion(instance.global_position)
	
	instance.queue_free()
	_projectile_instances.erase(projectile_id)


func _on_alert_changed(level: String) -> void:
	if _alert_tween:
		_alert_tween.kill()
	
	match level:
		"red":
			alert_status.text = "RED ALERT"
			alert_status.add_theme_color_override("font_color", Colors.ALERT_RED)
			_start_alert_flash(Colors.ALERT_RED)
		"yellow":
			alert_status.text = "YELLOW ALERT"
			alert_status.add_theme_color_override("font_color", Colors.ALERT_YELLOW)
			_start_alert_flash(Colors.ALERT_YELLOW)
		_:
			alert_status.text = "CONDITION GREEN"
			alert_status.add_theme_color_override("font_color", Colors.ALERT_GREEN)
			alert_overlay.visible = false


func _start_alert_flash(color: Color) -> void:
	alert_overlay.visible = true
	alert_overlay.color = Color(color.r, color.g, color.b, 0.0)
	
	_alert_tween = create_tween().set_loops()
	_alert_tween.tween_property(alert_overlay, "color:a", 0.12, 0.4)
	_alert_tween.tween_property(alert_overlay, "color:a", 0.0, 0.4)


func _create_placeholder_ship(ship) -> Node3D:
	var node := Node3D.new()
	
	# Simple box mesh for now
	var mesh_instance := MeshInstance3D.new()
	var box := BoxMesh.new()
	
	# Scale based on ship class
	match ship.ship_class:
		"dreadnought":
			box.size = Vector3(30, 10, 80)
		"cruiser", "heavy_cruiser":
			box.size = Vector3(20, 8, 50)
		"frigate":
			box.size = Vector3(12, 5, 30)
		_:
			box.size = Vector3(15, 6, 40)
	
	var material := StandardMaterial3D.new()
	material.albedo_color = Color(0.4, 0.45, 0.5)
	material.metallic = 0.7
	material.roughness = 0.4
	box.material = material
	
	mesh_instance.mesh = box
	node.add_child(mesh_instance)
	
	# Add engine glow
	var engine_light := OmniLight3D.new()
	engine_light.light_color = Colors.ENGINE_GLOW
	engine_light.light_energy = 2.0
	engine_light.omni_range = 15.0
	engine_light.position = Vector3(0, 0, box.size.z / 2 + 2)
	node.add_child(engine_light)
	
	return node


func _create_placeholder_projectile(proj) -> Node3D:
	var node := Node3D.new()
	
	var mesh_instance := MeshInstance3D.new()
	var sphere := SphereMesh.new()
	sphere.radius = 1.5
	sphere.height = 3.0
	
	var material := StandardMaterial3D.new()
	material.albedo_color = Colors.TORPEDO
	material.emission_enabled = true
	material.emission = Colors.TORPEDO
	material.emission_energy_multiplier = 3.0
	sphere.material = material
	
	mesh_instance.mesh = sphere
	node.add_child(mesh_instance)
	
	# Add light
	var light := OmniLight3D.new()
	light.light_color = Colors.TORPEDO
	light.light_energy = 2.0
	light.omni_range = 10.0
	node.add_child(light)
	
	return node


func _spawn_explosion(position: Vector3) -> void:
	if ResourceLoader.exists("res://scenes/3d/explosion.tscn"):
		var explosion := EXPLOSION_SCENE.instantiate()
		effects_container.add_child(explosion)
		explosion.global_position = position
	else:
		# Simple flash effect
		var light := OmniLight3D.new()
		light.light_color = Colors.EXPLOSION_CORE
		light.light_energy = 10.0
		light.omni_range = 50.0
		light.global_position = position
		effects_container.add_child(light)
		
		# Fade out and remove
		var tween := create_tween()
		tween.tween_property(light, "light_energy", 0.0, 0.5)
		tween.tween_callback(light.queue_free)
	
	# Add screen shake for nearby explosions
	var player_ship := GameState.get_player_ship()
	if player_ship:
		var distance := position.distance_to(player_ship.position.to_vector3())
		if distance < 500:
			add_shake(clampf(1.0 - distance / 500.0, 0.1, 0.6))


func _update_debug_info() -> void:
	$UI/HUD/DebugOverlay/DebugContent/FPSLabel.text = "FPS: %d" % Engine.get_frames_per_second()
	$UI/HUD/DebugOverlay/DebugContent/ShipsLabel.text = "Ships: %d" % _ship_instances.size()
	$UI/HUD/DebugOverlay/DebugContent/ProjectilesLabel.text = "Projectiles: %d" % _projectile_instances.size()
	$UI/HUD/DebugOverlay/DebugContent/CameraLabel.text = "Cam: %.0f, %.0f, %.0f" % [camera.global_position.x, camera.global_position.y, camera.global_position.z]
