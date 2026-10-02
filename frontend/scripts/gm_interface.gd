extends Control

const SPAWN_AT := ["Camera", "Origin", "Selected"]

@onready var viewport: SubViewport = %SubViewport
@onready var gm_camera: Camera3D = %GMCamera
@onready var ships_container: Node3D = %Ships
@onready var objects_container: Node3D = %Objects

@onready var time_value: Label = %TimeValue
@onready var paused_label: Label = %PausedLabel
@onready var pause_btn: Button = %PauseBtn
@onready var snapshot_btn: Button = %SnapshotBtn
@onready var mission_btn: Button = %MissionBtn
@onready var mission_info: Label = %MissionInfo
@onready var connection_dot: ColorRect = %ConnectionDot

@onready var filter_edit: LineEdit = %FilterEdit
@onready var ships_tree: Tree = %ShipsList
@onready var jump_btn: Button = %JumpBtn
@onready var destroy_btn: Button = %DestroyBtn
@onready var spawn_kind: OptionButton = %SpawnKind
@onready var spawn_class: OptionButton = %SpawnClass
@onready var spawn_name: LineEdit = %SpawnName
@onready var spawn_faction: OptionButton = %SpawnFaction
@onready var spawn_at: OptionButton = %SpawnAt
@onready var spawn_btn: Button = %SpawnBtn
@onready var objects_list: ItemList = %ObjectsList
@onready var remove_obj_btn: Button = %RemoveObjBtn

@onready var sel_name: Label = %SelName
@onready var faction_val: Label = %FactionVal
@onready var class_val: Label = %ClassVal
@onready var target_val: Label = %TargetVal
@onready var alert_val: Label = %AlertVal
@onready var pos_val: Label = %PosVal
@onready var hull_bar: ProgressBar = %HullBar
@onready var hull_val: Label = %HullVal
@onready var shield_bar: ProgressBar = %ShieldBar
@onready var shield_val: Label = %ShieldVal
@onready var pos_x: SpinBox = %PosX
@onready var pos_y: SpinBox = %PosY
@onready var pos_z: SpinBox = %PosZ
@onready var name_edit: LineEdit = %NameEdit
@onready var faction_select: OptionButton = %FactionSelect
@onready var dmg_amount: SpinBox = %DmgAmount
@onready var inspector_text: TextEdit = %InspectorText
@onready var event_name: LineEdit = %EventName

@onready var mission_popup: PanelContainer = %MissionPopup
@onready var mission_select: OptionButton = %MissionSelect
@onready var snapshot_popup: PanelContainer = %SnapshotPopup
@onready var snapshot_list: ItemList = %SnapshotList
@onready var restore_btn: Button = %RestoreBtn
@onready var disconnect_overlay: ColorRect = %DisconnectOverlay

var camera_speed: float = 100.0
var camera_rotation_speed: float = 0.003
var camera_velocity: Vector3 = Vector3.ZERO
var mouse_captured: bool = false

var _ship_instances: Dictionary = {}
var _object_instances: Dictionary = {}
var _selected_ship_id: String = ""
var _inspector_timer: float = 0.0
const INSPECTOR_INTERVAL := 2.0


func _ready() -> void:
	_setup_ui()
	_connect_signals()
	_create_grid()
	_setup_environment()
	_sync_ship_visuals()
	_sync_object_visuals()


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
		var mb := event as InputEventMouseButton
		if mb.button_index == MOUSE_BUTTON_RIGHT:
			if mb.pressed:
				Input.mouse_mode = Input.MOUSE_MODE_CAPTURED
				mouse_captured = true
			else:
				Input.mouse_mode = Input.MOUSE_MODE_VISIBLE
				mouse_captured = false
		elif mb.button_index == MOUSE_BUTTON_WHEEL_UP and mb.pressed:
			camera_speed = minf(camera_speed * 1.2, 1000.0)
		elif mb.button_index == MOUSE_BUTTON_WHEEL_DOWN and mb.pressed:
			camera_speed = maxf(camera_speed / 1.2, 10.0)
		elif mb.button_index == MOUSE_BUTTON_LEFT and mb.pressed:
			_try_viewport_pick(mb.position)
	elif event is InputEventMouseMotion and mouse_captured:
		var mm := event as InputEventMouseMotion
		gm_camera.rotate_y(-mm.relative.x * camera_rotation_speed)
		gm_camera.rotate_object_local(Vector3.RIGHT, -mm.relative.y * camera_rotation_speed)
	if NavUtils.is_menu_exit(event, get_viewport()):
		Input.mouse_mode = Input.MOUSE_MODE_VISIBLE
		NavUtils.exit_to_menu(get_tree())


func _setup_ui() -> void:
	for kind in ["Ship", "Object"]:
		spawn_kind.add_item(kind)
	spawn_kind.select(0)
	_refresh_spawn_classes()
	_refresh_factions()
	if spawn_faction.item_count > 1:
		spawn_faction.select(1)
	if faction_select.item_count > 0:
		faction_select.select(0)
	for loc in SPAWN_AT:
		spawn_at.add_item(loc)
	spawn_at.select(0)
	ships_tree.create_item()
	ships_tree.set_column_title(0, "Ships")


func _available_ship_classes() -> Array:
	var out: Array = []
	for entry in GameState.ship_classes:
		if entry is Dictionary and entry.has("id"):
			out.append(str(entry["id"]))
	return out


func _available_object_types() -> Array:
	var out: Array = []
	for entry in GameState.object_classes:
		if entry is Dictionary and entry.has("id"):
			out.append(str(entry["id"]))
	return out


func _available_factions() -> Array:
	var out: Array = []
	for entry in GameState.factions:
		if entry is Dictionary and entry.has("id"):
			out.append(str(entry["id"]))
		elif entry is String:
			out.append(entry)
	return out


func _refresh_factions() -> void:
	spawn_faction.clear()
	faction_select.clear()
	for f in _available_factions():
		spawn_faction.add_item(f)
		faction_select.add_item(f)


func _refresh_spawn_classes() -> void:
	spawn_class.clear()
	if spawn_kind.selected == 1:
		for t in _available_object_types():
			spawn_class.add_item(t)
	else:
		for c in _available_ship_classes():
			spawn_class.add_item(c.capitalize())
	if spawn_class.item_count > 0:
		spawn_class.select(0)


func _connect_signals() -> void:
	NetworkClient.connected.connect(_on_connected)
	NetworkClient.disconnected.connect(_on_disconnected)
	GameState.state_updated.connect(_on_state_updated)
	GameState.ship_added.connect(_on_ship_added)
	GameState.ship_removed.connect(_on_ship_removed)
	GameState.paused_changed.connect(_on_paused_changed)
	pause_btn.pressed.connect(_on_pause_toggle)
	snapshot_btn.pressed.connect(func(): snapshot_popup.visible = not snapshot_popup.visible)
	mission_btn.pressed.connect(func(): mission_popup.visible = not mission_popup.visible)
	filter_edit.text_changed.connect(func(_t: String): _update_ships_tree())
	ships_tree.item_selected.connect(_on_ship_selected)
	ships_tree.item_activated.connect(_on_ship_activated)
	jump_btn.pressed.connect(_on_jump_pressed)
	destroy_btn.pressed.connect(_on_destroy_pressed)
	spawn_kind.item_selected.connect(func(_i: int): _refresh_spawn_classes())
	spawn_btn.pressed.connect(_on_spawn_pressed)
	objects_list.item_selected.connect(func(_i: int): remove_obj_btn.disabled = false)
	remove_obj_btn.pressed.connect(_on_remove_object)
	%TeleportBtn.pressed.connect(_on_teleport_pressed)
	%HereBtn.pressed.connect(_on_here_pressed)
	%JumpToBtn.pressed.connect(_on_jump_pressed)
	%RenameBtn.pressed.connect(_on_rename_pressed)
	%FactionBtn.pressed.connect(_on_faction_pressed)
	%DamageBtn.pressed.connect(_on_damage_pressed)
	%HealBtn.pressed.connect(_on_heal_pressed)
	%FullBtn.pressed.connect(_on_full_pressed)
	%ShieldDrainBtn.pressed.connect(_on_shield_drain)
	%ShieldFullBtn.pressed.connect(_on_shield_full)
	%GreenBtn.pressed.connect(func(): _set_alert("normal"))
	%YellowBtn.pressed.connect(func(): _set_alert("yellow"))
	%RedBtn.pressed.connect(func(): _set_alert("red"))
	%WinBtn.pressed.connect(_on_win_pressed)
	%LoseBtn.pressed.connect(_on_lose_pressed)
	%TriggerBtn.pressed.connect(_on_trigger_pressed)
	%StartBtn.pressed.connect(_on_mission_start_pressed)
	%StopBtn.pressed.connect(_on_restart_pressed)
	%MissionCloseBtn.pressed.connect(func(): mission_popup.visible = false)
	%CreateSnapBtn.pressed.connect(func(): NetworkClient.send_gm_command("create_snapshot"))
	snapshot_list.item_selected.connect(func(_i: int): restore_btn.disabled = false)
	restore_btn.pressed.connect(_on_restore_pressed)
	%SnapCloseBtn.pressed.connect(func(): snapshot_popup.visible = false)


func _create_grid() -> void:
	var grid_helper := %GridHelper
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
	%WorldEnvironment.environment = env
	var sun := DirectionalLight3D.new()
	sun.light_color = Color(1.0, 0.98, 0.95)
	sun.light_energy = 0.8
	sun.rotation_degrees = Vector3(-45, -30, 0)
	%World.add_child(sun)


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


func _update_ships_visual(_delta: float) -> void:
	var t := GameState.get_interpolation_factor()
	var ahead := GameState.get_extrapolation_ahead()
	for ship_id in _ship_instances:
		var instance: Node3D = _ship_instances[ship_id]
		var ship := GameState.get_ship(ship_id)
		if ship == null:
			continue
		instance.global_position = ship.get_interpolated_position(t, ahead)
		instance.quaternion = ship.get_interpolated_rotation(t)
	for obj_id in _object_instances:
		var node: Node3D = _object_instances[obj_id]
		if node == null:
			continue


func _ship_display_name(ship_id: String, ship) -> String:
	var display_name: String = ship.name if not ship.name.is_empty() else ship_id
	if ship.is_player:
		display_name += " [P]"
	return display_name


func _update_ships_tree() -> void:
	# Clear existing items
	var root := ships_tree.get_root()
	for child in root.get_children():
		child.free()
	var filter := filter_edit.text.strip_edges().to_lower()
	var ids: Array = GameState.ships.keys()
	ids.sort()
	for ship_id in ids:
		var ship = GameState.ships[ship_id]
		var display_name := _ship_display_name(str(ship_id), ship)
		if not filter.is_empty() and display_name.to_lower().find(filter) < 0 and str(ship_id).to_lower().find(filter) < 0:
			continue
		var item := ships_tree.create_item(root)
		item.set_text(0, display_name)
		item.set_metadata(0, ship_id)
		
		# Color based on faction
		item.set_custom_color(0, GameState.get_faction_color(ship.faction))
		if str(ship_id) == _selected_ship_id:
			item.select(0)


var _objects_sig := ""


func _update_objects_list() -> void:
	var parts: Array = []
	for obj in GameState.space_objects:
		if obj is Dictionary:
			parts.append("%s:%s" % [str(obj.get("id", "")), str(obj.get("type", ""))])
	parts.sort()
	var sig := "|".join(parts)
	if sig == _objects_sig:
		remove_obj_btn.disabled = objects_list.get_selected_items().is_empty()
		_sync_object_visuals()
		return
	_objects_sig = sig
	var sel := objects_list.get_selected_items()
	var sel_id := ""
	if not sel.is_empty():
		sel_id = objects_list.get_item_text(sel[0]).split(" ")[0]
	objects_list.clear()
	for obj in GameState.space_objects:
		if not (obj is Dictionary):
			continue
		var oid := str(obj.get("id", "?"))
		var otype := str(obj.get("type", "?"))
		var idx := objects_list.add_item("%s (%s)" % [oid, otype])
		if oid == sel_id:
			objects_list.select(idx)
	remove_obj_btn.disabled = objects_list.get_selected_items().is_empty()
	_sync_object_visuals()


func _on_connected() -> void:
	NavUtils.set_connection_state(disconnect_overlay, true, connection_dot)


func _on_disconnected() -> void:
	NavUtils.set_connection_state(disconnect_overlay, false, connection_dot)


func _on_state_updated() -> void:
	_update_ships_tree()
	_update_objects_list()
	_update_snapshot_list()
	_update_mission_picker()
	_update_mission_info()
	_update_selected_panel()
	_maybe_refresh_catalogs()
	_sync_ship_visuals()


var _catalog_sig := ""


func _maybe_refresh_catalogs() -> void:
	var sig := "%s|%s|%s" % [str(GameState.ship_classes), str(GameState.object_classes), str(GameState.factions)]
	if sig == _catalog_sig:
		return
	_catalog_sig = sig
	var cls_sel := spawn_class.selected
	_refresh_spawn_classes()
	if spawn_class.item_count > 0:
		spawn_class.select(clampi(cls_sel, 0, spawn_class.item_count - 1))
	var fac_sel := spawn_faction.selected
	var insp_sel := faction_select.selected
	_refresh_factions()
	if fac_sel >= 0 and spawn_faction.item_count > 0:
		spawn_faction.select(clampi(fac_sel, 0, spawn_faction.item_count - 1))
	if insp_sel >= 0 and faction_select.item_count > 0:
		faction_select.select(clampi(insp_sel, 0, faction_select.item_count - 1))


func _sync_ship_visuals() -> void:
	for ship_id in GameState.ships:
		if not _ship_instances.has(ship_id):
			_on_ship_added(str(ship_id))
	for ship_id in _ship_instances.keys():
		if not GameState.ships.has(ship_id):
			_on_ship_removed(str(ship_id))


func _sync_object_visuals() -> void:
	var seen: Dictionary = {}
	for obj in GameState.space_objects:
		if not (obj is Dictionary):
			continue
		var oid := str(obj.get("id", ""))
		if oid.is_empty():
			continue
		seen[oid] = true
		if _object_instances.has(oid):
			var node: Node3D = _object_instances[oid]
			var p: Dictionary = obj.get("position", {})
			node.global_position = Vector3(float(p.get("x", 0)), float(p.get("y", 0)), float(p.get("z", 0)))
			continue
		var marker := _create_object_visual(str(obj.get("type", "")), float(obj.get("radius", 20.0)))
		objects_container.add_child(marker)
		# Positioned only after being added, since a global transform on a node
		# that is not in the tree yet has nothing to resolve against.
		var pp: Dictionary = obj.get("position", {})
		marker.global_position = Vector3(float(pp.get("x", 0)), float(pp.get("y", 0)), float(pp.get("z", 0)))
		_object_instances[oid] = marker
	for oid in _object_instances.keys():
		if not seen.has(oid):
			_object_instances[oid].queue_free()
			_object_instances.erase(oid)


func _create_object_visual(obj_type: String, radius: float = 20.0) -> Node3D:
	var node := Node3D.new()
	var mesh_instance := MeshInstance3D.new()
	var sphere := SphereMesh.new()
	sphere.radius = clampf(radius, 5.0, 500.0)
	sphere.height = sphere.radius * 2.0
	var mat := StandardMaterial3D.new()
	mat.albedo_color = Color(0.5, 0.7, 0.4) if obj_type == "waypoint" else Color(0.7, 0.6, 0.3)
	mat.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	sphere.material = mat
	mesh_instance.mesh = sphere
	node.add_child(mesh_instance)
	return node


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
	var select_idx := -1
	for i in range(mission_select.item_count):
		if mission_select.get_item_text(i) == GameState.active_mission:
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
			mission_info.text = "No mission"
		else:
			mission_info.text = "Mission: %s" % GameState.active_mission
		return
	var done := 0
	var total := 0
	for obj in GameState.get_mission_objectives():
		total += 1
		if obj.get("complete", false):
			done += 1
	mission_info.text = "%s (%d/%d)" % [GameState.get_mission_name(), done, total]


func _update_selected_panel() -> void:
	var ship := GameState.get_ship(_selected_ship_id)
	if ship == null:
		sel_name.text = "none"
		for l in [faction_val, class_val, target_val, alert_val, pos_val, hull_val, shield_val]:
			l.text = "-"
		hull_bar.value = 0
		shield_bar.value = 0
		return
	sel_name.text = ship.name if not ship.name.is_empty() else _selected_ship_id
	faction_val.text = ship.faction
	class_val.text = ship.ship_class
	target_val.text = ship.target_id if not ship.target_id.is_empty() else "-"
	alert_val.text = ship.alert_level
	var p := ship.position.to_vector3()
	pos_val.text = "(%d, %d, %d)" % [int(p.x), int(p.y), int(p.z)]
	var hull_pct := 0.0
	if ship.max_hull > 0:
		hull_pct = ship.hull_integrity / ship.max_hull * 100.0
	hull_bar.value = hull_pct
	hull_val.text = "%d/%d" % [int(ship.hull_integrity), int(ship.max_hull)]
	var shield_pct := 0.0
	if ship.max_shields > 0:
		shield_pct = ship.shields / ship.max_shields * 100.0
	shield_bar.value = shield_pct
	shield_val.text = "%d/%d" % [int(ship.shields), int(ship.max_shields)]
	if not pos_x.has_focus():
		pos_x.value = p.x
	if not pos_y.has_focus():
		pos_y.value = p.y
	if not pos_z.has_focus():
		pos_z.value = p.z


func _refresh_inspector() -> void:
	var ships := {}
	for ship_id in GameState.ships:
		var ship = GameState.ships[ship_id]
		var pos: Dictionary = ship.position.to_dict()
		ships[ship_id] = {
			"name": ship.name,
			"faction": ship.faction,
			"class": ship.ship_class,
			"hull": [int(ship.hull_integrity), int(ship.max_hull)],
			"shields": [int(ship.shields), int(ship.max_shields)],
			"position": [int(pos.x), int(pos.y), int(pos.z)],
			"target": ship.target_id,
			"alert": ship.alert_level,
		}
	var state := {
		"time": snappedf(GameState.simulation_time, 0.1),
		"paused": GameState.is_paused,
		"ships": ships.size(),
		"objects": GameState.space_objects.size(),
		"projectiles": GameState.projectiles.size(),
	}
	inspector_text.text = JSON.stringify(state, " ")
	_update_selected_panel()


func _on_paused_changed(is_paused: bool) -> void:
	pause_btn.text = "RESUME" if is_paused else "PAUSE"


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
	if _selected_ship_id == ship_id:
		_selected_ship_id = ""


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
	material.albedo_color = GameState.get_faction_color(ship.faction)
	material.metallic = 0.5
	material.roughness = 0.5
	box.material = material
	mesh_instance.mesh = box
	node.add_child(mesh_instance)
	var label := Label3D.new()
	label.text = ship.name if not ship.name.is_empty() else ship.id
	label.position = Vector3(0, box.size.y + 5, 0)
	label.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	label.font_size = 32
	label.modulate = Color.WHITE
	node.add_child(label)
	return node


func _try_viewport_pick(mouse_pos: Vector2) -> void:
	var container := %ViewportContainer as SubViewportContainer
	var rect := container.get_global_rect()
	if not rect.has_point(mouse_pos):
		return
	var best_id := ""
	var best_dist := 24.0
	for ship_id in GameState.ships:
		var ship = GameState.ships[ship_id]
		var world_pos: Vector3 = ship.position.to_vector3()
		if gm_camera.is_position_behind(world_pos):
			continue
		var screen := gm_camera.unproject_position(world_pos)
		var local: Vector2 = screen + container.global_position - mouse_pos
		var d := local.length()
		if d < best_dist:
			best_dist = d
			best_id = str(ship_id)
	if not best_id.is_empty():
		_selected_ship_id = best_id
		_update_ships_tree()
		_update_selected_panel()


func _require_selection() -> bool:
	return not _selected_ship_id.is_empty() and GameState.ships.has(_selected_ship_id)


func _on_ship_selected() -> void:
	var selected := ships_tree.get_selected()
	if selected:
		_selected_ship_id = str(selected.get_metadata(0))
		_update_selected_panel()


func _on_ship_activated() -> void:
	_on_jump_pressed()


func _jump_camera_to(pos: Vector3) -> void:
	var dir := -gm_camera.global_transform.basis.z.normalized()
	if dir.length_squared() < 0.01:
		dir = Vector3(0, 0.4, 1).normalized()
	gm_camera.global_position = pos + dir * -300.0 + Vector3(0, 150, 0)
	gm_camera.look_at(pos, Vector3.UP)


func _on_jump_pressed() -> void:
	if not _require_selection():
		return
	var ship := GameState.get_ship(_selected_ship_id)
	if ship == null:
		return
	_jump_camera_to(ship.position.to_vector3())


func _on_destroy_pressed() -> void:
	if not _require_selection():
		return
	NetworkClient.send_gm_command("remove_ship", {"ship_id": _selected_ship_id})


func _spawn_position() -> Vector3:
	match spawn_at.selected:
		1:
			return Vector3.ZERO
		2:
			if _require_selection():
				var ship := GameState.get_ship(_selected_ship_id)
				return ship.position.to_vector3() + Vector3(200, 50, 200)
	return gm_camera.global_position + gm_camera.global_transform.basis * Vector3(0, 0, -400)


func _on_spawn_pressed() -> void:
	var pos := _spawn_position()
	if spawn_kind.selected == 1:
		var types := _available_object_types()
		if spawn_class.selected < 0 or spawn_class.selected >= types.size():
			return
		var obj_type: String = types[spawn_class.selected]
		var obj_id := spawn_name.text.strip_edges()
		if obj_id.is_empty():
			obj_id = "%s_%d" % [obj_type, Time.get_ticks_msec() % 100000]
		NetworkClient.send_gm_command("spawn_object", {
			"object_id": obj_id,
			"object_type": obj_type,
			"position": {"x": pos.x, "y": pos.y, "z": pos.z},
		})
	else:
		var classes := _available_ship_classes()
		if spawn_class.selected < 0 or spawn_class.selected >= classes.size():
			return
		var ship_class: String = classes[spawn_class.selected]
		var base := spawn_name.text.strip_edges()
		if base.is_empty():
			base = ship_class
		var ship_id := "%s_%d" % [base.to_lower().replace(" ", "_"), Time.get_ticks_msec() % 100000]
		NetworkClient.send_gm_command("spawn_ship", {
			"class_id": ship_class,
			"ship_id": ship_id,
			"name": base,
			"is_player": false,
			"position": {"x": pos.x, "y": pos.y, "z": pos.z},
		})
		var factions := _available_factions()
		if spawn_faction.selected >= 0 and spawn_faction.selected < factions.size():
			NetworkClient.send_gm_command("set_faction", {"ship_id": ship_id, "faction": factions[spawn_faction.selected]})
	spawn_name.clear()


func _on_remove_object() -> void:
	var sel := objects_list.get_selected_items()
	if sel.is_empty():
		return
	var oid := objects_list.get_item_text(sel[0]).split(" ")[0]
	NetworkClient.send_gm_command("remove_object", {"object_id": oid})


func _on_teleport_pressed() -> void:
	if not _require_selection():
		return
	NetworkClient.send_gm_command("teleport_ship", {
		"ship_id": _selected_ship_id,
		"position": {"x": pos_x.value, "y": pos_y.value, "z": pos_z.value},
	})


func _on_here_pressed() -> void:
	if not _require_selection():
		return
	var pos := gm_camera.global_position + gm_camera.global_transform.basis * Vector3(0, 0, -300)
	NetworkClient.send_gm_command("teleport_ship", {
		"ship_id": _selected_ship_id,
		"position": {"x": pos.x, "y": pos.y, "z": pos.z},
	})


func _on_rename_pressed() -> void:
	if not _require_selection():
		return
	var new_name := name_edit.text.strip_edges()
	if new_name.is_empty():
		return
	NetworkClient.send_gm_command("rename_ship", {"ship_id": _selected_ship_id, "name": new_name})
	name_edit.clear()


func _on_faction_pressed() -> void:
	if not _require_selection():
		return
	var factions := _available_factions()
	if faction_select.selected < 0 or faction_select.selected >= factions.size():
		return
	NetworkClient.send_gm_command("set_faction", {
		"ship_id": _selected_ship_id,
		"faction": factions[faction_select.selected],
	})


func _on_damage_pressed() -> void:
	if not _require_selection():
		return
	NetworkClient.send_gm_command("modify_ship", {
		"ship_id": _selected_ship_id,
		"system": "hull",
		"value": -float(dmg_amount.value),
	})


func _on_heal_pressed() -> void:
	if not _require_selection():
		return
	NetworkClient.send_gm_command("modify_ship", {
		"ship_id": _selected_ship_id,
		"system": "hull",
		"value": float(dmg_amount.value),
	})


func _on_full_pressed() -> void:
	if not _require_selection():
		return
	NetworkClient.send_gm_command("modify_ship", {"ship_id": _selected_ship_id, "system": "hull_restore", "value": 1})


func _on_shield_drain() -> void:
	if not _require_selection():
		return
	NetworkClient.send_gm_command("modify_ship", {"ship_id": _selected_ship_id, "system": "shields_drain", "value": 999999})


func _on_shield_full() -> void:
	if not _require_selection():
		return
	NetworkClient.send_gm_command("modify_ship", {"ship_id": _selected_ship_id, "system": "shields_full", "value": 1})


func _on_pause_toggle() -> void:
	NetworkClient.send_gm_command("resume" if GameState.is_paused else "pause")


func _set_alert(level: String) -> void:
	NetworkClient.send_gm_command("set_alert", {"level": level})


func _on_win_pressed() -> void:
	NetworkClient.send_gm_command("mission_win")


func _on_lose_pressed() -> void:
	NetworkClient.send_gm_command("mission_lose")


func _on_mission_start_pressed() -> void:
	if mission_select.item_count == 0 or mission_select.selected < 0:
		return
	NetworkClient.send_gm_command("start_mission", {"mission": mission_select.get_item_text(mission_select.selected)})
	mission_popup.visible = false


func _on_trigger_pressed() -> void:
	var event := event_name.text.strip_edges()
	if event.is_empty():
		return
	NetworkClient.send_gm_command("trigger_mission_event", {"event": event, "data": {}})
	event_name.clear()


func _on_restart_pressed() -> void:
	NetworkClient.send_gm_command("stop_mission")
	mission_popup.visible = false


func _on_restore_pressed() -> void:
	var selected := snapshot_list.get_selected_items()
	if selected.is_empty():
		return
	NetworkClient.send_gm_command("restore_snapshot", {"snapshot_index": snapshot_list.get_item_metadata(selected[0])})
	snapshot_popup.visible = false
