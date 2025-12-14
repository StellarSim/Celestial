extends Control
## Flight control station - throttle, steering, and navigation.

@onready var throttle_slider: VSlider = $MainSplit/LeftSection/ThrottleSection/ThrottleContent/ThrottleSliderContainer/ThrottleSlider
@onready var throttle_value_label: Label = $MainSplit/LeftSection/ThrottleSection/ThrottleContent/ThrottleSliderContainer/ThrottleInfo/ThrottleValue
@onready var throttle_mode_label: Label = $MainSplit/LeftSection/ThrottleSection/ThrottleContent/ThrottleSliderContainer/ThrottleInfo/ThrottleMode
@onready var speed_value: Label = $MainSplit/LeftSection/ThrottleSection/ThrottleContent/ThrottleSliderContainer/ThrottleInfo/SpeedDisplay/SpeedValue

@onready var full_reverse_btn: Button = $MainSplit/LeftSection/ThrottleSection/ThrottleContent/QuickThrottle/FullReverse
@onready var half_reverse_btn: Button = $MainSplit/LeftSection/ThrottleSection/ThrottleContent/QuickThrottle/HalfReverse
@onready var all_stop_btn: Button = $MainSplit/LeftSection/ThrottleSection/ThrottleContent/QuickThrottle/AllStop
@onready var half_forward_btn: Button = $MainSplit/LeftSection/ThrottleSection/ThrottleContent/QuickThrottle/HalfForward
@onready var full_forward_btn: Button = $MainSplit/LeftSection/ThrottleSection/ThrottleContent/QuickThrottle/FullForward

@onready var compass: Control = $MainSplit/LeftSection/SteeringSection/SteeringContent/CompassContainer/Compass
@onready var heading_value: Label = $MainSplit/LeftSection/SteeringSection/SteeringContent/HeadingInfo/CurrentHeading/HeadingValue
@onready var target_heading_value: Label = $MainSplit/LeftSection/SteeringSection/SteeringContent/HeadingInfo/TargetHeading/TargetValue

@onready var hard_port_btn: Button = $MainSplit/LeftSection/SteeringSection/SteeringContent/TurnControls/HardPort
@onready var port_btn: Button = $MainSplit/LeftSection/SteeringSection/SteeringContent/TurnControls/Port
@onready var steady_btn: Button = $MainSplit/LeftSection/SteeringSection/SteeringContent/TurnControls/SteadyOn
@onready var starboard_btn: Button = $MainSplit/LeftSection/SteeringSection/SteeringContent/TurnControls/Starboard
@onready var hard_starboard_btn: Button = $MainSplit/LeftSection/SteeringSection/SteeringContent/TurnControls/HardStarboard

@onready var pos_value: Label = $MainSplit/RightSection/NavStatus/NavContent/Position/PosValue
@onready var vel_value: Label = $MainSplit/RightSection/NavStatus/NavContent/Velocity/VelValue
@onready var bearing_value: Label = $MainSplit/RightSection/NavStatus/NavContent/Bearing/BearingValue

@onready var waypoint_list: ItemList = $MainSplit/RightSection/WaypointSection/WaypointContent/WaypointList
@onready var navigate_btn: Button = $MainSplit/RightSection/WaypointSection/WaypointContent/WaypointActions/NavigateTo
@onready var clear_nav_btn: Button = $MainSplit/RightSection/WaypointSection/WaypointContent/WaypointActions/ClearNav

@onready var autopilot_status: Label = $MainSplit/RightSection/AutopilotSection/AutopilotContent/AutopilotStatus/StatusValue
@onready var autopilot_engage_btn: Button = $MainSplit/RightSection/AutopilotSection/AutopilotContent/AutopilotControls/EngageBtn
@onready var autopilot_mode: OptionButton = $MainSplit/RightSection/AutopilotSection/AutopilotContent/AutopilotControls/ModeSelect

var _current_throttle: float = 0.0
var _target_heading: float = -1.0
var _turn_rate: float = 0.0


func _ready() -> void:
	_setup_controls()
	_connect_signals()


func _process(_delta: float) -> void:
	_update_display()
	queue_redraw()


func _draw() -> void:
	_draw_compass()


func _setup_controls() -> void:
	# Autopilot modes
	autopilot_mode.add_item("HEADING HOLD", 0)
	autopilot_mode.add_item("WAYPOINT NAV", 1)
	autopilot_mode.add_item("INTERCEPT", 2)
	autopilot_mode.add_item("FORMATION", 3)


func _connect_signals() -> void:
	GameState.state_updated.connect(_on_state_updated)
	
	# Throttle controls
	throttle_slider.value_changed.connect(_on_throttle_changed)
	full_reverse_btn.pressed.connect(func(): _set_throttle(-100))
	half_reverse_btn.pressed.connect(func(): _set_throttle(-50))
	all_stop_btn.pressed.connect(func(): _set_throttle(0))
	half_forward_btn.pressed.connect(func(): _set_throttle(50))
	full_forward_btn.pressed.connect(func(): _set_throttle(100))
	
	# Steering controls
	hard_port_btn.pressed.connect(func(): _set_turn_rate(-2.0))
	port_btn.pressed.connect(func(): _set_turn_rate(-1.0))
	steady_btn.pressed.connect(func(): _set_turn_rate(0.0))
	starboard_btn.pressed.connect(func(): _set_turn_rate(1.0))
	hard_starboard_btn.pressed.connect(func(): _set_turn_rate(2.0))
	
	# Waypoint controls
	navigate_btn.pressed.connect(_on_navigate_pressed)
	clear_nav_btn.pressed.connect(_on_clear_nav_pressed)
	
	# Autopilot
	autopilot_engage_btn.toggled.connect(_on_autopilot_toggled)


func _update_display() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	_update_throttle_display(ship)
	_update_nav_display(ship)
	_update_waypoints()


func _update_throttle_display(ship: GameState.ShipState) -> void:
	var velocity := ship.velocity.to_vector3()
	var speed := velocity.length()
	
	speed_value.text = "%.0f m/s" % speed
	throttle_value_label.text = "%+.0f%%" % _current_throttle
	
	# Update throttle mode label
	if _current_throttle > 75:
		throttle_mode_label.text = "FULL AHEAD"
	elif _current_throttle > 25:
		throttle_mode_label.text = "HALF AHEAD"
	elif _current_throttle > 0:
		throttle_mode_label.text = "SLOW AHEAD"
	elif _current_throttle == 0:
		throttle_mode_label.text = "ALL STOP"
	elif _current_throttle > -25:
		throttle_mode_label.text = "SLOW ASTERN"
	elif _current_throttle > -75:
		throttle_mode_label.text = "HALF ASTERN"
	else:
		throttle_mode_label.text = "FULL ASTERN"


func _update_nav_display(ship: GameState.ShipState) -> void:
	var pos := ship.position.to_vector3()
	var vel := ship.velocity.to_vector3()
	
	pos_value.text = "X: %.0f  Y: %.0f  Z: %.0f" % [pos.x, pos.y, pos.z]
	vel_value.text = "%.0f m/s" % vel.length()
	
	# Calculate heading from rotation
	var rotation: Vector3 = ship.rotation.to_vector3()
	var heading := fmod(rad_to_deg(rotation.y) + 360, 360)
	heading_value.text = "%03.0f°" % heading
	
	if _target_heading >= 0:
		target_heading_value.text = "%03.0f°" % _target_heading
	else:
		target_heading_value.text = "---°"
	
	bearing_value.text = "%03.0f°" % heading


func _update_waypoints() -> void:
	# Update waypoint list from mission objectives
	var waypoints: Array = GameState.get_mission_waypoints()
	
	waypoint_list.clear()
	for wp in waypoints:
		var dist := _calculate_distance_to_waypoint(wp)
		waypoint_list.add_item("%s (%.0f km)" % [wp.name, dist / 1000])
	
	navigate_btn.disabled = waypoint_list.get_selected_items().is_empty()


func _calculate_distance_to_waypoint(waypoint: Dictionary) -> float:
	var ship := GameState.get_player_ship()
	if ship == null:
		return 0.0
	
	var ship_pos := ship.position.to_vector3()
	var wp_pos := Vector3(waypoint.get("x", 0), waypoint.get("y", 0), waypoint.get("z", 0))
	return ship_pos.distance_to(wp_pos)


func _draw_compass() -> void:
	var ship := GameState.get_player_ship()
	if ship == null:
		return
	
	var center := compass.size / 2
	var radius: float = min(center.x, center.y) - 10
	
	# Draw compass ring
	compass.draw_arc(center, radius, 0, TAU, 64, Colors.PRIMARY, 2.0)
	
	# Draw cardinal directions
	var cardinals := ["N", "E", "S", "W"]
	for i in range(4):
		var angle := i * PI / 2 - PI / 2
		var pos := center + Vector2(cos(angle), sin(angle)) * (radius - 20)
		# Note: draw_string requires font, use Label nodes in actual implementation
	
	# Draw tick marks
	for i in range(36):
		var angle := deg_to_rad(i * 10) - PI / 2
		var inner_r: float = radius - 8 if i % 9 == 0 else radius - 4
		var outer: Vector2 = center + Vector2(cos(angle), sin(angle)) * radius
		var inner: Vector2 = center + Vector2(cos(angle), sin(angle)) * inner_r
		compass.draw_line(inner, outer, Colors.PRIMARY, 1.0)
	
	# Draw heading indicator (ship)
	var ship_rotation: Vector3 = ship.rotation.to_vector3()
	var heading_rad: float = ship_rotation.y
	var indicator_points := PackedVector2Array([
		center + Vector2(0, -radius * 0.6).rotated(heading_rad),
		center + Vector2(-10, 10).rotated(heading_rad),
		center + Vector2(10, 10).rotated(heading_rad)
	])
	compass.draw_colored_polygon(indicator_points, Colors.PRIMARY)
	
	# Draw target heading indicator
	if _target_heading >= 0:
		var target_rad := deg_to_rad(_target_heading) - PI / 2
		var target_pos: Vector2 = center + Vector2(cos(target_rad), sin(target_rad)) * (radius - 15)
		compass.draw_circle(target_pos, 5, Colors.ALERT_YELLOW)


func _set_throttle(value: float) -> void:
	_current_throttle = value
	throttle_slider.set_value_no_signal(value)
	NetworkClient.send_action("flight", "set_throttle", {"throttle": value / 100.0})


func _on_throttle_changed(value: float) -> void:
	_current_throttle = value
	NetworkClient.send_action("flight", "set_throttle", {"throttle": value / 100.0})


func _set_turn_rate(rate: float) -> void:
	_turn_rate = rate
	NetworkClient.send_action("flight", "set_turn", {"rate": rate})


func _on_navigate_pressed() -> void:
	var selected := waypoint_list.get_selected_items()
	if selected.is_empty():
		return
	
	var waypoints: Array = GameState.get_mission_waypoints()
	if selected[0] < waypoints.size():
		var wp = waypoints[selected[0]]
		NetworkClient.send_action("flight", "navigate_to", {
			"waypoint_id": wp.get("id", ""),
			"x": wp.get("x", 0),
			"y": wp.get("y", 0),
			"z": wp.get("z", 0)
		})


func _on_clear_nav_pressed() -> void:
	_target_heading = -1.0
	NetworkClient.send_action("flight", "clear_navigation", {})


func _on_autopilot_toggled(enabled: bool) -> void:
	var mode_idx := autopilot_mode.selected
	NetworkClient.send_action("flight", "autopilot", {
		"enabled": enabled,
		"mode": mode_idx
	})
	
	if enabled:
		autopilot_status.text = "ENGAGED"
		autopilot_status.add_theme_color_override("font_color", Colors.STATUS_ONLINE)
	else:
		autopilot_status.text = "DISENGAGED"
		autopilot_status.add_theme_color_override("font_color", Colors.STATUS_OFFLINE)


func _on_state_updated() -> void:
	pass  # Updates handled in _process
