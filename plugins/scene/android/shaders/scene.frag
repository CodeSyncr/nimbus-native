#version 450
// The scene's fragment shader: the same light as the Metal one on iOS.
layout(location = 0) in vec3 world;
layout(location = 1) in vec3 normal;
layout(set = 0, binding = 0) uniform Scene { mat4 vp; vec4 eye; vec4 ambient; vec4 dirs[4]; vec4 colors[4]; ivec4 count; } s;
layout(push_constant) uniform Node { mat4 model; vec4 color; vec4 params; } n;
layout(location = 0) out vec4 outColor;
void main() {
    if (n.params.y > 0.5) { outColor = n.color; return; }
    vec3 base = n.color.rgb;
    vec3 N = normalize(normal);
    vec3 V = normalize(s.eye.xyz - world);
    vec3 c = base * s.ambient.x;
    float shine = n.params.x;
    for (int i = 0; i < s.count.x; i++) {
        vec3 L = -s.dirs[i].xyz;
        float diff = max(dot(N, L), 0.0);
        vec3 H = normalize(L + V);
        float spec = shine > 0.0 ? pow(max(dot(N, H), 0.0), mix(8.0, 128.0, shine)) * shine : 0.0;
        c += (base * diff + spec) * s.colors[i].rgb;
    }
    outColor = vec4(c, n.color.a);
}
