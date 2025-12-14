extends Node3D
## Visual representation of a ship in 3D space.
class_name ShipInstance3D

@export var ship_id: String = ""
@export var faction: String = "neutral"

@onready var hull_mesh: MeshInstance3D = $HullMesh
@onready var engine_pivot: Node3D = $EnginePivot
@onready var engine_glow: OmniLight3D = $EnginePivot/EngineGlow
@onready var shield_mesh: MeshInstance3D = $ShieldMesh
@onready var selection_indicator: Node3D = $SelectionIndicator

var _target_position: Vector3 = Vector3.ZERO
var _target_rotation: Vector3 = Vector3.ZERO
var _interpolation_speed: float = 10.0
var _is_selected: bool = false
var _shield_hit_time: float = 0.0
var _damage_level: float = 0.0

# Faction colors
const FACTION_MATERIALS := {
	"player": Color(0.2, 0.5, 0.8),
	"federation": Color(0.2, 0.4, 0.7),
	"klingon": Color(0.6, 0.2, 0.2),
	"romulan": Color(0.2, 0.6, 0.2),
	"neutral": Color(0.5, 0.5, 0.5),
	"hostile": Color(0.8, 0.2, 0.2),
	"civilian": Color(0.6, 0.6, 0.4)
}


func _ready() -> void:
	_setup_materials()
	_create_shield_mesh()
	_create_selection_indicator()


func _process(delta: float) -> void:
	_interpolate_transform(delta)
	_update_engine_glow(delta)
	_update_shield_effect(delta)


func _setup_materials() -> void:
	var mat := hull_mesh.get_surface_override_material(0)
	if mat == null:
		mat = StandardMaterial3D.new()
		hull_mesh.set_surface_override_material(0, mat)
	
	var color: Color = FACTION_MATERIALS.get(faction, FACTION_MATERIALS.neutral)
	mat.albedo_color = Color(color.r * 0.8, color.g * 0.8, color.b * 0.8)
	mat.emission = color
	mat.emission_energy_multiplier = 0.3


func _create_shield_mesh() -> void:
	# Create a sphere mesh for shields
	var sphere := SphereMesh.new()
	sphere.radius = 3.0
	sphere.height = 6.0
	sphere.radial_segments = 32
	sphere.rings = 16
	
	shield_mesh.mesh = sphere
	
	var mat := StandardMaterial3D.new()
	mat.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	mat.albedo_color = Color(0.3, 0.6, 1.0, 0.0)
	mat.emission_enabled = true
	mat.emission = Color(0.3, 0.6, 1.0)
	mat.emission_energy_multiplier = 0.0
	mat.cull_mode = BaseMaterial3D.CULL_DISABLED
	
	shield_mesh.set_surface_override_material(0, mat)
	shield_mesh.visible = false


func _create_selection_indicator() -> void:
	# Create a ring around selected ship
	var torus := TorusMesh.new()
	torus.inner_radius = 3.5
	torus.outer_radius = 4.0
	torus.rings = 32
	torus.ring_segments = 8
	
	var mesh_inst := MeshInstance3D.new()
	mesh_inst.mesh = torus
	mesh_inst.rotation_degrees.x = 90
	
	var mat := StandardMaterial3D.new()
	mat.albedo_color = Color(1.0, 0.8, 0.2)
	mat.emission_enabled = true
	mat.emission = Color(1.0, 0.8, 0.2)
	mat.emission_energy_multiplier = 2.0
	mesh_inst.set_surface_override_material(0, mat)
	
	selection_indicator.add_child(mesh_inst)


func set_target_transform(pos: Vector3, rot: Vector3) -> void:
	_target_position = pos
	_target_rotation = rot


func _interpolate_transform(delta: float) -> void:
	var t := clampf(delta * _interpolation_speed, 0.0, 1.0)
	
	global_position = global_position.lerp(_target_position, t)
	
	# Spherical interpolation for rotation
	var current_quat := Quaternion.from_euler(rotation)
	var target_quat := Quaternion.from_euler(_target_rotation)
	var new_quat := current_quat.slerp(target_quat, t)
	rotation = new_quat.get_euler()


func _update_engine_glow(delta: float) -> void:
	# Calculate engine glow based on velocity
	var ship_state: GameState.ShipState = GameState.ships.get(ship_id)
	if ship_state == null:
		return
	
	var velocity := ship_state.velocity.to_vector3()
	var speed := velocity.length()
	var max_speed := 1000.0
	
	var intensity := clampf(speed / max_speed, 0.1, 1.0)
	engine_glow.light_energy = intensity * 3.0
	
	# Pulse effect
	var pulse := sin(Time.get_ticks_msec() * 0.005) * 0.2 + 0.8
	engine_glow.light_energy *= pulse


func _update_shield_effect(delta: float) -> void:
	if _shield_hit_time > 0:
		_shield_hit_time -= delta
		
		var mat := shield_mesh.get_surface_override_material(0) as StandardMaterial3D
		if mat:
			var alpha := clampf(_shield_hit_time / 0.5, 0.0, 0.6)
			mat.albedo_color.a = alpha
			mat.emission_energy_multiplier = alpha * 3.0
		
		shield_mesh.visible = _shield_hit_time > 0
	else:
		shield_mesh.visible = false


func trigger_shield_hit(facing: String, intensity: float) -> void:
	_shield_hit_time = 0.5
	shield_mesh.visible = true
	
	# Could rotate shield mesh to show hit on specific facing
	var mat := shield_mesh.get_surface_override_material(0) as StandardMaterial3D
	if mat:
		var hit_color := Color(0.5, 0.7, 1.0).lerp(Color(1.0, 0.5, 0.3), clampf(intensity, 0, 1))
		mat.albedo_color = Color(hit_color.r, hit_color.g, hit_color.b, 0.6)
		mat.emission = hit_color


func set_damage_level(level: float) -> void:
	_damage_level = level
	
	# Add visual damage effects
	var mat := hull_mesh.get_surface_override_material(0) as StandardMaterial3D
	if mat:
		# Darken and add red tint based on damage
		var base_color: Color = FACTION_MATERIALS.get(faction, FACTION_MATERIALS.neutral)
		var damage_tint := Color(0.3, 0.1, 0.1)
		mat.albedo_color = base_color.lerp(damage_tint, level * 0.5)


func set_selected(selected: bool) -> void:
	_is_selected = selected
	selection_indicator.visible = selected


func update_from_state(state: GameState.ShipState) -> void:
	ship_id = state.id
	faction = state.faction
	
	set_target_transform(state.position.to_vector3(), state.rotation.to_vector3())
	set_damage_level(1.0 - state.hull / 100.0)
	
	# Update faction colors if changed
	_setup_materials()
