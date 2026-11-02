#version 450
// The scene's fragment shader: the same light as the Metal one on iOS, with
// the first light's shadow, softened by sampling around each point.
layout(location = 0) in vec3 world;
layout(location = 1) in vec3 normal;
layout(location = 2) in vec2 texUV;
layout(location = 3) in vec4 lightPos;
layout(set = 0, binding = 0) uniform Scene {
    mat4 vp; mat4 lightVP; vec4 eye; vec4 ambient;
    vec4 dirs[4]; vec4 colors[4]; ivec4 count;
} s;
layout(set = 0, binding = 1) uniform sampler2DShadow shadowMap;
layout(set = 1, binding = 0) uniform sampler2D tex;
layout(push_constant) uniform Node { mat4 model; vec4 color; vec4 params; } n;
layout(location = 0) out vec4 outColor;

float lit(vec3 N, vec3 L) {
    if (s.ambient.y < 0.5) return 1.0;
    vec3 p = lightPos.xyz / lightPos.w;
    vec2 at = p.xy * 0.5 + 0.5;
    if (at.x < 0.0 || at.x > 1.0 || at.y < 0.0 || at.y > 1.0 || p.z > 1.0) return 1.0;
    float bias = max(0.004 * (1.0 - dot(N, L)), 0.001);
    vec2 texel = 1.0 / vec2(textureSize(shadowMap, 0));
    float sum = 0.0;
    for (int x = -1; x <= 1; x++)
        for (int y = -1; y <= 1; y++)
            sum += texture(shadowMap, vec3(at + vec2(x, y) * texel, p.z - bias));
    return sum / 9.0;
}

void main() {
    vec4 base = texture(tex, texUV) * n.color;
    if (base.a < 0.02) discard;
    if (n.params.y > 0.5) { outColor = base; return; }
    vec3 N = normalize(normal);
    if (!gl_FrontFacing) N = -N; // a double-sided node's back
    vec3 V = normalize(s.eye.xyz - world);
    vec3 c = base.rgb * s.ambient.x;
    float shine = n.params.x;
    for (int i = 0; i < s.count.x; i++) {
        vec3 L = -s.dirs[i].xyz;
        float diff = max(dot(N, L), 0.0);
        vec3 H = normalize(L + V);
        float spec = shine > 0.0 ? pow(max(dot(N, H), 0.0), mix(8.0, 128.0, shine)) * shine : 0.0;
        float shade = i == 0 && diff > 0.0 ? lit(N, L) : 1.0;
        c += (base.rgb * diff + spec) * s.colors[i].rgb * shade;
    }
    outColor = vec4(c, base.a);
}
