shader_type sky;

uniform vec4 space_color : source_color = vec4(0.002, 0.002, 0.005, 1.0);
uniform vec4 nebula_color_a : source_color = vec4(0.20, 0.09, 0.34, 1.0);
uniform vec4 nebula_color_b : source_color = vec4(0.05, 0.17, 0.32, 1.0);
uniform vec4 nebula_color_c : source_color = vec4(0.32, 0.09, 0.17, 1.0);

uniform float nebula_scale : hint_range(0.5, 10.0) = 2.0;
uniform float nebula_intensity : hint_range(0.0, 1.0) = 0.18;
uniform float nebula_coverage : hint_range(0.0, 1.0) = 0.45;
// Accumulated distance travelled in metres, fed in from the ship each frame.
// Driving the drift from distance covered rather than TIME means the nebula
// stills when the ship does instead of crawling along at a fixed rate
// regardless of speed.
uniform vec3 travel = vec3(0.0);

// Converts metres travelled into noise-space offset. Small because the noise
// features are around one unit across, so even a few units of drift is enough
// to move the clouds noticeably.
const float TRAVEL_TO_DRIFT = 0.00008;

float hash13(vec3 p) {
	p = fract(p * 0.1031);
	p += dot(p, p.zyx + 31.32);
	return fract((p.x + p.y) * p.z);
}

float value_noise(vec3 p) {
	vec3 i = floor(p);
	vec3 f = fract(p);
	f = f * f * (3.0 - 2.0 * f);

	float n000 = hash13(i + vec3(0.0, 0.0, 0.0));
	float n100 = hash13(i + vec3(1.0, 0.0, 0.0));
	float n010 = hash13(i + vec3(0.0, 1.0, 0.0));
	float n110 = hash13(i + vec3(1.0, 1.0, 0.0));
	float n001 = hash13(i + vec3(0.0, 0.0, 1.0));
	float n101 = hash13(i + vec3(1.0, 0.0, 1.0));
	float n011 = hash13(i + vec3(0.0, 1.0, 1.0));
	float n111 = hash13(i + vec3(1.0, 1.0, 1.0));

	float nx00 = mix(n000, n100, f.x);
	float nx10 = mix(n010, n110, f.x);
	float nx01 = mix(n001, n101, f.x);
	float nx11 = mix(n011, n111, f.x);
	float nxy0 = mix(nx00, nx10, f.y);
	float nxy1 = mix(nx01, nx11, f.y);
	return mix(nxy0, nxy1, f.z);
}

float fbm(vec3 p) {
	float v = 0.0;
	float a = 0.5;
	for (int i = 0; i < 4; i++) {
		v += a * value_noise(p);
		p = p * 2.03 + vec3(11.3);
		a *= 0.55;
	}
	return v;
}

void sky() {
	vec3 dir = normalize(EYEDIR);
	vec3 drift = travel * TRAVEL_TO_DRIFT;

	// Large-scale mask: leaves big empty voids so coverage is never 100%.
	float mask = fbm(dir * 1.15 + vec3(4.7) + drift * 0.5);
	mask = smoothstep(0.45, 0.8, mask);

	vec3 p = dir * nebula_scale + drift;
	float base = fbm(p);
	float detail = fbm(p * 2.6 + vec3(13.7));

	float clouds = base * 0.6 + detail * 0.4;
	float neb = smoothstep(1.0 - nebula_coverage, 1.0, clouds) * mask;

	float wisps = fbm(p * 4.5 + vec3(31.0));
	wisps = smoothstep(0.55, 0.95, wisps) * neb;

	vec3 neb_col = mix(nebula_color_a.rgb, nebula_color_b.rgb, smoothstep(0.25, 0.8, detail));
	neb_col = mix(neb_col, nebula_color_c.rgb, smoothstep(0.68, 0.95, base * detail * 2.0) * 0.5);
	float region = fbm(dir * 0.9 + vec3(9.2));
	neb_col = mix(neb_col, nebula_color_b.rgb * 0.6 + nebula_color_a.rgb * 0.4, region * 0.35);

	vec3 col = space_color.rgb;
	col += neb_col * neb * nebula_intensity;
	col += neb_col * wisps * nebula_intensity * 0.5;

	COLOR = col;
}
