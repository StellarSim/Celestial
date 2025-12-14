extends Node3D
## Visual representation of a torpedo/projectile in flight.
class_name TorpedoInstance3D

@export var projectile_id: String = ""
@export var torpedo_type: String = "standard"

@onready var body: MeshInstance3D = $Body
@onready var engine_glow: OmniLight3D = $EngineGlow
@onready var trail: GPUParticles3D = $Trail

var _target_position: Vector3 = Vector3.ZERO
var _velocity: Vector3 = Vector3.ZERO
var _interpolation_speed: float = 15.0

# Type-specific colors
const TYPE_COLORS := {
	"standard": Color(1.0, 0.4, 0.2),
	"emp": Color(0.3, 0.5, 1.0),
	"nuclear": Color(1.0, 0.8, 0.2),
	"mine": Color(0.6, 0.6, 0.6)
}


func _ready() -> void:
	_setup_type_appearance()
	_setup_trail_particles()


func _process(delta: float) -> void:
	_interpolate_position(delta)
	_update_rotation()


func _setup_type_appearance() -> void:
	var color: Color = TYPE_COLORS.get(torpedo_type, TYPE_COLORS.standard)
	
	var mat := body.get_surface_override_material(0) as StandardMaterial3D
	if mat:
		mat.albedo_color = color
		mat.emission = color
	
	engine_glow.light_color = color


func _setup_trail_particles() -> void:
	var color: Color = TYPE_COLORS.get(torpedo_type, TYPE_COLORS.standard)
	
	# Create particle process material
	var process_mat := ParticleProcessMaterial.new()
	process_mat.direction = Vector3(0, 0, 1)
	process_mat.spread = 5.0
	process_mat.initial_velocity_min = 1.0
	process_mat.initial_velocity_max = 2.0
	process_mat.gravity = Vector3.ZERO
	process_mat.scale_min = 0.1
	process_mat.scale_max = 0.2
	process_mat.color = color
	
	trail.process_material = process_mat
	
	# Create simple mesh for particles
	var sphere := SphereMesh.new()
	sphere.radius = 0.05
	sphere.height = 0.1
	
	var draw_mat := StandardMaterial3D.new()
	draw_mat.albedo_color = color
	draw_mat.emission_enabled = true
	draw_mat.emission = color
	draw_mat.emission_energy_multiplier = 2.0
	
	trail.draw_pass_1 = sphere


func set_target_position(pos: Vector3) -> void:
	_target_position = pos


func set_velocity(vel: Vector3) -> void:
	_velocity = vel


func _interpolate_position(delta: float) -> void:
	var t := clampf(delta * _interpolation_speed, 0.0, 1.0)
	global_position = global_position.lerp(_target_position, t)


func _update_rotation() -> void:
	if _velocity.length_squared() > 0.01:
		look_at(global_position + _velocity.normalized())


func update_from_state(state: GameState.ProjectileState) -> void:
	projectile_id = state.id
	torpedo_type = state.projectile_type
	
	set_target_position(state.position.to_vector3())
	set_velocity(state.velocity.to_vector3())
	
	_setup_type_appearance()


func trigger_impact() -> void:
	# Stop emitting trail particles
	trail.emitting = false
	
	# Flash effect
	engine_glow.light_energy = 10.0
	
	# Create tween to fade out
	var tween := create_tween()
	tween.tween_property(engine_glow, "light_energy", 0.0, 0.3)
	tween.tween_callback(queue_free)
