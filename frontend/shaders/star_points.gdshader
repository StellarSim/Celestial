shader_type spatial;
render_mode unshaded, cull_disabled, blend_add, shadows_disabled, depth_draw_never;

uniform vec4 tint : source_color = vec4(1.0, 1.0, 1.0, 1.0);
uniform float energy : hint_range(0.0, 8.0) = 1.0;

// Viewport size in pixels, so the pixel-to-clip-space conversion stays correct
// at any window size or stretch setting.SW
uniform vec2 viewport_px = vec2(1920.0, 1080.0);

// Smallest on-screen radius in pixels. This floor is what keeps the faintest
// distant stars from shimmering as they cross pixel boundaries.
uniform float min_radius_px : hint_range(0.5, 8.0) = 1.5;

void vertex() {
	vec3 view_pos = (VIEW_MATRIX * MODEL_MATRIX * vec4(0.0, 0.0, 0.0, 1.0)).xyz;

	// The per-instance scale is read off the instance basis and used directly
	// as a pixel radius, so the layer scale ranges set star size instead of
	// being swallowed by the distance to the shell.
	float radius_px = max(length(MODEL_MATRIX[0].xyz), min_radius_px);

	float side = (UV.x - 0.5) * 2.0;
	float along = (UV.y - 0.5) * 2.0;
	vec2 offset_px = vec2(side, along) * radius_px;

	vec4 clip = PROJECTION_MATRIX * vec4(view_pos, 1.0);
	// Dividing by w keeps the offset a constant size on screen at any depth.
	clip.xy += offset_px * 2.0 / viewport_px * clip.w;
	POSITION = clip;
}

void fragment() {
	// Soft round falloff so a star does not read as a hard square.
	vec2 d = (UV - 0.5) * 2.0;
	float shape = max(1.0 - dot(d, d), 0.0);
	ALBEDO = tint.rgb * energy * shape;
	ALPHA = shape;
}
