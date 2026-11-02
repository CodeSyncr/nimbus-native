#version 450
// The scene's vertex shader. plugins/scene/android/spv.go compiles it into scene_spv.h.
layout(location = 0) in vec3 pos;
layout(location = 1) in vec3 nrm;
layout(set = 0, binding = 0) uniform Scene { mat4 vp; vec4 eye; vec4 ambient; vec4 dirs[4]; vec4 colors[4]; ivec4 count; } s;
layout(push_constant) uniform Node { mat4 model; vec4 color; vec4 params; } n;
layout(location = 0) out vec3 world;
layout(location = 1) out vec3 normal;
void main() {
    vec4 w = n.model * vec4(pos, 1.0);
    world = w.xyz;
    normal = transpose(inverse(mat3(n.model))) * nrm;
    gl_Position = s.vp * w;
}
