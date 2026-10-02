extends Node
## Polls connected joypads (gamepad, HOTAS, joystick are all the same API)
## and exposes deadzoned axes for helm flight. Tweak AXIS_* to rebind.

signal joypad_changed(connected: bool, joy_name: String)

const DEADZONE := 0.15
const CURVE := 2.0

# Yoke layout: left stick X yaws, right stick X rolls and Y pitches, left
# stick Y is the throttle. Roll matters for getting back onto an axis after a
# hard pitch, so it stays on the yoke.
# HOTAS devices use the same stick indices; check the console log for the
# device name and adjust if needed.
const AXIS_YAW := JOY_AXIS_LEFT_X
const AXIS_ROLL := JOY_AXIS_RIGHT_X
const AXIS_PITCH := JOY_AXIS_RIGHT_Y
const AXIS_THROTTLE := JOY_AXIS_LEFT_Y
const AXIS_TRIGGER := JOY_AXIS_TRIGGER_RIGHT
const BTN_ALL_STOP := JOY_BUTTON_A

var _joy_id: int = -1


func _ready() -> void:
	_refresh()
	Input.joy_connection_changed.connect(_on_joy_connection_changed)


func has_joypad() -> bool:
	return _joy_id >= 0


func device_name() -> String:
	if _joy_id < 0:
		return ""
	return Input.get_joy_name(_joy_id)


func axis(axis_index: int) -> float:
	if _joy_id < 0:
		return 0.0
	return _shape(Input.get_joy_axis(_joy_id, axis_index))


func trigger() -> float:
	if _joy_id < 0:
		return 0.0
	var v := Input.get_joy_axis(_joy_id, AXIS_TRIGGER)
	return _shape((v + 1.0) / 2.0)


func _shape(v: float) -> float:
	if absf(v) < DEADZONE:
		return 0.0
	var t := (absf(v) - DEADZONE) / (1.0 - DEADZONE)
	t = pow(t, CURVE)
	return signf(v) * t


func button_pressed(button_index: int) -> bool:
	if _joy_id < 0:
		return false
	return Input.is_joy_button_pressed(_joy_id, button_index)


func flight_axes() -> Dictionary:
	var yaw := axis(AXIS_YAW)
	var pitch := axis(AXIS_PITCH)
	var roll := axis(AXIS_ROLL)
	var thrust := axis(AXIS_THROTTLE)
	return {
		# Stick forward (negative Y) is ahead, matching the throttle slider.
		"throttle": -thrust,
		# Stick right is starboard: positive yaw turns toward -X, which is
		# port, so the X axis is negated.
		"yaw": -yaw,
		# Stick back is nose up, which is positive pitch about the ship's
		# own X axis.
		"pitch": pitch,
		# Stick right is roll clockwise viewed from behind, which is positive
		# about the ship's forward axis, so it passes through unnegated.
		"roll": roll,
		"active": has_joypad() and (
			thrust != 0.0 or yaw != 0.0 or pitch != 0.0 or roll != 0.0
		),
	}


func _refresh() -> void:
	var pads := Input.get_connected_joypads()
	_joy_id = pads[0] if not pads.is_empty() else -1


func _on_joy_connection_changed(device: int, connected: bool) -> void:
	if connected:
		if _joy_id < 0:
			_joy_id = device
		print("[Joy] Connected: ", Input.get_joy_name(device))
	else:
		if device == _joy_id:
			_refresh()
		print("[Joy] Disconnected device ", device)
	joypad_changed.emit(connected, Input.get_joy_name(device) if connected else "")
