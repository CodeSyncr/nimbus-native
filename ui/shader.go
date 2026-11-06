package ui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Shader is a small GPU program that colors a shape, pixel by pixel: noise,
// ripples, glows, grain. Write it once, in AGSL (Android's shading language,
// close to GLSL); each phone runs it in its own: AGSL on Android 13+, Metal
// on iOS, WebGL on the web.
type Shader struct {
	src      string
	floats   int               // how many numbers its uniforms take
	byTarget map[string]string // the source each platform runs
}

// NewShader reads a shader: uniform floats (float, float2, float3, float4
// and their half and vec spellings) then functions, one of them
//
//	half4 main(float2 p)
//
// which gets the point it colors, in the canvas's units, and returns its
// color, premultiplied (rgb × a). It panics on a shader it can't read, so
// make shaders package variables.
//
//nimbus:param src the shader's source
func NewShader(src string) *Shader {
	s, err := parseShader(src)
	if err != nil {
		panic("ui.NewShader: " + err.Error())
	}
	shaders.Store(s.byTarget["android"], s)
	return s
}

// shaders finds a shader by its Android source, which a Draw carries until
// the runtime swaps in the platform's.
var shaders sync.Map

// FillShader fills the path with sh, giving its uniforms these numbers, in
// the order the shader declares them (a float2 takes two), then starts a
// new path. Where shaders can't run (Android 12 and older) it fills with
// fallback.
//
//nimbus:param sh the shader
//nimbus:param fallback the color where shaders can't run
//nimbus:param uniforms the uniforms' numbers, which can animate
func (p *Painter) FillShader(sh *Shader, fallback Color, uniforms ...Num) {
	if sh == nil {
		return
	}
	if len(uniforms) != sh.floats {
		panic(fmt.Sprintf("ui.Painter.FillShader: the shader takes %d numbers, not %d", sh.floats, len(uniforms)))
	}
	for i, u := range uniforms {
		p.bind(5+i, u)
	}
	x0, y0, x1, y1 := p.bounds()
	p.add(render.DrawFillShader, x0, y0, x1-x0, y1-y0, float32(len(uniforms)))
	for _, u := range uniforms {
		p.nums = append(p.nums, u.from)
	}
	p.colors = append(p.colors, uint32(fallback))
	p.strs = append(p.strs, sh.byTarget["android"])
	p.painted()
}

// shadersFor swaps the shaders in a Draw's strings for the platform's own.
func shadersFor(strs []string, platform string) []string {
	var out []string
	for i, s := range strs {
		v, ok := shaders.Load(s)
		if !ok {
			continue
		}
		t := v.(*Shader).byTarget[platform]
		if t == "" || t == s {
			continue
		}
		if out == nil {
			out = append([]string(nil), strs...)
		}
		out[i] = t
	}
	if out == nil {
		return strs
	}
	return out
}

// ---- reading and rewriting ----

type tok struct {
	s     string
	ident bool
}

func lexShader(src string) ([]tok, error) {
	var out []tok
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case strings.HasPrefix(src[i:], "/*"):
			j := strings.Index(src[i+2:], "*/")
			if j < 0 {
				return nil, fmt.Errorf("a comment that doesn't end")
			}
			i += j + 4
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			j := i
			for j < len(src) && (src[j] == '_' || src[j] >= 'a' && src[j] <= 'z' || src[j] >= 'A' && src[j] <= 'Z' || src[j] >= '0' && src[j] <= '9') {
				j++
			}
			out = append(out, tok{src[i:j], true})
			i = j
		case c >= '0' && c <= '9' || c == '.' && i+1 < len(src) && src[i+1] >= '0' && src[i+1] <= '9':
			j := i
			for j < len(src) && (src[j] >= '0' && src[j] <= '9' || src[j] == '.' || src[j] == 'e' || src[j] == 'E' || src[j] == 'f' ||
				(src[j] == '-' || src[j] == '+') && (src[j-1] == 'e' || src[j-1] == 'E')) {
				j++
			}
			out = append(out, tok{strings.TrimSuffix(src[i:j], "f"), false})
			i = j
		case c == '#':
			return nil, fmt.Errorf("no preprocessor (#)")
		default:
			// Two-character operators stay together.
			if i+1 < len(src) && twoChar[src[i:i+2]] {
				out = append(out, tok{src[i : i+2], false})
				i += 2
				continue
			}
			out = append(out, tok{string(c), false})
			i++
		}
	}
	return out, nil
}

var twoChar = map[string]bool{"==": true, "!=": true, "<=": true, ">=": true, "&&": true, "||": true, "++": true, "--": true,
	"+=": true, "-=": true, "*=": true, "/=": true}

// widths are the uniform types and how many floats each takes.
var widths = map[string]int{"float": 1, "half": 1, "float2": 2, "half2": 2, "vec2": 2, "float3": 3, "half3": 3, "vec3": 3,
	"float4": 4, "half4": 4, "vec4": 4}

// Type names in each target.
var metalTypes = map[string]string{"vec2": "float2", "vec3": "float3", "vec4": "float4", "mat2": "float2x2", "mat3": "float3x3",
	"mat4": "float4x4", "ivec2": "int2", "ivec3": "int3", "ivec4": "int4", "bvec2": "bool2", "bvec3": "bool3", "bvec4": "bool4",
	"inversesqrt": "rsqrt", "mod": "nn_mod", "radians": "nn_radians", "degrees": "nn_degrees"}
var glslTypes = map[string]string{"half": "float", "float2": "vec2", "half2": "vec2", "float3": "vec3", "half3": "vec3",
	"float4": "vec4", "half4": "vec4", "float2x2": "mat2", "float3x3": "mat3", "float4x4": "mat4", "half2x2": "mat2",
	"half3x3": "mat3", "half4x4": "mat4", "int2": "ivec2", "int3": "ivec3", "int4": "ivec4", "bool2": "bvec2", "bool3": "bvec3",
	"bool4": "bvec4", "saturate": "nn_saturate"}

func parseShader(src string) (*Shader, error) {
	toks, err := lexShader(src)
	if err != nil {
		return nil, err
	}
	// The uniforms, taken out: their names become slots of one array.
	type uniform struct {
		at, n int
	}
	uniforms := map[string]uniform{}
	floats := 0
	var body []tok
	for i := 0; i < len(toks); i++ {
		if toks[i].s != "uniform" {
			body = append(body, toks[i])
			continue
		}
		if i+3 >= len(toks) || toks[i+3].s != ";" {
			return nil, fmt.Errorf("uniforms are written \"uniform float2 size;\"")
		}
		n, ok := widths[toks[i+1].s]
		if !ok {
			return nil, fmt.Errorf("uniform %s: only float, float2, float3 and float4 uniforms", toks[i+2].s)
		}
		uniforms[toks[i+2].s] = uniform{floats, n}
		floats += n
		i += 3
	}
	for _, t := range body {
		switch t.s {
		case "in", "out", "inout":
			return nil, fmt.Errorf("no %s parameters", t.s)
		case "shader", "eval", "sampler2D", "texture":
			return nil, fmt.Errorf("no %s: shaders color by numbers alone", t.s)
		}
	}
	// The functions: at the top level, a type, a name and "(".
	funcs := map[string]bool{}
	depth := 0
	mainAt := -1
	for i, t := range body {
		switch t.s {
		case "{":
			depth++
		case "}":
			depth--
		}
		if depth == 0 && t.ident && i+2 < len(body) && body[i+1].ident && body[i+2].s == "(" {
			funcs[body[i+1].s] = true
			if body[i+1].s == "main" {
				mainAt = i
			}
		}
	}
	if mainAt < 0 || !(body[mainAt].s == "half4" || body[mainAt].s == "float4" || body[mainAt].s == "vec4") ||
		mainAt+5 >= len(body) || !(body[mainAt+3].s == "float2" || body[mainAt+3].s == "vec2") || body[mainAt+5].s != ")" {
		return nil, fmt.Errorf("no \"half4 main(float2 p)\"")
	}
	s := &Shader{src: src, floats: floats, byTarget: map[string]string{}}
	for _, target := range []string{"android", "ios", "web"} {
		s.byTarget[target] = emitShader(target, body, funcs, func(name string) (string, bool) {
			u, ok := uniforms[name]
			if !ok {
				return "", false
			}
			parts := make([]string, u.n)
			for k := range parts {
				parts[k] = "nn_u[" + strconv.Itoa(u.at+k) + "]"
			}
			if u.n == 1 {
				return parts[0], true
			}
			vec := map[string]string{"android": "float", "ios": "float", "web": "vec"}[target] + strconv.Itoa(u.n)
			return vec + "(" + strings.Join(parts, ", ") + ")", true
		}, floats)
	}
	return s, nil
}

// emitShader writes the shader for a target: uniforms read from nn_u, main
// renamed nn_main, the target's type names and helpers, and (Metal, which
// has no global uniforms) nn_u passed to every function.
func emitShader(target string, body []tok, funcs map[string]bool, uniform func(string) (string, bool), floats int) string {
	var b strings.Builder
	switch target {
	case "android":
		if floats > 0 {
			fmt.Fprintf(&b, "uniform float nn_u[%d];\n", floats)
		}
	case "web":
		b.WriteString("#version 300 es\nprecision highp float;\n")
		fmt.Fprintf(&b, "uniform float nn_u[%d];\nuniform vec4 nn_box;\nin vec2 nn_uv;\nout vec4 nn_color;\n", max(floats, 1))
		b.WriteString("float nn_saturate(float x) { return clamp(x, 0.0, 1.0); }\n" +
			"vec2 nn_saturate(vec2 x) { return clamp(x, 0.0, 1.0); }\n" +
			"vec3 nn_saturate(vec3 x) { return clamp(x, 0.0, 1.0); }\n" +
			"vec4 nn_saturate(vec4 x) { return clamp(x, 0.0, 1.0); }\n")
	case "ios":
		b.WriteString("#include <metal_stdlib>\nusing namespace metal;\n")
		for _, t := range []string{"float", "float2", "float3", "float4"} {
			fmt.Fprintf(&b, "%[1]s nn_mod(%[1]s x, %[1]s y) { return x - y * floor(x / y); }\n", t)
			if t != "float" {
				fmt.Fprintf(&b, "%[1]s nn_mod(%[1]s x, float y) { return x - y * floor(x / y); }\n", t)
			}
			fmt.Fprintf(&b, "%[1]s nn_radians(%[1]s d) { return d * 0.017453292519943295; }\n", t)
			fmt.Fprintf(&b, "%[1]s nn_degrees(%[1]s r) { return r * 57.29577951308232; }\n", t)
		}
	}
	types := map[string]map[string]string{"ios": metalTypes, "web": glslTypes}[target]
	var out []string
	depth := 0
	for i := 0; i < len(body); i++ {
		t := body[i]
		switch t.s {
		case "{":
			depth++
		case "}":
			depth--
		}
		s := t.s
		if t.ident {
			if v, ok := uniform(s); ok {
				out = append(out, v)
				continue
			}
			if s == "main" && depth == 0 {
				s = "nn_main"
			}
			if r, ok := types[s]; ok {
				s = r
			}
			// Metal: atan(y, x) is atan2.
			if target == "ios" && s == "atan" && commas(body, i+1) == 1 {
				s = "atan2"
			}
		}
		out = append(out, s)
		// Metal: every function takes nn_u, and every call passes it.
		if target == "ios" && t.ident && funcs[t.s] && i+1 < len(body) && body[i+1].s == "(" {
			def := depth == 0 && i > 0 && body[i-1].ident
			end := closing(body, i+1)
			out = append(out, "(")
			if def {
				if end == i+2 {
					out = append(out, "constant float *nn_u")
				} else {
					out = append(out, "constant float *nn_u", ",")
				}
			} else {
				if end == i+2 {
					out = append(out, "nn_u")
				} else {
					out = append(out, "nn_u", ",")
				}
			}
			i++ // past "(": the arguments follow as usual
		}
	}
	b.WriteString(joinTokens(out))
	b.WriteString("\n")
	switch target {
	case "android":
		b.WriteString("half4 main(float2 p) { return nn_main(p); }\n")
	case "web":
		b.WriteString("void main() { nn_color = vec4(nn_main(nn_box.xy + nn_uv * nn_box.zw)); }\n")
	case "ios":
		b.WriteString(`struct NNOut { float4 pos [[position]]; float2 uv; };
vertex NNOut nn_vertex(uint v [[vertex_id]]) {
	float2 c = float2((v << 1) & 2, v & 2);
	NNOut o;
	o.pos = float4(c * 2.0 - 1.0, 0.0, 1.0);
	o.uv = float2(c.x, 1.0 - c.y);
	return o;
}
fragment float4 nn_fragment(NNOut in [[stage_in]], constant float *nn_u [[buffer(0)]], constant float4 &nn_box [[buffer(1)]]) {
	return float4(nn_main(nn_u, nn_box.xy + in.uv * nn_box.zw));
}
`)
	}
	return b.String()
}

// closing is the index of the ")" matching the "(" at i.
func closing(toks []tok, i int) int {
	depth := 0
	for j := i; j < len(toks); j++ {
		switch toks[j].s {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return len(toks) - 1
}

// commas counts the top-level commas between the "(" at i and its ")".
func commas(toks []tok, i int) int {
	if i >= len(toks) || toks[i].s != "(" {
		return -1
	}
	n, depth := 0, 0
	for j := i; j < len(toks); j++ {
		switch toks[j].s {
		case "(", "[":
			depth++
		case ")", "]":
			depth--
			if depth == 0 {
				return n
			}
		case ",":
			if depth == 1 {
				n++
			}
		}
	}
	return n
}

func joinTokens(ts []string) string {
	var b strings.Builder
	for i, t := range ts {
		if i > 0 {
			prev := ts[i-1]
			switch {
			case word(prev) && word(t[:1]), t == "{", prev == "{", prev == ";", prev == "}", prev == ",":
				b.WriteByte(' ')
			}
		}
		b.WriteString(t)
	}
	return b.String()
}

// word is a token that would run into the next one: a name or a number.
func word(s string) bool {
	if s == "" {
		return false
	}
	c := s[len(s)-1]
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
