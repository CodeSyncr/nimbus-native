#version 450
// Depth only, from the first light: what's nearest it.
layout(location = 0) in vec3 pos;
layout(set = 0, binding = 0) uniform Scene { mat4 vp; mat4 lightVP; } s;
layout(push_constant) uniform Node { mat4 model; } n;
void main() { gl_Position = s.lightVP * n.model * vec4(pos, 1.0); }
