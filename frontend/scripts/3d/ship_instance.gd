extends Node3D
## Visual representation of a ship in 3D space.
class_name ShipInstance3D

@export var ship_id: String = ""
@export var faction: String = "neutral"

const DAMAGE_SHADER := preload("res://shaders/damage_cracks.gdshader")
const SHIELD_SHADER := preload("res://shaders/shield_ripple.gdshader")
const ENGINE_GLOW_COLOR := Color(0.5, 0.7, 1.0)

@onready var hull_mesh: MeshInstance3D = $HullMesh
@onready var engine_pivot: Node3D = $EnginePivot
@onready var engine_glow: OmniLight3D = %EngineGlow
@onready var shield_mesh: MeshInstance3D = $ShieldMesh
@onready var selection_indicator: Node3D = $SelectionIndicator

var _is_selected: bool = false
var _shield_hit_time: float = 0.0
var _damage_level: float = 0.0

func _faction_material_color() -> Color:
	return GameState.get_faction_color(faction)


func _ready() -> void:
	_setup_materials()
	_create_shield_mesh()
	_create_selection_indicator()


func _process(delta: float) -> void:
	_update_engine_glow(delta)
	_update_shield_effect(delta)


func _setup_materials() -> void:
	var mat := hull_mesh.get_surface_override_material(0)
	if mat == null:
		mat = ShaderMaterial.new()
		mat.shader = DAMAGE_SHADER
		hull_mesh.set_surface_override_material(0, mat)

	if mat is ShaderMaterial:
		var color: Color = _faction_material_color()
		mat.set_shader_parameter("base_color", Color(color.r * 0.8, color.g * 0.8, color.b * 0.8, 1.0))
		mat.set_shader_parameter("damage_level", _damage_level)
		return

	var flat := mat as StandardMaterial3D
	var color: Color = _faction_material_color()
	flat.albedo_color = Color(color.r * 0.8, color.g * 0.8, color.b * 0.8)
	flat.emission = color
	flat.emission_energy_multiplier = 0.3


func _create_shield_mesh() -> void:
	# Create a sphere mesh for shields
	var sphere := SphereMesh.new()
	sphere.radius = 3.0
	sphere.height = 6.0
	sphere.radial_segments = 32
	sphere.rings = 16

	shield_mesh.mesh = sphere

	var mat := ShaderMaterial.new()
	mat.shader = SHIELD_SHADER
	mat.set_shader_parameter("shield_color", Color(0.3, 0.6, 1.0, 0.5))
	mat.set_shader_parameter("base_alpha", 0.0)

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


func _update_engine_glow(_delta: float) -> void:
	var ship_state: GameState.ShipState = GameState.ships.get(ship_id)
	if ship_state == null:
		return

	# The glow follows the server's commanded throttle, not a local timer.
	var throttle := 0.0
	if ship_state.engines_list.size() > 0:
		throttle = absf(ship_state.engines_list[0].thrust.to_vector3().length() / 100000.0)
	throttle = clampf(throttle, 0.0, 1.0)

	var pulse := sin(Time.get_ticks_msec() * 0.005) * 0.2 + 0.8
	engine_glow.light_energy = (0.1 + throttle) * 3.0 * pulse
	engine_glow.light_color = ENGINE_GLOW_COLOR.lerp(Color.RED, 1.0 - throttle)


func _update_shield_effect(delta: float) -> void:
	var mat := shield_mesh.get_surface_override_material(0) as ShaderMaterial
	if mat == null:
		return

	if _shield_hit_time > 0:
		_shield_hit_time -= delta
		mat.set_shader_parameter("hit_time", maxf(_shield_hit_time, 0.0))
		mat.set_shader_parameter("base_alpha", 0.0)
		shield_mesh.visible = true
	else:
		mat.set_shader_parameter("hit_time", 0.0)
		shield_mesh.visible = false


func trigger_shield_hit(facing: String, intensity: float) -> void:
	_shield_hit_time = 0.5
	shield_mesh.visible = true

	var mat := shield_mesh.get_surface_override_material(0) as ShaderMaterial
	if mat == null:
		return

	var facing_dirs := {
		"fore": Vector3(0, 0, -1),
		"aft": Vector3(0, 0, 1),
		"port": Vector3(-1, 0, 0),
		"starboard": Vector3(1, 0, 0),
	}
	var dir: Vector3 = facing_dirs.get(facing, Vector3(0, 0, -1))
	var local := global_transform.basis.inverse() * dir
	mat.set_shader_parameter("hit_point", local)
	mat.set_shader_parameter("hit_time", _shield_hit_time)
	mat.set_shader_parameter("hit_alpha", clampf(intensity, 0.0, 1.0))


func set_damage_level(level: float) -> void:
	_damage_level = clampf(level, 0.0, 1.0)

	var mat := hull_mesh.get_surface_override_material(0)
	if mat is ShaderMaterial:
		(mat as ShaderMaterial).set_shader_parameter("damage_level", _damage_level)
		return

	var flat := mat as StandardMaterial3D
	if flat:
		var base_color: Color = _faction_material_color()
		var damage_tint := Color(0.3, 0.1, 0.1)
		flat.albedo_color = base_color.lerp(damage_tint, _damage_level * 0.5)


func set_selected(selected: bool) -> void:
	_is_selected = selected
	selection_indicator.visible = selected


func update_from_state(state: GameState.ShipState) -> void:
	ship_id = state.id
	faction = state.faction
	
	set_damage_level(clampf(1.0 - state.hull_integrity / 100.0, 0.0, 1.0))
	
	# Update faction colors if changed
	_setup_materials()
