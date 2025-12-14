shader_type spatial;
render_mode blend_mix, depth_draw_opaque, cull_disabled, unshaded;

// Shield ripple effect shader for hit visualization

uniform vec4 shield_color : source_color = vec4(0.3, 0.6, 1.0, 0.5);
uniform float ripple_speed : hint_range(0.0, 10.0) = 3.0;
uniform float ripple_frequency : hint_range(0.0, 50.0) = 10.0;
uniform float ripple_decay : hint_range(0.0, 5.0) = 2.0;
uniform float base_alpha : hint_range(0.0, 1.0) = 0.1;
uniform float hit_alpha : hint_range(0.0, 1.0) = 0.8;
uniform vec3 hit_point = vec3(0.0, 0.0, 1.0);
uniform float hit_time : hint_range(0.0, 10.0) = 0.0;
uniform float hit_radius : hint_range(0.0, 5.0) = 1.0;

varying vec3 world_position;
varying vec3 world_normal;

void vertex() {
	world_position = (MODEL_MATRIX * vec4(VERTEX, 1.0)).xyz;
	world_normal = normalize((MODEL_MATRIX * vec4(NORMAL, 0.0)).xyz);
}

void fragment() {
	// Calculate distance from hit point
	vec3 hit_dir = normalize(hit_point);
	float dist_from_hit = distance(normalize(world_position), hit_dir);
	
	// Calculate ripple effect
	float time_since_hit = hit_time;
	float ripple_travel = time_since_hit * ripple_speed;
	
	// Create expanding ring pattern
	float ring_dist = abs(dist_from_hit - ripple_travel);
	float ring = exp(-ring_dist * ripple_frequency) * exp(-time_since_hit * ripple_decay);
	
	// Add secondary ripples
	float ring2_dist = abs(dist_from_hit - ripple_travel * 0.7);
	float ring2 = exp(-ring2_dist * ripple_frequency * 0.8) * exp(-time_since_hit * ripple_decay * 1.2) * 0.5;
	
	// Combine ripples
	float ripple_intensity = ring + ring2;
	
	// Fresnel effect for edge glow
	float fresnel = pow(1.0 - abs(dot(world_normal, normalize(VIEW))), 3.0);
	
	// Combine base shield visibility with hit effect
	float alpha = base_alpha + fresnel * 0.3 + ripple_intensity * hit_alpha;
	alpha = clamp(alpha, 0.0, 1.0);
	
	// Color shift on impact
	vec3 hit_color = mix(shield_color.rgb, vec3(1.0, 0.8, 0.6), ripple_intensity * 0.5);
	
	ALBEDO = hit_color;
	ALPHA = alpha * shield_color.a;
	
	// Add emission for glow
	EMISSION = hit_color * (0.5 + ripple_intensity * 2.0);
}
