// node compile.js <dir>: GLSL → SPIR-V → scene_spv.h, with glslang (@webgpu/glslang).
const fs = require('fs'), path = require('path');
const factory = require('@webgpu/glslang/dist/node-devel/glslang.js');
// Emscripten's module is a thenable that resolves to itself: take it once.
const glslang = new Promise(r => factory().then(m => { delete m.then; r(m); }));
const dir = process.argv[2];
let out = '// Code generated from shaders/*.vert and *.frag by glslang (node shaders/compile.js .); DO NOT EDIT.\n\n#include <stdint.h>\n\n';
(async () => {
  const g = await glslang;
  for (const [file, stage, name] of [['scene.vert', 'vertex', 'scene_vert'], ['scene.frag', 'fragment', 'scene_frag'], ['shadow.vert', 'vertex', 'shadow_vert']]) {
    const src = fs.readFileSync(path.join(dir, 'shaders', file), 'utf8');
    const words = g.compileGLSL(src, stage, false);
    out += `static const uint32_t ${name}[] = {\n`;
    for (let i = 0; i < words.length; i += 8) out += '    ' + Array.from(words.slice(i, i + 8)).map(w => '0x' + w.toString(16).padStart(8, '0')).join(', ') + ',\n';
    out += '};\n\n';
  }
  fs.writeFileSync(path.join(dir, 'scene_spv.h'), out);
})().catch(e => { console.error(e); process.exit(1); });
