extends Control
## Game Master interface with free camera, ship management, and simulation controls.

const SHIP_CLASSES := ["player_cruiser", "enemy_frigate", "enemy_dreadnought"]

@onready var viewport: SubViewport = $MainSplit/ViewportContainer/SubViewport
@onready var gm_camera: Camera3D = $MainSplit/ViewportContainer/SubViewport/World/GMCamera
@onready var ships_container: Node3D = $MainSplit/ViewportContainer/SubViewport/World/Ships

@onready var time_value: Label = $MainSplit/ControlPanel/PanelScroll/PanelContent/SimulationSection/TimeDisplay/TimeValue
@onready var paused_label: Label = $MainSplit/ControlPanel/PanelScroll/PanelContent/SimulationSection/TimeDisplay/PausedLabel
@onready var pause_btn: Button = $MainSplit/ControlPanel/PanelScroll/PanelContent/SimulationSection/ControlButtons/PauseBtn
@onready var resume_btn: Button = $MainSplit/ControlPanel/PanelScroll/PanelContent/SimulationSection/ControlButtons/ResumeBtn
@onready var snapshot_list: ItemList = $MainSplit/ControlPanel/PanelScroll/PanelContent/SnapshotSection/SnapshotList
@onready var restore_btn: Button = $MainSplit/ControlPanel/PanelScroll/PanelContent/SnapshotSection/RestoreBtn
@onready var ships_tree: Tree = $MainSplit/ControlPanel/PanelScroll/PanelContent/ShipsSection/ShipsList
@onready var ship_class_select: OptionButton = $MainSplit/ControlPanel/PanelScroll/PanelContent/SpawnSection/SpawnRow/ShipClassSelect
@onready var connection_dot: ColorRect = $MainSplit/ControlPanel/PanelScroll/PanelContent/Header/ConnectionDot
@onready var disconnect_overlay: ColorRect = $DisconnectOverlay
@onready var mission_info: Label = $MainSplit/ControlPanel/PanelScroll/PanelContent/MissionSection/MissionInfo
@onready var mission_select: OptionButton = $MainSplit/ControlPanel/PanelScroll/PanelContent/MissionSection/MissionPicker/MissionSelect
@onready var start_btn: Button = $MainSplit/ControlPanel/PanelScroll/PanelContent/MissionSection/MissionPicker/StartBtn
@onready var event_name: LineEdit = $MainSplit/ControlPanel/PanelScroll/PanelContent/MissionSection/EventRow/EventName
@onready var trigger_btn: Button = $MainSplit/ControlPanel/PanelScroll/PanelContent/MissionSection/EventRow/TriggerBtn
@onready var inspector_text: TextEdit = $MainSplit/ControlPanel/PanelScroll/PanelContent/InspectorSection/InspectorText

# Camera control
var camera_speed: float = 100.0
var camera_rotation_speed: float = 0.003
var camera_velocity: Vector3 = Vector3.ZERO
var mouse_captured: bool = false

# Ship visualization
var _ship_instances: Dictionary = {}
var _selected_ship_id: String = ""

# State inspector auto-refresh pacing (seconds).
var _inspector_timer: float = 0.0
const INSPECTOR_INTERVAL := 2.0


func _ready() -> void:
	_setup_ui()
	_connect_signals()
	_create_grid()
	_setup_environment()
	_sync_ship_visuals()


func _process(delta: float) -> void:
	_update_camera(delta)
	_update_time_display()
	_update_ships_visual(delta)
	_inspector_timer += delta
	if _inspector_timer >= INSPECTOR_INTERVAL:
		_inspector_timer = 0.0
		_refresh_inspector()


func _input(event: InputEvent) -> void:
	if event is InputEventMouseButton:
		if event.button_index == MOUSE_BUTTON_RIGHT:
			if event.pressed:
				Input.mouse_mode = Input.MOUSE_MODE_CAPTURED
				mouse_captured = true
			else:
				Input.mouse_mode = Input.MOUSE_MODE_VISIBLE
				mouse_captured = false
		elif event.button_index == MOUSE_BUTTON_WHEEL_UP:
			camera_speed = minf(camera_speed * 1.2, 1000.0)
		elif event.button_index == MOUSE_BUTTON_WHEEL_DOWN:
			camera_speed = maxf(camera_speed / 1.2, 10.0)
	
	elif event is InputEventMouseMotion and mouse_captured:
		gm_camera.rotate_y(-event.relative.x * camera_rotation_speed)
		gm_camera.rotate_object_local(Vector3.RIGHT, -event.relative.y * camera_rotation_speed)

	if NavUtils.is_menu_exit(event, get_viewport()):
		Input.mouse_mode = Input.MOUSE_MODE_VISIBLE
		NavUtils.exit_to_menu(get_tree())


func _setup_ui() -> void:
	# Populate ship class dropdown
	for ship_class in SHIP_CLASSES:
		ship_class_select.add_item(ship_class.capitalize())
	ship_class_select.select(0)
	
	# Initialize ships tree
	ships_tree.create_item()  # Root
	ships_tree.set_column_title(0, "Ships")


func _connect_signals() -> void:
	NetworkClient.connected.connect(_on_connected)
	NetworkClient.disconnected.connect(_on_disconnected)
	GameState.state_updated.connect(_on_state_updated)
	GameState.ship_added.connect(_on_ship_added)
	GameState.ship_removed.connect(_on_ship_removed)
	GameState.paused_changed.connect(_on_paused_changed)
	
	# Simulation controls
	pause_btn.pressed.connect(_on_pause_pressed)
	resume_btn.pressed.connect(_on_resume_pressed)
	
	# Snapshot controls
	snapshot_list.item_selected.connect(_on_snapshot_selected)
	restore_btn.pressed.connect(_on_restore_pressed)
	
	# Ship controls
	ships_tree.item_selected.connect(_on_ship_selected)
	$MainSplit/ControlPanel/PanelScroll/PanelContent/ShipsSection/ShipActions/DamageBtn.pressed.connect(_on_damage_pressed)
	$MainSplit/ControlPanel/PanelScroll/PanelContent/ShipsSection/ShipActions/HealBtn.pressed.connect(_on_heal_pressed)
	$MainSplit/ControlPanel/PanelScroll/PanelContent/ShipsSection/ShipActions/DestroyBtn.pressed.connect(_on_destroy_pressed)
	
	# Spawn controls
	$MainSplit/ControlPanel/PanelScroll/PanelContent/SpawnSection/SpawnRow/SpawnBtn.pressed.connect(_on_spawn_pressed)
	
	# Alert controls
	$MainSplit/ControlPanel/PanelScroll/PanelContent/AlertSection/AlertButtons/GreenBtn.pressed.connect(func(): _set_alert("normal"))
	$MainSplit/ControlPanel/PanelScroll/PanelContent/AlertSection/AlertButtons/YellowBtn.pressed.connect(func(): _set_alert("yellow"))
	$MainSplit/ControlPanel/PanelScroll/PanelContent/AlertSection/AlertButtons/RedBtn.pressed.connect(func(): _set_alert("red"))
	
	# Mission controls
	$MainSplit/ControlPanel/PanelScroll/PanelContent/MissionSection/MissionButtons/WinBtn.pressed.connect(_on_win_pressed)
	$MainSplit/ControlPanel/PanelScroll/PanelContent/MissionSection/MissionButtons/LoseBtn.pressed.connect(_on_lose_pressed)
	$MainSplit/ControlPanel/PanelScroll/PanelContent/MissionSection/MissionButtons/RestartBtn.pressed.connect(_on_restart_pressed)
	start_btn.pressed.connect(_on_mission_start_pressed)
	trigger_btn.pressed.connect(_on_trigger_pressed)
	
	# State inspector
	$MainSplit/ControlPanel/PanelScroll/PanelContent/InspectorSection/RefreshBtn.pressed.connect(_refresh_inspector)


func _create_grid() -> void:
	var grid_helper := $MainSplit/ViewportContainer/SubViewport/World/GridHelper
	
	# Create a simple grid using lines
	var grid_material := StandardMaterial3D.new()
	grid_material.albedo_color = Color(0.2, 0.3, 0.4, 0.5)
	grid_material.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	grid_material.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	
	var grid_size := 5000.0
	var grid_spacing := 100.0
	
	var immediate_mesh := ImmediateMesh.new()
	immediate_mesh.surface_begin(Mesh.PRIMITIVE_LINES, grid_material)
	
	# Draw grid lines
	var half_size := grid_size / 2.0
	var num_lines := int(grid_size / grid_spacing)
	
	for i in range(-num_lines / 2, num_lines / 2 + 1):
		var pos := i * grid_spacing
		
		# X-axis lines
		immediate_mesh.surface_add_vertex(Vector3(-half_size, 0, pos))
		immediate_mesh.surface_add_vertex(Vector3(half_size, 0, pos))
		
		# Z-axis lines
		immediate_mesh.surface_add_vertex(Vector3(pos, 0, -half_size))
		immediate_mesh.surface_add_vertex(Vector3(pos, 0, half_size))
	
	immediate_mesh.surface_end()
	
	var mesh_instance := MeshInstance3D.new()
	mesh_instance.mesh = immediate_mesh
	grid_helper.add_child(mesh_instance)


func _setup_environment() -> void:
	var env := Environment.new()
	env.background_mode = Environment.BG_COLOR
	env.background_color = Color(0.01, 0.02, 0.04)
	env.ambient_light_source = Environment.AMBIENT_SOURCE_COLOR
	env.ambient_light_color = Color(0.15, 0.18, 0.25)
	env.ambient_light_energy = 0.5
	
	var world_env := $MainSplit/ViewportContainer/SubViewport/World/WorldEnvironment
	world_env.environment = env
	
	# Add directional light
	var sun := DirectionalLight3D.new()
	sun.light_color = Color(1.0, 0.98, 0.95)
	sun.light_energy = 0.8
	sun.rotation_degrees = Vector3(-45, -30, 0)
	$MainSplit/ViewportContainer/SubViewport/World.add_child(sun)


func _update_camera(delta: float) -> void:
	var input_dir := Vector3.ZERO
	
	if Input.is_key_pressed(KEY_W):
		input_dir.z -= 1
	if Input.is_key_pressed(KEY_S):
		input_dir.z += 1
	if Input.is_key_pressed(KEY_A):
		input_dir.x -= 1
	if Input.is_key_pressed(KEY_D):
		input_dir.x += 1
	if Input.is_key_pressed(KEY_Q):
		input_dir.y -= 1
	if Input.is_key_pressed(KEY_E):
		input_dir.y += 1
	
	if input_dir.length_squared() > 0:
		input_dir = input_dir.normalized()
		var move_dir := gm_camera.global_transform.basis * input_dir
		camera_velocity = camera_velocity.lerp(move_dir * camera_speed, delta * 10.0)
	else:
		camera_velocity = camera_velocity.lerp(Vector3.ZERO, delta * 5.0)
	
	gm_camera.global_position += camera_velocity * delta


func _update_time_display() -> void:
	time_value.text = NavUtils.format_clock(GameState.simulation_time)

	paused_label.visible = GameState.is_paused


func _update_ships_visual(delta: float) -> void:
	var t := GameState.get_interpolation_factor()
	
	for ship_id in _ship_instances:
		var instance: Node3D = _ship_instances[ship_id]
		var ship := GameState.get_ship(ship_id)
		
		if ship == null:
			continue
		
		instance.global_position = ship.get_interpolated_position(t)
		instance.quaternion = ship.get_interpolated_rotation(t)


func _update_ships_tree() -> void:
	# Clear existing items
	var root := ships_tree.get_root()
	for child in root.get_children():
		child.free()
	
	# Add ships
	for ship_id in GameState.ships:
		var ship: GameState.ShipState = GameState.ships[ship_id]
		var item := ships_tree.create_item(root)
		
		var display_name: String = ship.name if not ship.name.is_empty() else ship_id
		if ship.is_player:
			display_name += " [PLAYER]"
		
		item.set_text(0, display_name)
		item.set_metadata(0, ship_id)
		
		# Color based on faction
		item.set_custom_color(0, Colors.get_faction_color(ship.faction))


func _on_connected() -> void:
	NavUtils.set_connection_state(disconnect_overlay, true, connection_dot)


func _on_disconnected() -> void:
	NavUtils.set_connection_state(disconnect_overlay, false, connection_dot)


func _on_state_updated() -> void:
	_update_ships_tree()
	_update_snapshot_list()
	_update_mission_picker()
	_update_mission_info()
	_sync_ship_visuals()


func _sync_ship_visuals() -> void:
	# Rebuild visuals for ships that already exist (re-enter after menu exit)
	# and drop visuals for ships that are gone. Signal-only creation misses
	# pre-existing ships because ship_added is not re-emitted for them.
	for ship_id in GameState.ships:
		if not _ship_instances.has(ship_id):
			_on_ship_added(str(ship_id))
	for ship_id in _ship_instances.keys():
		if not GameState.ships.has(ship_id):
			_on_ship_removed(str(ship_id))


func _update_snapshot_list() -> void:
	var selected_index := -1
	var selected := snapshot_list.get_selected_items()
	if not selected.is_empty():
		selected_index = snapshot_list.get_item_metadata(selected[0])

	snapshot_list.clear()
	for snap in GameState.snapshots:
		if not (snap is Dictionary):
			continue
		var idx := int(snap.get("index", snapshot_list.item_count))
		var t := float(snap.get("time", 0.0))
		var total_seconds := int(t)
		var text := "Snapshot %d - %02d:%02d:%02d" % [idx, total_seconds / 3600, (total_seconds % 3600) / 60, total_seconds % 60]
		snapshot_list.add_item(text)
		snapshot_list.set_item_metadata(snapshot_list.item_count - 1, idx)
		if idx == selected_index:
			snapshot_list.select(snapshot_list.item_count - 1)

	restore_btn.disabled = snapshot_list.get_selected_items().is_empty()


func _update_mission_picker() -> void:
	var current := ""
	if mission_select.item_count > 0 and mission_select.selected >= 0:
		current = mission_select.get_item_text(mission_select.selected)

	mission_select.clear()
	for mission_id in GameState.missions:
		mission_select.add_item(str(mission_id))

	if mission_select.item_count == 0:
		return

	var active := GameState.active_mission
	var select_idx := -1
	for i in range(mission_select.item_count):
		if mission_select.get_item_text(i) == active:
			select_idx = i
			break
	if select_idx < 0:
		for i in range(mission_select.item_count):
			if mission_select.get_item_text(i) == current:
				select_idx = i
				break
	if select_idx < 0:
		select_idx = 0
	mission_select.select(select_idx)


func _update_mission_info() -> void:
	if GameState.mission.is_empty():
		if GameState.active_mission.is_empty():
			mission_info.text = "No mission loaded"
		else:
			mission_info.text = "Mission: %s" % GameState.active_mission
		return
	var done := 0
	var total := 0
	for obj in GameState.get_mission_objectives():
		total += 1
		if obj.get("complete", false):
			done += 1
	mission_info.text = "%s (%d/%d objectives)" % [GameState.get_mission_name(), done, total]


func _refresh_inspector() -> void:
	var ships := {}
	for ship_id in GameState.ships:
		var ship = GameState.ships[ship_id]
		var pos: Dictionary = ship.position.to_dict()
		ships[ship_id] = {
			"name": ship.name,
			"faction": ship.faction,
			"class": ship.ship_class,
			"hull": [ship.hull_integrity, ship.max_hull],
			"shields": [ship.shields, ship.max_shields],
			"position": [pos.x, pos.y, pos.z],
			"target": ship.target_id,
			"alert": ship.alert_level,
		}
	var state := {
		"time": GameState.simulation_time,
		"paused": GameState.is_paused,
		"alert_level": GameState.alert_level,
		"active_mission": GameState.active_mission,
		"missions": GameState.missions,
		"snapshot_count": GameState.snapshot_count,
		"ships": ships,
		"projectiles": GameState.projectiles.size(),
		"orders": GameState.orders,
		"waypoints": GameState.waypoints.size(),
	}
	inspector_text.text = JSON.stringify(state, "\t")


func _on_paused_changed(is_paused: bool) -> void:
	pause_btn.disabled = is_paused
	resume_btn.disabled = not is_paused


func _on_ship_added(ship_id: String) -> void:
	var ship := GameState.get_ship(ship_id)
	if ship == null:
		return
	
	var instance := _create_ship_visual(ship)
	ships_container.add_child(instance)
	instance.global_position = ship.position.to_vector3()
	_ship_instances[ship_id] = instance


func _on_ship_removed(ship_id: String) -> void:
	if _ship_instances.has(ship_id):
		_ship_instances[ship_id].queue_free()
		_ship_instances.erase(ship_id)


func _create_ship_visual(ship) -> Node3D:
	var node := Node3D.new()
	
	var mesh_instance := MeshInstance3D.new()
	var box := BoxMesh.new()
	
	var dims := Vector3(15, 6, 40)
	match ship.ship_class:
		"enemy_dreadnought":
			dims = Vector3(30, 10, 80)
		"player_cruiser":
			dims = Vector3(20, 8, 50)
		"enemy_frigate":
			dims = Vector3(12, 5, 30)
	box.size = dims
	
	var material := StandardMaterial3D.new()
	material.albedo_color = Colors.get_faction_color(ship.faction)
	material.metallic = 0.5
	material.roughness = 0.5
	box.material = material
	
	mesh_instance.mesh = box
	node.add_child(mesh_instance)
	
	# Add label
	var label := Label3D.new()
	label.text = ship.name if not ship.name.is_empty() else ship.id
	label.position = Vector3(0, box.size.y + 5, 0)
	label.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	label.font_size = 32
	label.modulate = Color.WHITE
	node.add_child(label)
	
	return node


func _on_pause_pressed() -> void:
	NetworkClient.send_gm_command("pause")


func _on_resume_pressed() -> void:
	NetworkClient.send_gm_command("resume")


func _on_snapshot_selected(index: int) -> void:
	restore_btn.disabled = false


func _on_restore_pressed() -> void:
	var selected := snapshot_list.get_selected_items()
	if selected.is_empty():
		return

	NetworkClient.send_gm_command("restore_snapshot", {"snapshot_index": snapshot_list.get_item_metadata(selected[0])})


func _on_ship_selected() -> void:
	var selected := ships_tree.get_selected()
	if selected:
		_selected_ship_id = selected.get_metadata(0)


func _on_damage_pressed() -> void:
	if _selected_ship_id.is_empty():
		return
	NetworkClient.send_gm_command("modify_ship", {
		"ship_id": _selected_ship_id,
		"system": "hull",
		"value": -100.0
	})


func _on_heal_pressed() -> void:
	if _selected_ship_id.is_empty():
		return
	NetworkClient.send_gm_command("modify_ship", {
		"ship_id": _selected_ship_id,
		"system": "hull",
		"value": 100.0
	})


func _on_destroy_pressed() -> void:
	if _selected_ship_id.is_empty():
		return
	NetworkClient.send_gm_command("remove_ship", {"ship_id": _selected_ship_id})


func _on_spawn_pressed() -> void:
	var ship_class: String = SHIP_CLASSES[ship_class_select.selected]
	var spawn_pos := gm_camera.global_position + gm_camera.global_transform.basis * Vector3(0, 0, -200)
	
	NetworkClient.send_gm_command("spawn_ship", {
		"class_id": ship_class,
		"ship_id": "%s_%d" % [ship_class, Time.get_ticks_msec()],
		"name": ship_class.capitalize(),
		"position": {"x": spawn_pos.x, "y": spawn_pos.y, "z": spawn_pos.z}
	})


func _set_alert(level: String) -> void:
	NetworkClient.send_gm_command("set_alert", {"level": level})


func _on_win_pressed() -> void:
	NetworkClient.send_gm_command("mission_win")


func _on_lose_pressed() -> void:
	NetworkClient.send_gm_command("mission_lose")


func _on_mission_start_pressed() -> void:
	if mission_select.item_count == 0 or mission_select.selected < 0:
		return
	NetworkClient.send_gm_command("start_mission", {
		"mission": mission_select.get_item_text(mission_select.selected)
	})


func _on_trigger_pressed() -> void:
	var event := event_name.text.strip_edges()
	if event.is_empty():
		return
	NetworkClient.send_gm_command("trigger_mission_event", {
		"event": event,
		"data": {}
	})
	event_name.clear()


func _on_restart_pressed() -> void:
	NetworkClient.send_gm_command("stop_mission")
