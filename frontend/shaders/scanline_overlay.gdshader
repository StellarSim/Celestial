shader_type canvas_item;

// Scanline overlay shader for retro CRT effect on displays

uniform float scanline_opacity : hint_range(0.0, 1.0) = 0.1;
uniform float scanline_frequency : hint_range(1.0, 500.0) = 200.0;
uniform float vignette_intensity : hint_range(0.0, 1.0) = 0.3;
uniform float noise_intensity : hint_range(0.0, 0.5) = 0.02;
uniform float chromatic_aberration : hint_range(0.0, 0.01) = 0.001;
uniform float flicker_intensity : hint_range(0.0, 0.1) = 0.02;
uniform float glow_amount : hint_range(0.0, 1.0) = 0.2;

float random(vec2 st) {
	return fract(sin(dot(st.xy, vec2(12.9898, 78.233))) * 43758.5453123);
}

void fragment() {
	vec2 uv = UV;
	
	// Chromatic aberration
	float ca = chromatic_aberration;
	vec4 color;
	color.r = texture(TEXTURE, uv + vec2(ca, 0.0)).r;
	color.g = texture(TEXTURE, uv).g;
	color.b = texture(TEXTURE, uv - vec2(ca, 0.0)).b;
	color.a = texture(TEXTURE, uv).a;
	
	// Scanlines
	float scanline = sin(uv.y * scanline_frequency * PI) * 0.5 + 0.5;
	scanline = pow(scanline, 1.5) * scanline_opacity;
	
	// Vignette
	vec2 center_dist = uv - vec2(0.5);
	float vignette = 1.0 - dot(center_dist, center_dist) * vignette_intensity * 2.0;
	vignette = clamp(vignette, 0.0, 1.0);
	
	// Noise
	float noise = random(uv + TIME * 0.1) * noise_intensity;
	
	// Flicker
	float flicker = random(vec2(TIME * 10.0, 0.0)) * flicker_intensity;
	
	// Apply effects
	color.rgb = color.rgb * (1.0 - scanline) * vignette;
	color.rgb += noise;
	color.rgb *= (1.0 + flicker);
	
	// Subtle glow
	vec4 glow_color = texture(TEXTURE, uv);
	color.rgb += glow_color.rgb * glow_amount * 0.5;
	
	COLOR = color;
}
