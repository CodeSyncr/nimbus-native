package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

const waveShader = `
uniform float time;
uniform float2 size;
// A wave, as a helper.
float wave(float2 uv, float k) {
	return 0.5 + 0.5 * sin(k * uv.x + time);
}
half4 main(float2 p) {
	float2 uv = p / size;
	float a = atan(uv.y - 0.5, uv.x - 0.5);
	float r = mod(a + time, 1.0);
	return half4(saturate(half3(uv.x, wave(uv, 12.0), r)), 1.0);
}`

// Each platform's source reads uniforms from one array, renames main, and
// speaks its own language; these sources also compiled with Metal's
// compiler and in Chromium's WebGL 2.
func TestShaderTranslation(t *testing.T) {
	s := NewShader(waveShader)
	if s.floats != 3 {
		t.Fatalf("%d uniform floats", s.floats)
	}
	for target, want := range map[string][]string{
		"android": {"uniform float nn_u[3];", "half4 nn_main(float2 p)", "p/float2(nn_u[1], nn_u[2])", "sin(k*uv.x+nn_u[0])", "mod(a+nn_u[0], 1.0)",
			"half4 main(float2 p) { return nn_main(p); }"},
		"web": {"#version 300 es", "uniform float nn_u[3];", "vec4 nn_main(vec2 p)", "p/vec2(nn_u[1], nn_u[2])",
			"nn_saturate(vec3(", "nn_color = vec4(nn_main("},
		"ios": {"#include <metal_stdlib>", "float wave(constant float *nn_u, float2 uv, float k)",
			"half4 nn_main(constant float *nn_u, float2 p)", "wave(nn_u, uv, 12.0)", "atan2(uv.y-0.5, uv.x-0.5)",
			"nn_mod(a+nn_u[0], 1.0)", "fragment float4 nn_fragment"},
	} {
		got := s.byTarget[target]
		for _, w := range want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: no %q in\n%s", target, w, got)
			}
		}
		if strings.Contains(got, "uniform float time") || strings.Contains(got, "half4 main(float2 p) {\n") {
			t.Errorf("%s: kept the original uniforms or main:\n%s", target, got)
		}
	}
}

func TestShaderRefused(t *testing.T) {
	for _, src := range []string{
		`half4 nope(float2 p) { return half4(1); }`,
		`uniform shader img; half4 main(float2 p) { return img.eval(p); }`,
		`uniform int n; half4 main(float2 p) { return half4(1); }`,
		`void f(inout float x) {} half4 main(float2 p) { return half4(1); }`,
		`#define K 2.0
half4 main(float2 p) { return half4(K); }`,
		`half4 main(float2 p) { /* never ends`,
	} {
		if _, err := parseShader(src); err == nil {
			t.Errorf("read %q", src)
		}
	}
}

// FillShader sends the path's box, its uniforms (which can animate) and
// the fallback; the runtime swaps in the platform's source.
func TestFillShader(t *testing.T) {
	s := NewShader(waveShader)
	p := &Painter{}
	p.Fill(1) // an earlier path's bounds don't count
	p.Rect(10, 20, 100, 50, 8)
	p.Circle(120, 30, 20)
	p.FillShader(s, Hex("#336699"), Fixed(0), Fixed(100), Fixed(50))
	if !slices.Equal(p.cmds, []uint8{render.DrawFill, render.DrawRect, render.DrawCircle, render.DrawFillShader}) {
		t.Fatalf("cmds %v", p.cmds)
	}
	if got := p.nums[len(p.nums)-8:]; !slices.Equal(got, []float32{10, 10, 130, 60, 3, 0, 100, 50}) {
		t.Fatalf("box and uniforms %v", got)
	}
	if p.strs[0] != s.byTarget["android"] || p.colors[1] != uint32(Hex("#336699")) {
		t.Fatal("source or fallback")
	}
	for _, platform := range []string{"ios", "web", "android"} {
		if got := shadersFor(p.strs, platform); got[0] != s.byTarget[platform] {
			t.Errorf("%s got another source", platform)
		}
	}
	if got := shadersFor([]string{"https://x/y.png"}, "ios"); got[0] != "https://x/y.png" {
		t.Fatal("a picture's URL was swapped")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("the wrong number of uniforms was taken")
		}
	}()
	p.FillShader(s, 0, Fixed(1))
}
