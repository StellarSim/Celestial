extends Control
## Rear tactical display showing 2D tactical map and ship status overview.
## Can mirror data from other stations or show tactical overview.

@onready var ship_name_label: Label = $MainLayout/Header/HeaderContent/ShipInfo/ShipName
@onready var ship_class_label: Label = $MainLayout/Header/HeaderContent/ShipInfo/ShipClass
@onready var time_label: Label = $MainLayout/Header/HeaderContent/TimeLabel
@onready var alert_indicator: ColorRect = $MainLayout/Header/HeaderContent/AlertIndicator

@onready var hull_bar: ProgressBar = $MainLayout/ContentArea/SidePanel/SidePanelContent/StatusSection/HullRow/HullBar
@onready var shields_bar: ProgressBar = $MainLayout/ContentArea/SidePanel/SidePanelContent/StatusSection/ShieldsRow/ShieldsBar
@onready var power_bar: ProgressBar = $MainLayout/ContentArea/SidePanel/SidePanelContent/StatusSection/PowerRow/PowerBar

@onready var contacts_list: ItemList = $MainLayout/ContentArea/SidePanel/SidePanelContent/ContactsSection/ContactsList

@onready var bow_status: Label = $MainLayout/ContentArea/SidePanel/SidePanelContent/DamageSection/DamageGrid/BowStatus
@onready var stern_status: Label = $MainLayout/ContentArea/SidePanel/SidePanelContent/DamageSection/DamageGrid/SternStatus
@onready var port_status: Label = $MainLayout/ContentArea/SidePanel/SidePanelContent/DamageSection/DamageGrid/PortStatus
@onready var starboard_status: Label = $MainLayout/ContentArea/SidePanel/SidePanelContent/DamageSection/DamageGrid/StarboardStatus

@onready var tactical_map: Control = $MainLayout/ContentArea/TacticalMap
@onready var ships_layer: Control = $MainLayout/ContentArea/TacticalMap/ShipsLayer
@onready var grid_overlay: Control = $MainLayout/ContentArea/TacticalMap/GridOverlay
@onready var range_circles: Control = $MainLayout/ContentArea/TacticalMap/RangeCircles

@onready var connection_dot: ColorRect = $MainLayout/Footer/FooterContent/ConnectionStatus/StatusDot
@onready var connection_text: Label = $MainLayout/Footer/FooterContent/ConnectionStatus/StatusText
@onready var alert_overlay: ColorRect = $AlertOverlay
@onready var disconnect_overlay: ColorRect = $DisconnectOverlay

# Map settings
var map_scale: float = 0.1  # pixels per game unit
var map_center: Vector2 = Vector2.ZERO  # Center on player ship
var zoom_level: float = 1.0

# Ship markers on the map
var _ship_markers: Dictionary = {}

var _alert_tween: Tween = null


func _ready() -> void:
	_connect_signals()
	_setup_map()


func _process(_delta: float) -> void:
	_update_status_display()
	_update_time_display()
	_update_tactical_map()


func _connect_signals() -> void:
	NetworkClient.connected.connect(_on_connected)
	NetworkClient.disconnected.connect(_on_disconnected)
	GameState.state_updated.connect(_on_state_updated)
	GameState.ship_added.connect(_on_ship_added)
	GameState.ship_removed.connect(_on_ship_removed)
	GameState.alert_level_changed.connect(_on_alert_changed)
	
	$MainLayout/ContentArea/TacticalMap/MapControls/ZoomInBtn.pressed.connect(_on_zoom_in)
	$MainLayout/ContentArea/TacticalMap/MapControls/ZoomOutBtn.pressed.connect(_on_zoom_out)
	$MainLayout/Footer/FooterContent/BackButton.pressed.connect(_on_back_pressed)


func _setup_map() -> void:
	# Draw grid
	grid_overlay.queue_redraw()
	range_circles.queue_redraw()
	
	grid_overlay.draw.connect(_draw_grid)
	range_circles.draw.connect(_draw_range_circles)


func _draw_grid() -> void:
	var size := grid_overlay.size
	var grid_spacing := 100.0 * map_scale * zoom_level
	var grid_color := Color(0.15, 0.25, 0.35, 0.5)
	
	if grid_spacing < 20:
		grid_spacing *= 5
	
	var center := size / 2
	
	# Vertical lines
	var x := fmod(center.x, grid_spacing)
	while x < size.x:
		grid_overlay.draw_line(Vector2(x, 0), Vector2(x, size.y), grid_color, 1.0)
		x += grid_spacing
	
	# Horizontal lines  
	var y := fmod(center.y, grid_spacing)
	while y < size.y:
		grid_overlay.draw_line(Vector2(0, y), Vector2(size.x, y), grid_color, 1.0)
		y += grid_spacing


func _draw_range_circles() -> void:
	var center := range_circles.size / 2
	var circle_color := Color(0.2, 0.4, 0.6, 0.3)
	
	# Draw range circles at various distances
	var ranges := [500.0, 1000.0, 2000.0, 5000.0]
	for range_dist in ranges:
		var radius: float = range_dist * map_scale * zoom_level
		if radius > 10 and radius < range_circles.size.x:
			range_circles.draw_arc(center, radius, 0, TAU, 64, circle_color, 1.0)


func _update_status_display() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	ship_name_label.text = ship.name.to_upper() if not ship.name.is_empty() else "USS UNKNOWN"
	ship_class_label.text = ship.ship_class.replace("_", " ").capitalize()
	
	# Status bars
	var hull_percent: float = (ship.hull_integrity / ship.max_hull) * 100.0 if ship.max_hull > 0 else 0.0
	hull_bar.value = hull_percent
	hull_bar.modulate = Colors.get_health_color(hull_percent / 100.0)
	
	var shields_percent: float = (ship.shields / ship.max_shields) * 100.0 if ship.max_shields > 0 else 0.0
	shields_bar.value = shields_percent
	shields_bar.modulate = Colors.get_shield_color(shields_percent / 100.0)
	
	var power_percent: float = (ship.power_available / ship.power_total) * 100.0 if ship.power_total > 0 else 0.0
	power_bar.value = power_percent
	power_bar.modulate = Colors.get_power_color(power_percent / 100.0)
	
	# Damage sections
	_update_damage_section(bow_status, ship.damage_sections.get("bow"))
	_update_damage_section(stern_status, ship.damage_sections.get("stern"))
	_update_damage_section(port_status, ship.damage_sections.get("port"))
	_update_damage_section(starboard_status, ship.damage_sections.get("starboard"))


func _update_damage_section(label: Label, section) -> void:
	if section == null:
		label.text = "100%"
		label.add_theme_color_override("font_color", Colors.DAMAGE_NONE)
		return
	
	var health_percent: float = section.health
	label.text = "%d%%" % int(health_percent)
	label.add_theme_color_override("font_color", Colors.get_damage_section_color(health_percent / 100.0))
	
	# Add indicators for fires/breaches
	var indicators := ""
	if section.fires > 0:
		indicators += " 🔥"
	if section.breaches > 0:
		indicators += " ⚠"
	label.text += indicators


func _update_time_display() -> void:
	var total_seconds := int(GameState.simulation_time)
	var hours := total_seconds / 3600
	var minutes := (total_seconds % 3600) / 60
	var seconds := total_seconds % 60
	time_label.text = "%02d:%02d:%02d" % [hours, minutes, seconds]


func _update_tactical_map() -> void:
	var player_ship := GameState.get_player_ship()
	if player_ship:
		map_center = Vector2(player_ship.position.x, player_ship.position.z)
	
	var map_center_screen := ships_layer.size / 2
	
	# Update ship markers
	for ship_id in _ship_markers:
		var marker: Control = _ship_markers[ship_id]
		var ship := GameState.get_ship(ship_id)
		
		if ship == null:
			marker.visible = false
			continue
		
		marker.visible = true
		
		# Convert world position to map position
		var world_pos := Vector2(ship.position.x, ship.position.z)
		var relative_pos := (world_pos - map_center) * map_scale * zoom_level
		marker.position = map_center_screen + relative_pos - marker.size / 2
		
		# Update rotation to match ship heading
		var rotation := ship.rotation.to_quaternion()
		var forward := rotation * Vector3.FORWARD
		var heading := atan2(forward.x, -forward.z)
		marker.rotation = heading


func _update_contacts_list() -> void:
	contacts_list.clear()
	
	var player_ship := GameState.get_player_ship()
	var player_pos := Vector3.ZERO
	if player_ship:
		player_pos = player_ship.position.to_vector3()
	
	for ship_id in GameState.ships:
		var ship := GameState.get_ship(ship_id)
		if ship.is_player:
			continue
		
		var distance := ship.position.to_vector3().distance_to(player_pos)
		var display_name: String = ship.name if not ship.name.is_empty() else ship_id
		var text := "%s - %.0fm" % [display_name, distance]
		
		contacts_list.add_item(text)
		var idx := contacts_list.item_count - 1
		
		if ship.is_player:
			contacts_list.set_item_custom_fg_color(idx, Colors.FACTION_PLAYER)
		else:
			contacts_list.set_item_custom_fg_color(idx, Colors.FACTION_HOSTILE)


func _on_connected() -> void:
	disconnect_overlay.visible = false
	connection_dot.color = Colors.STATUS_ONLINE
	connection_text.text = "Connected"


func _on_disconnected() -> void:
	disconnect_overlay.visible = true
	connection_dot.color = Colors.STATUS_OFFLINE
	connection_text.text = "Disconnected"


func _on_state_updated() -> void:
	_update_contacts_list()


func _on_ship_added(ship_id: String) -> void:
	if _ship_markers.has(ship_id):
		return
	
	var ship := GameState.get_ship(ship_id)
	if ship == null:
		return
	
	var marker := _create_ship_marker(ship)
	ships_layer.add_child(marker)
	_ship_markers[ship_id] = marker


func _on_ship_removed(ship_id: String) -> void:
	if _ship_markers.has(ship_id):
		_ship_markers[ship_id].queue_free()
		_ship_markers.erase(ship_id)


func _create_ship_marker(ship) -> Control:
	var marker := Control.new()
	marker.custom_minimum_size = Vector2(20, 20)
	marker.size = Vector2(20, 20)
	
	var triangle := ColorRect.new()
	triangle.custom_minimum_size = Vector2(20, 20)
	triangle.size = Vector2(20, 20)
	
	if ship.is_player:
		triangle.color = Colors.FACTION_PLAYER
	else:
		triangle.color = Colors.FACTION_HOSTILE
	
	marker.add_child(triangle)
	
	# Add ship name label
	var label := Label.new()
	label.text = ship.name if not ship.name.is_empty() else ship.id
	label.position = Vector2(25, 0)
	label.add_theme_font_size_override("font_size", 10)
	
	if ship.is_player:
		label.add_theme_color_override("font_color", Colors.FACTION_PLAYER)
	else:
		label.add_theme_color_override("font_color", Colors.FACTION_HOSTILE)
	
	marker.add_child(label)
	
	return marker


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
	_alert_tween.tween_property(alert_overlay, "color:a", 0.1, 0.5)
	_alert_tween.tween_property(alert_overlay, "color:a", 0.0, 0.5)


func _on_zoom_in() -> void:
	zoom_level = minf(zoom_level * 1.5, 5.0)
	$MainLayout/ContentArea/TacticalMap/MapControls/ZoomLabel.text = "%.1fx" % zoom_level
	grid_overlay.queue_redraw()
	range_circles.queue_redraw()


func _on_zoom_out() -> void:
	zoom_level = maxf(zoom_level / 1.5, 0.1)
	$MainLayout/ContentArea/TacticalMap/MapControls/ZoomLabel.text = "%.1fx" % zoom_level
	grid_overlay.queue_redraw()
	range_circles.queue_redraw()


func _on_back_pressed() -> void:
	NetworkClient.disconnect_from_server()
	get_tree().change_scene_to_file("res://scenes/main.tscn")
