shader_type spatial;
render_mode blend_mix, depth_draw_opaque, cull_back, unshaded;

// Engine glow shader for ship engine trails

uniform vec4 inner_color : source_color = vec4(1.0, 0.9, 0.7, 1.0);
uniform vec4 outer_color : source_color = vec4(0.3, 0.5, 1.0, 1.0);
uniform float glow_intensity : hint_range(0.0, 10.0) = 3.0;
uniform float pulse_speed : hint_range(0.0, 10.0) = 4.0;
uniform float pulse_amount : hint_range(0.0, 1.0) = 0.3;
uniform float throttle : hint_range(0.0, 1.0) = 0.5;

varying vec3 local_position;

void vertex() {
	local_position = VERTEX;
}

void fragment() {
	// Calculate distance from center
	float dist = length(local_position.xy);
	
	// Core glow falloff
	float core_glow = exp(-dist * 3.0);
	
	// Outer glow falloff
	float outer_glow = exp(-dist * 1.5) * 0.5;
	
	// Pulsing effect
	float pulse = sin(TIME * pulse_speed) * pulse_amount + (1.0 - pulse_amount);
	
	// Throttle affects intensity
	float intensity = throttle * glow_intensity * pulse;
	
	// Mix inner and outer colors based on distance
	vec3 color = mix(inner_color.rgb, outer_color.rgb, smoothstep(0.0, 0.5, dist));
	
	// Final color with glow
	vec3 final_color = color * (core_glow + outer_glow) * intensity;
	
	ALBEDO = final_color;
	EMISSION = final_color;
	ALPHA = (core_glow + outer_glow * 0.5) * throttle;
}
