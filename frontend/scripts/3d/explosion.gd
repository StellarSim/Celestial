extends Node3D
## Visual explosion effect.
class_name Explosion3D

@export var explosion_type: String = "standard"  # standard, nuclear, emp
@export var scale_factor: float = 1.0

@onready var core_flash: OmniLight3D = $CoreFlash
@onready var particles: GPUParticles3D = $Particles
@onready var shockwave_light: OmniLight3D = $ShockwaveLight

var _lifetime: float = 2.0
var _time: float = 0.0

const TYPE_CONFIGS := {
	"standard": {
		"color": Color(1.0, 0.6, 0.2),
		"energy": 10.0,
		"range": 50.0,
		"lifetime": 1.5
	},
	"nuclear": {
		"color": Color(1.0, 1.0, 0.8),
		"energy": 50.0,
		"range": 200.0,
		"lifetime": 3.0
	},
	"emp": {
		"color": Color(0.3, 0.5, 1.0),
		"energy": 15.0,
		"range": 100.0,
		"lifetime": 2.0
	},
	"beam": {
		"color": Color(1.0, 0.4, 0.2),
		"energy": 5.0,
		"range": 20.0,
		"lifetime": 0.5
	}
}


func _ready() -> void:
	_configure_explosion()
	_setup_particles()


func _process(delta: float) -> void:
	_time += delta
	_update_effects(delta)
	
	if _time >= _lifetime:
		queue_free()


func _configure_explosion() -> void:
	var config: Dictionary = TYPE_CONFIGS.get(explosion_type, TYPE_CONFIGS.standard)
	
	_lifetime = config.lifetime * scale_factor
	
	core_flash.light_color = config.color
	core_flash.light_energy = config.energy * scale_factor
	core_flash.omni_range = config.range * scale_factor
	
	shockwave_light.light_color = config.color
	shockwave_light.light_energy = config.energy * 0.5 * scale_factor
	shockwave_light.omni_range = config.range * 0.2 * scale_factor


func _setup_particles() -> void:
	var config: Dictionary = TYPE_CONFIGS.get(explosion_type, TYPE_CONFIGS.standard)
	
	var process_mat := particles.process_material as ParticleProcessMaterial
	if process_mat:
		process_mat.color = config.color
		process_mat.initial_velocity_min *= scale_factor
		process_mat.initial_velocity_max *= scale_factor
		process_mat.scale_min *= scale_factor
		process_mat.scale_max *= scale_factor
	
	particles.lifetime = _lifetime * 0.75
	
	# Create mesh for particles
	var sphere := SphereMesh.new()
	sphere.radius = 0.2 * scale_factor
	sphere.height = 0.4 * scale_factor
	
	var mat := StandardMaterial3D.new()
	mat.albedo_color = config.color
	mat.emission_enabled = true
	mat.emission = config.color
	mat.emission_energy_multiplier = 3.0
	mat.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	
	particles.draw_pass_1 = sphere


func _update_effects(delta: float) -> void:
	var progress := _time / _lifetime
	
	# Fade out core flash
	var core_decay := 1.0 - pow(progress, 0.5)
	core_flash.light_energy *= pow(0.1, delta * 3)
	
	# Expand shockwave
	var shockwave_progress := clampf(progress * 2, 0, 1)
	var config: Dictionary = TYPE_CONFIGS.get(explosion_type, TYPE_CONFIGS.standard)
	shockwave_light.omni_range = config.range * scale_factor * shockwave_progress
	shockwave_light.light_energy = config.energy * 0.5 * scale_factor * (1.0 - shockwave_progress)


static func create_at(pos: Vector3, type: String = "standard", scale: float = 1.0) -> Explosion3D:
	var scene := load("res://scenes/3d/explosion.tscn")
	var instance: Explosion3D = scene.instantiate()
	instance.explosion_type = type
	instance.scale_factor = scale
	instance.global_position = pos
	return instance
