extends Node3D
## Visual beam weapon effect.
class_name BeamEffect3D

@export var origin: Vector3 = Vector3.ZERO
@export var target: Vector3 = Vector3.FORWARD * 10
@export var beam_color: Color = Color(1.0, 0.4, 0.2)
@export var duration: float = 0.5
@export var beam_width: float = 0.2

@onready var beam_mesh: MeshInstance3D = $BeamMesh
@onready var origin_light: OmniLight3D = $OriginLight
@onready var impact_light: OmniLight3D = $ImpactLight

var _time: float = 0.0
var _active: bool = true


func _ready() -> void:
	_create_beam_mesh()
	_setup_lights()


func _process(delta: float) -> void:
	if not _active:
		return
	
	_time += delta
	_update_beam(delta)
	
	if _time >= duration:
		_fade_out()


func _create_beam_mesh() -> void:
	var direction := (target - origin).normalized()
	var length := origin.distance_to(target)
	
	# Create cylinder mesh for beam
	var cylinder := CylinderMesh.new()
	cylinder.top_radius = beam_width
	cylinder.bottom_radius = beam_width
	cylinder.height = length
	cylinder.radial_segments = 8
	cylinder.rings = 1
	
	beam_mesh.mesh = cylinder
	
	# Position and orient the beam
	beam_mesh.global_position = origin + direction * (length / 2)
	beam_mesh.look_at(target)
	beam_mesh.rotate_object_local(Vector3.RIGHT, PI / 2)
	
	# Set material color
	var mat := beam_mesh.material_override as StandardMaterial3D
	if mat:
		mat.albedo_color = Color(beam_color.r, beam_color.g, beam_color.b, 0.8)
		mat.emission = beam_color


func _setup_lights() -> void:
	origin_light.global_position = origin
	origin_light.light_color = beam_color
	
	impact_light.global_position = target
	impact_light.light_color = beam_color


func _update_beam(delta: float) -> void:
	var progress := _time / duration
	
	# Flicker effect
	var flicker := randf_range(0.8, 1.2)
	
	var mat := beam_mesh.material_override as StandardMaterial3D
	if mat:
		mat.emission_energy_multiplier = 5.0 * flicker * (1.0 - progress * 0.5)
	
	origin_light.light_energy = 3.0 * flicker
	impact_light.light_energy = 5.0 * flicker


func _fade_out() -> void:
	_active = false
	
	var tween := create_tween()
	tween.set_parallel(true)
	
	var mat := beam_mesh.material_override as StandardMaterial3D
	if mat:
		tween.tween_property(mat, "albedo_color:a", 0.0, 0.2)
		tween.tween_property(mat, "emission_energy_multiplier", 0.0, 0.2)
	
	tween.tween_property(origin_light, "light_energy", 0.0, 0.2)
	tween.tween_property(impact_light, "light_energy", 0.0, 0.2)
	
	tween.chain().tween_callback(queue_free)


func set_endpoints(from: Vector3, to: Vector3) -> void:
	origin = from
	target = to
	
	if beam_mesh:
		_create_beam_mesh()
		_setup_lights()


static func create_beam(from: Vector3, to: Vector3, color: Color = Color(1, 0.4, 0.2), width: float = 0.2, dur: float = 0.5) -> BeamEffect3D:
	var scene := load("res://scenes/3d/beam_effect.tscn")
	var instance: BeamEffect3D = scene.instantiate()
	instance.origin = from
	instance.target = to
	instance.beam_color = color
	instance.beam_width = width
	instance.duration = dur
	return instance
