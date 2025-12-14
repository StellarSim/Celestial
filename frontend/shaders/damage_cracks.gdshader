shader_type spatial;
render_mode blend_mix, depth_draw_opaque, cull_back;

// Hull damage cracks shader for visual damage indication

uniform sampler2D albedo_texture : source_color;
uniform sampler2D normal_texture : hint_normal;
uniform sampler2D crack_texture : hint_default_white;
uniform sampler2D emission_texture : hint_default_black;

uniform vec4 base_color : source_color = vec4(0.4, 0.4, 0.45, 1.0);
uniform vec4 crack_color : source_color = vec4(0.1, 0.05, 0.02, 1.0);
uniform vec4 glow_color : source_color = vec4(1.0, 0.4, 0.1, 1.0);

uniform float damage_level : hint_range(0.0, 1.0) = 0.0;
uniform float crack_intensity : hint_range(0.0, 2.0) = 1.0;
uniform float glow_intensity : hint_range(0.0, 5.0) = 2.0;
uniform float metallic : hint_range(0.0, 1.0) = 0.7;
uniform float roughness : hint_range(0.0, 1.0) = 0.4;

uniform float crack_scale : hint_range(0.1, 10.0) = 1.0;
uniform float crack_threshold : hint_range(0.0, 1.0) = 0.5;
uniform float glow_pulse_speed : hint_range(0.0, 10.0) = 2.0;

varying vec3 world_position;

void vertex() {
	world_position = (MODEL_MATRIX * vec4(VERTEX, 1.0)).xyz;
}

void fragment() {
	vec2 uv = UV * crack_scale;
	
	// Sample textures
	vec4 albedo = texture(albedo_texture, UV);
	vec3 normal_map = texture(normal_texture, UV).rgb;
	float crack_mask = texture(crack_texture, uv).r;
	float emission_mask = texture(emission_texture, uv).r;
	
	// Calculate crack visibility based on damage level
	float crack_visibility = smoothstep(crack_threshold, crack_threshold + 0.2, damage_level);
	
	// Procedural crack pattern based on noise-like effect
	float noise1 = fract(sin(dot(uv, vec2(12.9898, 78.233))) * 43758.5453);
	float noise2 = fract(sin(dot(uv * 1.5, vec2(78.233, 12.9898))) * 23421.6312);
	float procedural_crack = step(1.0 - damage_level * 0.8, noise1 * noise2);
	
	// Combine texture and procedural cracks
	float final_crack = max(crack_mask, procedural_crack) * crack_visibility * crack_intensity;
	
	// Calculate glow for hot damage areas
	float pulse = sin(TIME * glow_pulse_speed) * 0.3 + 0.7;
	float glow = final_crack * damage_level * glow_intensity * pulse;
	
	// Mix base color with crack color
	vec3 final_color = mix(base_color.rgb * albedo.rgb, crack_color.rgb, final_crack * 0.8);
	
	// Add scorching around cracks
	float scorch_radius = final_crack * 0.3;
	vec3 scorch_color = mix(final_color, vec3(0.1, 0.05, 0.02), scorch_radius);
	
	ALBEDO = scorch_color;
	METALLIC = metallic * (1.0 - final_crack * 0.5);
	ROUGHNESS = roughness + final_crack * 0.3;
	NORMAL_MAP = normal_map;
	
	// Emission for hot cracks
	EMISSION = glow_color.rgb * glow * smoothstep(0.3, 0.7, damage_level);
}
