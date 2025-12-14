extends Control
## Relay station panel - sector map, waypoint management, and probes.

@onready var tactical_map: Control = $MainSplit/MapSection/MapContent/TacticalMap
@onready var zoom_out_btn: Button = $MainSplit/MapSection/MapContent/MapHeader/ZoomOut
@onready var zoom_level_label: Label = $MainSplit/MapSection/MapContent/MapHeader/ZoomLevel
@onready var zoom_in_btn: Button = $MainSplit/MapSection/MapContent/MapHeader/ZoomIn
@onready var center_btn: Button = $MainSplit/MapSection/MapContent/MapControls/CenterPlayer
@onready var show_waypoints_btn: CheckButton = $MainSplit/MapSection/MapContent/MapControls/ShowWaypoints
@onready var show_grid_btn: CheckButton = $MainSplit/MapSection/MapContent/MapControls/ShowGrid

@onready var waypoint_list: ItemList = $MainSplit/RightSection/WaypointSection/WaypointContent/WaypointList
@onready var add_waypoint_btn: Button = $MainSplit/RightSection/WaypointSection/WaypointContent/WaypointActions/AddWaypoint
@onready var remove_waypoint_btn: Button = $MainSplit/RightSection/WaypointSection/WaypointContent/WaypointActions/RemoveWaypoint
@onready var send_to_flight_btn: Button = $MainSplit/RightSection/WaypointSection/WaypointContent/WaypointActions/SendToFlight

@onready var selected_name: Label = $MainSplit/RightSection/SelectedInfo/SelectedContent/SelectedName
@onready var dist_value: Label = $MainSplit/RightSection/SelectedInfo/SelectedContent/SelectedDetails/DistValue
@onready var bearing_value: Label = $MainSplit/RightSection/SelectedInfo/SelectedContent/SelectedDetails/BearingValue
@onready var type_value: Label = $MainSplit/RightSection/SelectedInfo/SelectedContent/SelectedDetails/TypeValue
@onready var faction_value: Label = $MainSplit/RightSection/SelectedInfo/SelectedContent/SelectedDetails/FactionValue
@onready var mark_target_btn: Button = $MainSplit/RightSection/SelectedInfo/SelectedContent/SelectedActions/MarkTarget
@onready var set_waypoint_btn: Button = $MainSplit/RightSection/SelectedInfo/SelectedContent/SelectedActions/SetWaypoint

@onready var probe_count: Label = $MainSplit/RightSection/ProbeSection/ProbeContent/ProbeInventory/ProbeCount
@onready var launch_probe_btn: Button = $MainSplit/RightSection/ProbeSection/ProbeContent/LaunchProbe

var _zoom_level: float = 1.0
var _map_center: Vector2 = Vector2.ZERO
var _map_scale: float = 0.01  # World units to screen pixels
var _show_waypoints: bool = true
var _show_grid: bool = true
var _selected_contact_id: String = ""
var _dragging: bool = false
var _drag_start: Vector2 = Vector2.ZERO
var _waypoints: Array[Dictionary] = []
var _probes_available: int = 5


func _ready() -> void:
	_connect_signals()


func _process(_delta: float) -> void:
	_update_display()
	queue_redraw()


func _draw() -> void:
	_draw_tactical_map()


func _connect_signals() -> void:
	GameState.state_updated.connect(_on_state_updated)
	
	zoom_out_btn.pressed.connect(_on_zoom_out)
	zoom_in_btn.pressed.connect(_on_zoom_in)
	center_btn.pressed.connect(_on_center_player)
	show_waypoints_btn.toggled.connect(func(v): _show_waypoints = v)
	show_grid_btn.toggled.connect(func(v): _show_grid = v)
	
	add_waypoint_btn.pressed.connect(_on_add_waypoint)
	remove_waypoint_btn.pressed.connect(_on_remove_waypoint)
	send_to_flight_btn.pressed.connect(_on_send_to_flight)
	waypoint_list.item_selected.connect(_on_waypoint_selected)
	
	mark_target_btn.pressed.connect(_on_mark_target)
	set_waypoint_btn.pressed.connect(_on_set_waypoint_from_contact)
	launch_probe_btn.pressed.connect(_on_launch_probe)
	
	tactical_map.gui_input.connect(_on_map_input)


func _update_display() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	_update_zoom_label()
	_update_waypoint_list()
	_update_selected_info()
	_update_probe_count()


func _update_zoom_label() -> void:
	zoom_level_label.text = "%.0f%%" % (_zoom_level * 100)


func _update_waypoint_list() -> void:
	var player_ship := GameState.get_player_ship()
	if player_ship == null:
		return
	
	waypoint_list.clear()
	var player_pos := player_ship.position.to_vector3()
	
	var waypoints: Array = GameState.get_mission_waypoints()
	for wp in waypoints:
		var wp_pos := Vector3(wp.get("x", 0), wp.get("y", 0), wp.get("z", 0))
		var dist := player_pos.distance_to(wp_pos)
		waypoint_list.add_item("%s (%.1f km)" % [wp.get("name", "Unknown"), dist / 1000.0])
	
	for wp in _waypoints:
		var wp_pos := Vector3(wp.get("x", 0), wp.get("y", 0), wp.get("z", 0))
		var dist := player_pos.distance_to(wp_pos)
		waypoint_list.add_item("📍 %s (%.1f km)" % [wp.get("name", "Custom"), dist / 1000.0])


func _update_selected_info() -> void:
	if _selected_contact_id.is_empty():
		selected_name.text = "---"
		dist_value.text = "---"
		bearing_value.text = "---"
		type_value.text = "---"
		faction_value.text = "---"
		mark_target_btn.disabled = true
		set_waypoint_btn.disabled = true
		return
	
	var contact: GameState.ShipState = GameState.ships.get(_selected_contact_id)
	if contact == null:
		_selected_contact_id = ""
		return
	
	var player_ship := GameState.get_player_ship()
	if player_ship == null:
		return
	
	var player_pos := player_ship.position.to_vector3()
	var contact_pos := contact.position.to_vector3()
	var dist := player_pos.distance_to(contact_pos)
	
	var dir := (contact_pos - player_pos).normalized()
	var bearing := rad_to_deg(atan2(dir.x, dir.z))
	bearing = fmod(bearing + 360, 360)
	
	selected_name.text = contact.display_name
	selected_name.add_theme_color_override("font_color", Colors.get_faction_color(contact.faction))
	dist_value.text = "%.1f km" % (dist / 1000.0)
	bearing_value.text = "%03.0f°" % bearing
	type_value.text = contact.ship_class
	faction_value.text = contact.faction.capitalize()
	var faction_color: Color = Colors.get_faction_color(contact.faction)
	faction_value.add_theme_color_override("font_color", faction_color)
	
	mark_target_btn.disabled = false
	set_waypoint_btn.disabled = false


func _update_probe_count() -> void:
	probe_count.text = str(_probes_available)
	launch_probe_btn.disabled = _probes_available <= 0


func _draw_tactical_map() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	var map_rect := tactical_map.get_rect()
	var map_center := map_rect.size / 2
	
	# Draw grid
	if _show_grid:
		_draw_grid(map_rect, map_center)
	
	# Draw waypoints
	if _show_waypoints:
		_draw_waypoints(map_center)
	
	# Draw all contacts
	_draw_contacts(map_center)
	
	# Draw player ship
	_draw_player_ship(map_center)


func _draw_grid(rect: Rect2, center: Vector2) -> void:
	var grid_spacing := 100.0 * _zoom_level  # pixels
	var grid_color := Color(Colors.PRIMARY.r, Colors.PRIMARY.g, Colors.PRIMARY.b, 0.2)
	
	# Vertical lines
	var x := fmod(center.x, grid_spacing)
	while x < rect.size.x:
		tactical_map.draw_line(Vector2(x, 0), Vector2(x, rect.size.y), grid_color, 1.0)
		x += grid_spacing
	
	# Horizontal lines
	var y := fmod(center.y, grid_spacing)
	while y < rect.size.y:
		tactical_map.draw_line(Vector2(0, y), Vector2(rect.size.x, y), grid_color, 1.0)
		y += grid_spacing


func _draw_waypoints(center: Vector2) -> void:
	var player_ship := GameState.get_player_ship()
	if player_ship == null:
		return
	
	var player_pos := player_ship.position.to_vector3()
	
	# Draw mission waypoints
	for wp in GameState.get_mission_waypoints():
		var wp_pos := Vector3(wp.get("x", 0), wp.get("y", 0), wp.get("z", 0))
		var screen_pos := _world_to_screen(wp_pos, player_pos, center)
		
		tactical_map.draw_circle(screen_pos, 8, Color(Colors.ALERT_YELLOW, 0.3))
		tactical_map.draw_arc(screen_pos, 8, 0, TAU, 16, Colors.ALERT_YELLOW, 2.0)
	
	# Draw custom waypoints
	for wp in _waypoints:
		var wp_pos := Vector3(wp.get("x", 0), wp.get("y", 0), wp.get("z", 0))
		var screen_pos := _world_to_screen(wp_pos, player_pos, center)
		
		tactical_map.draw_circle(screen_pos, 6, Color(Colors.PRIMARY, 0.3))
		tactical_map.draw_arc(screen_pos, 6, 0, TAU, 16, Colors.PRIMARY, 2.0)


func _draw_contacts(center: Vector2) -> void:
	var player_ship := GameState.get_player_ship()
	if player_ship == null:
		return
	
	var player_pos := player_ship.position.to_vector3()
	
	for ship_id in GameState.ships:
		if ship_id == GameState.player_ship_id:
			continue
		
		var ship: GameState.ShipState = GameState.ships[ship_id]
		var ship_pos := ship.position.to_vector3()
		var screen_pos := _world_to_screen(ship_pos, player_pos, center)
		
		var color: Color = Colors.get_faction_color(ship.faction)
		var size := 6.0
		
		if ship_id == _selected_contact_id:
			size = 10.0
			tactical_map.draw_arc(screen_pos, 15, 0, TAU, 16, color, 2.0)
		
		# Draw as triangle pointing in direction of movement
		var ship_rotation: Vector3 = ship.rotation.to_vector3()
		var ship_heading: float = ship_rotation.y
		var points := PackedVector2Array([
			screen_pos + Vector2(0, -size).rotated(ship_heading),
			screen_pos + Vector2(-size * 0.6, size * 0.6).rotated(ship_heading),
			screen_pos + Vector2(size * 0.6, size * 0.6).rotated(ship_heading)
		])
		tactical_map.draw_colored_polygon(points, color)


func _draw_player_ship(center: Vector2) -> void:
	var player_ship := GameState.get_player_ship()
	if player_ship == null:
		return
	
	var player_rotation: Vector3 = player_ship.rotation.to_vector3()
	var heading: float = player_rotation.y
	
	# Draw player ship in center
	var size := 10.0
	var points := PackedVector2Array([
		center + Vector2(0, -size).rotated(heading),
		center + Vector2(-size * 0.6, size * 0.6).rotated(heading),
		center + Vector2(size * 0.6, size * 0.6).rotated(heading)
	])
	tactical_map.draw_colored_polygon(points, Colors.FACTION_PLAYER)
	tactical_map.draw_polyline(points + PackedVector2Array([points[0]]), Colors.PRIMARY, 2.0)
	
	# Draw range rings
	for r in [5000, 10000, 20000]:  # meters
		var screen_r: float = r * _map_scale * _zoom_level
		tactical_map.draw_arc(center, screen_r, 0, TAU, 32, Color(Colors.PRIMARY, 0.3), 1.0)


func _world_to_screen(world_pos: Vector3, player_pos: Vector3, screen_center: Vector2) -> Vector2:
	var offset := world_pos - player_pos
	var screen_offset := Vector2(offset.x, offset.z) * _map_scale * _zoom_level
	return screen_center + screen_offset + _map_center


func _screen_to_world(screen_pos: Vector2, player_pos: Vector3, screen_center: Vector2) -> Vector3:
	var screen_offset := (screen_pos - screen_center - _map_center)
	var world_offset := screen_offset / (_map_scale * _zoom_level)
	return player_pos + Vector3(world_offset.x, 0, world_offset.y)


func _on_zoom_out() -> void:
	_zoom_level = clampf(_zoom_level * 0.8, 0.1, 5.0)


func _on_zoom_in() -> void:
	_zoom_level = clampf(_zoom_level * 1.25, 0.1, 5.0)


func _on_center_player() -> void:
	_map_center = Vector2.ZERO


func _on_map_input(event: InputEvent) -> void:
	if event is InputEventMouseButton:
		var mb := event as InputEventMouseButton
		if mb.button_index == MOUSE_BUTTON_LEFT:
			if mb.pressed:
				_dragging = true
				_drag_start = mb.position
				_try_select_contact(mb.position)
			else:
				_dragging = false
		elif mb.button_index == MOUSE_BUTTON_WHEEL_UP:
			_on_zoom_in()
		elif mb.button_index == MOUSE_BUTTON_WHEEL_DOWN:
			_on_zoom_out()
	elif event is InputEventMouseMotion and _dragging:
		var mm := event as InputEventMouseMotion
		_map_center += mm.relative


func _try_select_contact(screen_pos: Vector2) -> void:
	var player_ship := GameState.get_player_ship()
	if player_ship == null:
		return
	
	var map_center := tactical_map.size / 2
	var player_pos := player_ship.position.to_vector3()
	
	var closest_id := ""
	var closest_dist := 20.0  # Minimum click distance
	
	for ship_id in GameState.ships:
		if ship_id == GameState.player_ship_id:
			continue
		
		var ship: GameState.ShipState = GameState.ships[ship_id]
		var ship_screen := _world_to_screen(ship.position.to_vector3(), player_pos, map_center)
		var dist := screen_pos.distance_to(ship_screen)
		
		if dist < closest_dist:
			closest_dist = dist
			closest_id = ship_id
	
	_selected_contact_id = closest_id


func _on_add_waypoint() -> void:
	# Add waypoint at map center
	var player_ship := GameState.get_player_ship()
	if player_ship == null:
		return
	
	var map_center := tactical_map.size / 2
	var world_pos := _screen_to_world(map_center + _map_center, player_ship.position.to_vector3(), map_center)
	
	var wp := {
		"name": "WP-%d" % (_waypoints.size() + 1),
		"x": world_pos.x,
		"y": world_pos.y,
		"z": world_pos.z
	}
	_waypoints.append(wp)


func _on_remove_waypoint() -> void:
	var selected := waypoint_list.get_selected_items()
	if selected.is_empty():
		return
	
	var mission_wp_count: int = GameState.get_mission_waypoints().size()
	var idx: int = selected[0] - mission_wp_count
	
	if idx >= 0 and idx < _waypoints.size():
		_waypoints.remove_at(idx)


func _on_waypoint_selected(_idx: int) -> void:
	remove_waypoint_btn.disabled = waypoint_list.get_selected_items().is_empty()
	send_to_flight_btn.disabled = waypoint_list.get_selected_items().is_empty()


func _on_send_to_flight() -> void:
	var selected := waypoint_list.get_selected_items()
	if selected.is_empty():
		return
	
	var waypoints: Array = GameState.get_mission_waypoints() + _waypoints
	if selected[0] < waypoints.size():
		var wp = waypoints[selected[0]]
		NetworkClient.send_action("relay", "send_waypoint", {
			"name": wp.get("name", ""),
			"x": wp.get("x", 0),
			"y": wp.get("y", 0),
			"z": wp.get("z", 0)
		})


func _on_mark_target() -> void:
	if _selected_contact_id.is_empty():
		return
	NetworkClient.send_action("relay", "mark_target", {"target_id": _selected_contact_id})


func _on_set_waypoint_from_contact() -> void:
	if _selected_contact_id.is_empty():
		return
	
	var contact: GameState.ShipState = GameState.ships.get(_selected_contact_id)
	if contact == null:
		return
	
	var pos := contact.position.to_vector3()
	_waypoints.append({
		"name": contact.display_name,
		"x": pos.x,
		"y": pos.y,
		"z": pos.z
	})


func _on_launch_probe() -> void:
	if _probes_available <= 0:
		return
	
	_probes_available -= 1
	NetworkClient.send_action("relay", "launch_probe", {})


func _on_state_updated() -> void:
	pass  # Updates handled in _process
