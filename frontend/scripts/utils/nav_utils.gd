extends RefCounted
class_name NavUtils

## Clock ------------------------------------------------------------------

static func format_clock(total_seconds: float) -> String:
	var total := int(total_seconds)
	return "%02d:%02d:%02d" % [total / 3600, (total % 3600) / 60, total % 60]

## Bearings and ranges -----------------------------------------------------

## Compass heading of a ship rotation, 0..360 degrees.
static func heading_deg(rotation: Quaternion) -> float:
	var forward := rotation * Vector3.FORWARD
	return fmod(rad_to_deg(atan2(forward.x, -forward.z)) + 360.0, 360.0)

## Compass heading in radians, for rotating 2D markers.
static func heading_rad(rotation: Quaternion) -> float:
	var forward := rotation * Vector3.FORWARD
	return atan2(forward.x, -forward.z)

## Bearing from one world position to another, 0..360 degrees.
static func bearing_to(from_pos: Vector3, to_pos: Vector3) -> float:
	var dir := (to_pos - from_pos).normalized()
	return fmod(rad_to_deg(atan2(dir.x, dir.z)) + 360.0, 360.0)

static func format_bearing(bearing: float) -> String:
	return "%03.0f°" % bearing

static func format_range(dist_m: float) -> String:
	return "%.1f km" % (dist_m / 1000.0)

static func player_position() -> Vector3:
	var ship = GameState.get_player_ship()
	if ship == null:
		return Vector3.ZERO
	return ship.position.to_vector3()

## Non-player ships as [{id, ship, distance}], nearest first.
static func contacts_by_distance() -> Array:
	var out: Array = []
	if GameState.get_player_ship() == null:
		return out
	var player_pos := player_position()
	for ship_id in GameState.ships:
		if ship_id == GameState.player_ship_id:
			continue
		var ship = GameState.ships[ship_id]
		out.append({"id": ship_id, "ship": ship, "distance": player_pos.distance_to(ship.position.to_vector3())})
	out.sort_custom(func(a, b): return a["distance"] < b["distance"])
	return out

## Distance from the player ship to a {x, y, z} waypoint dict.
static func waypoint_distance(wp: Dictionary) -> float:
	var player = GameState.get_player_ship()
	if player == null:
		return 0.0
	var target := Vector3(wp.get("x", 0), wp.get("y", 0), wp.get("z", 0))
	return player.position.to_vector3().distance_to(target)

## Shared chrome ------------------------------------------------------------

## Flashes an alert overlay. The caller owns the returned tween: kill the previous one before starting a new flash.
static func start_alert_flash(
		host: Node,
		overlay: ColorRect,
		color: Color,
		peak := 0.05,
		half_period := 0.75
	) -> Tween:
	overlay.visible = true
	overlay.color = Color(color.r, color.g, color.b, 0.0)

	var tween := host.create_tween().set_loops()
	tween.tween_property(overlay, "color:a", peak, half_period).set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_IN_OUT)
	tween.tween_property(overlay, "color:a", 0.0, half_period).set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_IN_OUT)

	return tween

static func set_connection_state(overlay: CanvasItem, online: bool, dot: ColorRect = null, label: Label = null) -> void:
	overlay.visible = not online
	if dot != null:
		dot.color = Colors.STATUS_ONLINE if online else Colors.STATUS_OFFLINE
	if label != null:
		label.text = "Connected" if online else "Disconnected"

## Menu exit -----------------------------------------------------------------
## Shift+ESC returns to the launcher. Typing in an input field never exits.

static func is_menu_exit(event: InputEvent, viewport: Viewport) -> bool:
	if event is InputEventKey:
		var key := event as InputEventKey
		if key.pressed and not key.echo and key.keycode == KEY_ESCAPE and key.shift_pressed:
			var focus := viewport.gui_get_focus_owner()
			if focus is LineEdit or focus is TextEdit:
				return false
			return true
	return false

static func exit_to_menu(tree: SceneTree) -> void:
	NetworkClient.disconnect_from_server()
	tree.change_scene_to_file("res://scenes/main.tscn")
