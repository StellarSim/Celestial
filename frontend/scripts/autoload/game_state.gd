extends Node
## Global game state manager that stores authoritative state from the backend.
## All game state flows through here - clients should read from this, never modify directly.

signal state_updated
signal ship_added(ship_id: String)
signal ship_removed(ship_id: String)
signal projectile_added(projectile_id: String)
signal projectile_removed(projectile_id: String)
signal mission_event(event_name: String, data: Dictionary)
signal paused_changed(is_paused: bool)
signal alert_level_changed(level: String)

# Client configuration
var client_role: String = ""
var client_id: String = ""
var is_gm: bool = false
var is_display: bool = false

# Simulation state
var simulation_time: float = 0.0
var is_paused: bool = false
var alert_level: String = "normal"

# Game objects
var ships: Dictionary = {}  # ship_id -> ShipState
var projectiles: Dictionary = {}  # projectile_id -> ProjectileState
var player_ship_id: String = ""

# Previous state for interpolation
var _previous_ships: Dictionary = {}
var _previous_projectiles: Dictionary = {}
var _last_update_time: float = 0.0
var _update_interval: float = 0.05  # 20Hz from server


class Vector3State:
	var x: float = 0.0
	var y: float = 0.0
	var z: float = 0.0
	
	func _init(data: Dictionary = {}) -> void:
		if data.has("x"): x = data.x
		if data.has("y"): y = data.y
		if data.has("z"): z = data.z
	
	func to_vector3() -> Vector3:
		return Vector3(x, y, z)
	
	func to_dict() -> Dictionary:
		return {"x": x, "y": y, "z": z}


class QuaternionState:
	var x: float = 0.0
	var y: float = 0.0
	var z: float = 0.0
	var w: float = 1.0
	
	func _init(data: Dictionary = {}) -> void:
		if data.has("x"): x = data.x
		if data.has("y"): y = data.y
		if data.has("z"): z = data.z
		if data.has("w"): w = data.w
	
	func to_quaternion() -> Quaternion:
		return Quaternion(x, y, z, w)
	
	func to_dict() -> Dictionary:
		return {"x": x, "y": y, "z": z, "w": w}


class EngineState:
	var thrust: Vector3State
	var health: float = 100.0
	var enabled: bool = true
	
	func _init(data: Dictionary = {}) -> void:
		thrust = Vector3State.new(data.get("thrust", {}))
		if data.has("health"): health = data.health
		if data.has("enabled"): enabled = data.enabled


class TorpedoBayState:
	var bay_id: int = 0
	var armed: bool = false
	var loaded: bool = false
	var locked: bool = false
	var ammo: int = 0
	var max_ammo: int = 20
	var cooldown: float = 0.0
	var target_id: String = ""
	
	func _init(data: Dictionary = {}) -> void:
		if data.has("bay_id"): bay_id = data.bay_id
		if data.has("armed"): armed = data.armed
		if data.has("loaded"): loaded = data.loaded
		if data.has("locked"): locked = data.locked
		if data.has("ammo"): ammo = data.ammo
		if data.has("max_ammo"): max_ammo = data.max_ammo
		if data.has("cooldown"): cooldown = data.cooldown
		if data.has("target_id"): target_id = data.target_id


class PhaserArrayState:
	var array_id: String = ""
	var facing: Vector3State
	var health: float = 100.0
	var cooldown: float = 0.0
	var power_level: float = 100.0
	
	func _init(data: Dictionary = {}) -> void:
		if data.has("array_id"): array_id = data.array_id
		facing = Vector3State.new(data.get("facing", {}))
		if data.has("health"): health = data.health
		if data.has("cooldown"): cooldown = data.cooldown
		if data.has("power_level"): power_level = data.power_level


class WeaponsState:
	var torpedo_bays: Array[TorpedoBayState] = []
	var phaser_arrays: Array[PhaserArrayState] = []
	
	func _init(data: Dictionary = {}) -> void:
		torpedo_bays.clear()
		phaser_arrays.clear()
		
		if data.has("torpedo_bays"):
			for bay_data in data.torpedo_bays:
				torpedo_bays.append(TorpedoBayState.new(bay_data))
		
		if data.has("phaser_arrays"):
			for array_data in data.phaser_arrays:
				phaser_arrays.append(PhaserArrayState.new(array_data))


class DamageSectionState:
	var health: float = 100.0
	var fires: int = 0
	var breaches: int = 0
	var crew_trapped: int = 0
	
	func _init(data: Dictionary = {}) -> void:
		if data.has("health"): health = data.health
		if data.has("fires"): fires = data.fires
		if data.has("breaches"): breaches = data.breaches
		if data.has("crew_trapped"): crew_trapped = data.crew_trapped


class LifeSupportState:
	var oxygen_level: float = 100.0
	var temperature: float = 21.0
	var gravity: float = 1.0
	var enabled: bool = true
	
	func _init(data: Dictionary = {}) -> void:
		if data.has("oxygen_level"): oxygen_level = data.oxygen_level
		if data.has("temperature"): temperature = data.temperature
		if data.has("gravity"): gravity = data.gravity
		if data.has("enabled"): enabled = data.enabled


class CrewMemberState:
	var health: float = 100.0
	var stress: float = 0.0
	var available: bool = true
	
	func _init(data: Dictionary = {}) -> void:
		if data.has("health"): health = data.health
		if data.has("stress"): stress = data.stress
		if data.has("available"): available = data.available


class SensorsState:
	var health: float = 100.0
	var enabled: bool = true
	var scan_active: bool = false
	var scan_target: String = ""
	var scan_progress: float = 0.0
	
	func _init(data: Dictionary = {}) -> void:
		if data.has("health"): health = data.health
		if data.has("enabled"): enabled = data.enabled
		if data.has("scan_active"): scan_active = data.scan_active
		if data.has("scan_target"): scan_target = data.scan_target
		if data.has("scan_progress"): scan_progress = data.scan_progress


class CommsState:
	var health: float = 100.0
	var enabled: bool = true
	var hailing: bool = false
	var hail_target: String = ""
	
	func _init(data: Dictionary = {}) -> void:
		if data.has("health"): health = data.health
		if data.has("enabled"): enabled = data.enabled
		if data.has("hailing"): hailing = data.hailing
		if data.has("hail_target"): hail_target = data.hail_target


class ShipState:
	var id: String = ""
	var name: String = ""
	var is_player: bool = false
	var ship_class: String = ""
	
	var position: Vector3State
	var velocity: Vector3State
	var rotation: QuaternionState
	
	var hull_integrity: float = 1000.0
	var max_hull: float = 1000.0
	var shields: float = 1000.0
	var max_shields: float = 1000.0
	var shields_enabled: bool = true
	
	var power_available: float = 1000.0
	var power_total: float = 1000.0
	
	var engines: EngineState
	var weapons: WeaponsState
	
	var shield_facings: Dictionary = {
		"fore": 250.0,
		"aft": 250.0,
		"port": 250.0,
		"starboard": 250.0
	}
	
	var power_breakers: Dictionary = {
		"reactor": true,
		"engines": true,
		"shields": true,
		"weapons": true,
		"sensors": true,
		"comms": true,
		"life_support": true,
		"navigation": true
	}
	
	var damage_sections: Dictionary = {}
	var life_support: LifeSupportState
	var crew: Dictionary = {}
	var sensors: SensorsState
	var communications: CommsState
	
	var docked: bool = false
	var docking_target: String = ""
	var alert_level: String = "normal"
	
	# Interpolation helpers
	var _prev_position: Vector3
	var _prev_rotation: Quaternion
	
	func _init(data: Dictionary = {}) -> void:
		position = Vector3State.new()
		velocity = Vector3State.new()
		rotation = QuaternionState.new()
		engines = EngineState.new()
		weapons = WeaponsState.new()
		life_support = LifeSupportState.new()
		sensors = SensorsState.new()
		communications = CommsState.new()
		
		update_from_dict(data)
	
	func update_from_dict(data: Dictionary) -> void:
		# Store previous values for interpolation
		_prev_position = position.to_vector3()
		_prev_rotation = rotation.to_quaternion()
		
		if data.has("id"): id = data.id
		if data.has("name"): name = data.name
		if data.has("is_player"): is_player = data.is_player
		if data.has("class"): ship_class = data["class"]
		
		if data.has("position"): position = Vector3State.new(data.position)
		if data.has("velocity"): velocity = Vector3State.new(data.velocity)
		if data.has("rotation"): rotation = QuaternionState.new(data.rotation)
		
		if data.has("hull_integrity"): hull_integrity = data.hull_integrity
		if data.has("max_hull"): max_hull = data.max_hull
		if data.has("shields"): shields = data.shields
		if data.has("max_shields"): max_shields = data.max_shields
		if data.has("shields_enabled"): shields_enabled = data.shields_enabled
		
		if data.has("power_available"): power_available = data.power_available
		if data.has("power_total"): power_total = data.power_total
		
		if data.has("engines"): engines = EngineState.new(data.engines)
		if data.has("weapons"): weapons = WeaponsState.new(data.weapons)
		
		if data.has("shield_facings"): shield_facings = data.shield_facings.duplicate()
		if data.has("power_breakers"): power_breakers = data.power_breakers.duplicate()
		
		if data.has("damage_sections"):
			damage_sections.clear()
			for section_name in data.damage_sections:
				damage_sections[section_name] = DamageSectionState.new(data.damage_sections[section_name])
		
		if data.has("life_support"): life_support = LifeSupportState.new(data.life_support)
		
		if data.has("crew"):
			crew.clear()
			for role in data.crew:
				crew[role] = CrewMemberState.new(data.crew[role])
		
		if data.has("sensors"): sensors = SensorsState.new(data.sensors)
		if data.has("communications"): communications = CommsState.new(data.communications)
		
		if data.has("docked"): docked = data.docked
		if data.has("docking_target"): docking_target = data.docking_target
		if data.has("alert_level"): alert_level = data.alert_level
	
	func get_interpolated_position(t: float) -> Vector3:
		return _prev_position.lerp(position.to_vector3(), t)
	
	func get_interpolated_rotation(t: float) -> Quaternion:
		return _prev_rotation.slerp(rotation.to_quaternion(), t)


class ProjectileState:
	var id: String = ""
	var type: String = ""
	var position: Vector3State
	var velocity: Vector3State
	var owner_id: String = ""
	var target_id: String = ""
	
	var _prev_position: Vector3
	
	func _init(data: Dictionary = {}) -> void:
		position = Vector3State.new()
		velocity = Vector3State.new()
		update_from_dict(data)
	
	func update_from_dict(data: Dictionary) -> void:
		_prev_position = position.to_vector3()
		
		if data.has("id"): id = data.id
		if data.has("type"): type = data.type
		if data.has("position"): position = Vector3State.new(data.position)
		if data.has("velocity"): velocity = Vector3State.new(data.velocity)
		if data.has("owner_id"): owner_id = data.owner_id
		if data.has("target_id"): target_id = data.target_id
	
	func get_interpolated_position(t: float) -> Vector3:
		return _prev_position.lerp(position.to_vector3(), t)


func _ready() -> void:
	# Generate a unique client ID if not set
	if client_id.is_empty():
		client_id = "client_" + str(randi())


func get_interpolation_factor() -> float:
	var time_since_update := Time.get_ticks_msec() / 1000.0 - _last_update_time
	return clampf(time_since_update / _update_interval, 0.0, 1.0)


func apply_state_update(data: Dictionary) -> void:
	_last_update_time = Time.get_ticks_msec() / 1000.0
	
	if data.has("time"):
		simulation_time = data.time
	
	# Handle pause state
	if data.has("paused"):
		var was_paused := is_paused
		is_paused = data.paused
		if was_paused != is_paused:
			paused_changed.emit(is_paused)
	
	# Update ships
	if data.has("ships"):
		var received_ship_ids: Array[String] = []
		
		for ship_data in data.ships:
			var ship_id: String = ship_data.id
			received_ship_ids.append(ship_id)
			
			if ships.has(ship_id):
				ships[ship_id].update_from_dict(ship_data)
			else:
				ships[ship_id] = ShipState.new(ship_data)
				ship_added.emit(ship_id)
			
			# Track player ship
			if ship_data.get("is_player", false):
				player_ship_id = ship_id
		
		# Remove ships no longer in state
		var ships_to_remove: Array[String] = []
		for ship_id in ships:
			if ship_id not in received_ship_ids:
				ships_to_remove.append(ship_id)
		
		for ship_id in ships_to_remove:
			ships.erase(ship_id)
			ship_removed.emit(ship_id)
	
	# Update projectiles
	if data.has("projectiles"):
		var received_projectile_ids: Array[String] = []
		
		for proj_data in data.projectiles:
			var proj_id: String = proj_data.id
			received_projectile_ids.append(proj_id)
			
			if projectiles.has(proj_id):
				projectiles[proj_id].update_from_dict(proj_data)
			else:
				projectiles[proj_id] = ProjectileState.new(proj_data)
				projectile_added.emit(proj_id)
		
		# Remove projectiles no longer in state
		var projectiles_to_remove: Array[String] = []
		for proj_id in projectiles:
			if proj_id not in received_projectile_ids:
				projectiles_to_remove.append(proj_id)
		
		for proj_id in projectiles_to_remove:
			projectiles.erase(proj_id)
			projectile_removed.emit(proj_id)
	
	state_updated.emit()


func get_player_ship() -> ShipState:
	if player_ship_id.is_empty() or not ships.has(player_ship_id):
		return null
	return ships[player_ship_id]


func get_ship(ship_id: String) -> ShipState:
	return ships.get(ship_id)


func get_all_ships() -> Array:
	return ships.values()


func get_enemy_ships() -> Array:
	var enemies: Array = []
	for ship in ships.values():
		if not ship.is_player:
			enemies.append(ship)
	return enemies


func clear_state() -> void:
	ships.clear()
	projectiles.clear()
	player_ship_id = ""
	simulation_time = 0.0
	is_paused = false
