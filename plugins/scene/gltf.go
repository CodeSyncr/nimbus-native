package scene

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// LoadGLTF reads a glTF 2.0 model: a .glb, or a .gltf with its buffers and
// pictures embedded or beside it. It gives a Node holding the file's
// scene: each glTF node becomes a Node with the same name as its ID, with
// its meshes, colors and textures, placed as the file says.
//
// files reads what a .gltf points to (a .bin, a .png) by its relative path;
// nil is fine for a .glb or a .gltf with everything embedded. Draco and
// meshopt compression aren't supported, and nor are skins, morph targets or
// animations: export without them.
//
//nimbus:param data the file's bytes
//nimbus:param files reads the files it points to, or nil
//nimbus:returns the model, ready for Fit
func LoadGLTF(data []byte, files func(uri string) ([]byte, error)) (Node, error) {
	g := &gltf{files: files}
	if err := g.parse(data); err != nil {
		return Node{}, fmt.Errorf("scene: glTF: %w", err)
	}
	n, err := g.build()
	if err != nil {
		return Node{}, fmt.Errorf("scene: glTF: %w", err)
	}
	return n, nil
}

type gltf struct {
	files    func(string) ([]byte, error)
	bin      []byte
	buffers  [][]byte
	textures map[int]*Texture
	doc      struct {
		Asset struct {
			Version string `json:"version"`
		} `json:"asset"`
		ExtensionsRequired []string `json:"extensionsRequired"`
		Scene              *int     `json:"scene"`
		Scenes             []struct {
			Nodes []int `json:"nodes"`
		} `json:"scenes"`
		Nodes []struct {
			Name        string    `json:"name"`
			Mesh        *int      `json:"mesh"`
			Children    []int     `json:"children"`
			Matrix      []float32 `json:"matrix"`
			Translation []float32 `json:"translation"`
			Rotation    []float32 `json:"rotation"`
			Scale       []float32 `json:"scale"`
		} `json:"nodes"`
		Meshes []struct {
			Primitives []struct {
				Attributes map[string]int `json:"attributes"`
				Indices    *int           `json:"indices"`
				Material   *int           `json:"material"`
				Mode       *int           `json:"mode"`
			} `json:"primitives"`
		} `json:"meshes"`
		Accessors []struct {
			BufferView    *int            `json:"bufferView"`
			ByteOffset    int             `json:"byteOffset"`
			ComponentType int             `json:"componentType"`
			Normalized    bool            `json:"normalized"`
			Count         int             `json:"count"`
			Type          string          `json:"type"`
			Sparse        json.RawMessage `json:"sparse"`
		} `json:"accessors"`
		BufferViews []struct {
			Buffer     int `json:"buffer"`
			ByteOffset int `json:"byteOffset"`
			ByteLength int `json:"byteLength"`
			ByteStride int `json:"byteStride"`
		} `json:"bufferViews"`
		Buffers []struct {
			URI        string `json:"uri"`
			ByteLength int    `json:"byteLength"`
		} `json:"buffers"`
		Materials []struct {
			PBR *struct {
				BaseColorFactor  []float32 `json:"baseColorFactor"`
				BaseColorTexture *struct {
					Index int `json:"index"`
				} `json:"baseColorTexture"`
				RoughnessFactor *float32 `json:"roughnessFactor"`
			} `json:"pbrMetallicRoughness"`
			DoubleSided bool                       `json:"doubleSided"`
			Extensions  map[string]json.RawMessage `json:"extensions"`
		} `json:"materials"`
		Textures []struct {
			Source *int `json:"source"`
		} `json:"textures"`
		Images []struct {
			URI        string `json:"uri"`
			BufferView *int   `json:"bufferView"`
		} `json:"images"`
	}
}

const (
	glbMagic  = 0x46546C67 // "glTF"
	chunkJSON = 0x4E4F534A
	chunkBIN  = 0x004E4942
)

func (g *gltf) parse(data []byte) error {
	js := data
	if len(data) >= 12 && binary.LittleEndian.Uint32(data) == glbMagic {
		if v := binary.LittleEndian.Uint32(data[4:]); v != 2 {
			return fmt.Errorf("a version %d .glb; only 2 is supported", v)
		}
		js = nil
		for at := 12; at+8 <= len(data); {
			n, kind := int(binary.LittleEndian.Uint32(data[at:])), binary.LittleEndian.Uint32(data[at+4:])
			if at+8+n > len(data) {
				return errors.New("the .glb is cut short")
			}
			chunk := data[at+8 : at+8+n]
			switch kind {
			case chunkJSON:
				js = chunk
			case chunkBIN:
				if g.bin == nil {
					g.bin = chunk
				}
			}
			at += 8 + (n+3)&^3
		}
		if js == nil {
			return errors.New("the .glb has no JSON")
		}
	}
	if err := json.Unmarshal(js, &g.doc); err != nil {
		return err
	}
	if !strings.HasPrefix(g.doc.Asset.Version, "2") {
		return fmt.Errorf("glTF %q; only 2.0 is supported", g.doc.Asset.Version)
	}
	for _, e := range g.doc.ExtensionsRequired {
		if e != "KHR_materials_unlit" && e != "KHR_texture_transform" {
			return fmt.Errorf("it needs %s, which isn't supported: export without it", e)
		}
	}
	for i, b := range g.doc.Buffers {
		var d []byte
		var err error
		switch {
		case b.URI == "" && i == 0 && g.bin != nil:
			d = g.bin
		case b.URI != "":
			d, err = g.uri(b.URI)
		default:
			err = fmt.Errorf("buffer %d has no data", i)
		}
		if err != nil {
			return err
		}
		if len(d) < b.ByteLength {
			return fmt.Errorf("buffer %d is shorter than it says", i)
		}
		g.buffers = append(g.buffers, d)
	}
	return nil
}

func (g *gltf) uri(u string) ([]byte, error) {
	if strings.HasPrefix(u, "data:") {
		i := strings.Index(u, ";base64,")
		if i < 0 {
			return nil, errors.New("a data URI that isn't base64")
		}
		return base64.StdEncoding.DecodeString(u[i+8:])
	}
	if g.files == nil {
		return nil, fmt.Errorf("it points to %q: pass LoadGLTF a files function, or use a .glb", u)
	}
	return g.files(u)
}

func (g *gltf) view(i int) ([]byte, int, error) {
	if i < 0 || i >= len(g.doc.BufferViews) {
		return nil, 0, fmt.Errorf("no buffer view %d", i)
	}
	v := g.doc.BufferViews[i]
	if v.Buffer < 0 || v.Buffer >= len(g.buffers) || v.ByteOffset+v.ByteLength > len(g.buffers[v.Buffer]) {
		return nil, 0, fmt.Errorf("buffer view %d is outside its buffer", i)
	}
	return g.buffers[v.Buffer][v.ByteOffset : v.ByteOffset+v.ByteLength], v.ByteStride, nil
}

var components = map[string]int{"SCALAR": 1, "VEC2": 2, "VEC3": 3, "VEC4": 4}

// floats reads an accessor of want components a value, as float32s.
func (g *gltf) floats(i, want int) ([]float32, error) {
	if i < 0 || i >= len(g.doc.Accessors) {
		return nil, fmt.Errorf("no accessor %d", i)
	}
	a := g.doc.Accessors[i]
	if len(a.Sparse) > 0 {
		return nil, errors.New("sparse accessors aren't supported")
	}
	n := components[a.Type]
	if n != want {
		return nil, fmt.Errorf("accessor %d is %s", i, a.Type)
	}
	size := map[int]int{5120: 1, 5121: 1, 5122: 2, 5123: 2, 5125: 4, 5126: 4}[a.ComponentType]
	if size == 0 {
		return nil, fmt.Errorf("accessor %d: component type %d", i, a.ComponentType)
	}
	out := make([]float32, a.Count*n)
	if a.BufferView == nil {
		return out, nil // all zeros, as the spec says
	}
	b, stride, err := g.view(*a.BufferView)
	if err != nil {
		return nil, err
	}
	if stride == 0 {
		stride = size * n
	}
	if a.Count > 0 && a.ByteOffset+(a.Count-1)*stride+size*n > len(b) {
		return nil, fmt.Errorf("accessor %d runs past its buffer view", i)
	}
	for k := 0; k < a.Count; k++ {
		for c := 0; c < n; c++ {
			at := a.ByteOffset + k*stride + c*size
			var f float32
			switch a.ComponentType {
			case 5126:
				f = math.Float32frombits(binary.LittleEndian.Uint32(b[at:]))
			case 5121:
				f = float32(b[at]) / 255
			case 5123:
				f = float32(binary.LittleEndian.Uint16(b[at:])) / 65535
			case 5120:
				f = max(float32(int8(b[at]))/127, -1)
			case 5122:
				f = max(float32(int16(binary.LittleEndian.Uint16(b[at:])))/32767, -1)
			case 5125:
				f = float32(binary.LittleEndian.Uint32(b[at:]))
			}
			out[k*n+c] = f
		}
	}
	return out, nil
}

func (g *gltf) indices(i int) ([]uint32, error) {
	if i < 0 || i >= len(g.doc.Accessors) {
		return nil, fmt.Errorf("no accessor %d", i)
	}
	a := g.doc.Accessors[i]
	if a.BufferView == nil || a.Type != "SCALAR" {
		return nil, fmt.Errorf("accessor %d isn't indices", i)
	}
	b, _, err := g.view(*a.BufferView)
	if err != nil {
		return nil, err
	}
	size := map[int]int{5121: 1, 5123: 2, 5125: 4}[a.ComponentType]
	if size == 0 || a.ByteOffset+a.Count*size > len(b) {
		return nil, fmt.Errorf("accessor %d: bad indices", i)
	}
	out := make([]uint32, a.Count)
	for k := range out {
		at := a.ByteOffset + k*size
		switch size {
		case 1:
			out[k] = uint32(b[at])
		case 2:
			out[k] = uint32(binary.LittleEndian.Uint16(b[at:]))
		default:
			out[k] = binary.LittleEndian.Uint32(b[at:])
		}
	}
	return out, nil
}

func vecs(f []float32) []Vec3 {
	out := make([]Vec3, len(f)/3)
	for i := range out {
		out[i] = Vec3{f[i*3], f[i*3+1], f[i*3+2]}
	}
	return out
}

func (g *gltf) texture(i int) (*Texture, error) {
	if t, ok := g.textures[i]; ok {
		return t, nil
	}
	if i < 0 || i >= len(g.doc.Textures) || g.doc.Textures[i].Source == nil {
		return nil, nil // a texture with no picture we can read: go without
	}
	src := *g.doc.Textures[i].Source
	if src < 0 || src >= len(g.doc.Images) {
		return nil, fmt.Errorf("no image %d", src)
	}
	img := g.doc.Images[src]
	var data []byte
	var err error
	if img.BufferView != nil {
		data, _, err = g.view(*img.BufferView)
	} else {
		data, err = g.uri(img.URI)
	}
	if err != nil {
		return nil, err
	}
	t, err := NewTexture(data)
	if err != nil {
		return nil, err
	}
	if g.textures == nil {
		g.textures = map[int]*Texture{}
	}
	g.textures[i] = t
	return t, nil
}

// srgb turns glTF's linear color factors into the display values Color holds.
func srgb(f []float32) ui.Color {
	if len(f) < 4 {
		return ui.Hex("#ffffff")
	}
	b := func(x float32, gamma bool) uint32 {
		v := float64(clamp01(x))
		if gamma {
			v = math.Pow(v, 1/2.2)
		}
		return uint32(math.Round(v * 255))
	}
	return ui.Color(b(f[0], true)<<24 | b(f[1], true)<<16 | b(f[2], true)<<8 | b(f[3], false))
}

// primitive makes a Node for one of a mesh's parts.
func (g *gltf) primitive(id string, mesh, p int) (Node, bool, error) {
	pr := g.doc.Meshes[mesh].Primitives[p]
	if pr.Mode != nil && *pr.Mode != 4 {
		return Node{}, false, nil // points and lines: nothing to draw here
	}
	pi, ok := pr.Attributes["POSITION"]
	if !ok {
		return Node{}, false, nil
	}
	pf, err := g.floats(pi, 3)
	if err != nil {
		return Node{}, false, err
	}
	pos := vecs(pf)
	var nrm []Vec3
	if i, ok := pr.Attributes["NORMAL"]; ok {
		f, err := g.floats(i, 3)
		if err != nil {
			return Node{}, false, err
		}
		nrm = vecs(f)
	}
	var uvs []UV
	if i, ok := pr.Attributes["TEXCOORD_0"]; ok {
		f, err := g.floats(i, 2)
		if err != nil {
			return Node{}, false, err
		}
		uvs = make([]UV, len(f)/2)
		for k := range uvs {
			uvs[k] = UV{f[k*2], f[k*2+1]}
		}
	}
	var idx []uint32
	if pr.Indices != nil {
		if idx, err = g.indices(*pr.Indices); err != nil {
			return Node{}, false, err
		}
	} else {
		idx = make([]uint32, len(pos))
		for k := range idx {
			idx[k] = uint32(k)
		}
	}
	for _, k := range idx {
		if int(k) >= len(pos) {
			return Node{}, false, fmt.Errorf("mesh %d: an index past its vertices", mesh)
		}
	}
	n := Node{ID: id, Mesh: NewTexturedMesh(pos, nrm, uvs, idx), Color: ui.Hex("#ffffff")}
	if pr.Material != nil && *pr.Material >= 0 && *pr.Material < len(g.doc.Materials) {
		m := g.doc.Materials[*pr.Material]
		n.DoubleSided = m.DoubleSided
		_, n.Unlit = m.Extensions["KHR_materials_unlit"]
		rough := float32(1)
		if m.PBR != nil {
			if m.PBR.BaseColorFactor != nil {
				n.Color = srgb(m.PBR.BaseColorFactor)
			}
			if m.PBR.RoughnessFactor != nil {
				rough = *m.PBR.RoughnessFactor
			}
			if m.PBR.BaseColorTexture != nil {
				if n.Texture, err = g.texture(m.PBR.BaseColorTexture.Index); err != nil {
					return Node{}, false, err
				}
			}
		}
		n.Shine = clamp01(1-rough) * 0.8
	}
	return n, true, nil
}

func (g *gltf) build() (Node, error) {
	var roots []int
	switch {
	case g.doc.Scene != nil && *g.doc.Scene >= 0 && *g.doc.Scene < len(g.doc.Scenes):
		roots = g.doc.Scenes[*g.doc.Scene].Nodes
	case len(g.doc.Scenes) > 0:
		roots = g.doc.Scenes[0].Nodes
	default: // no scenes: every node no other has as a child
		child := map[int]bool{}
		for _, n := range g.doc.Nodes {
			for _, c := range n.Children {
				child[c] = true
			}
		}
		for i := range g.doc.Nodes {
			if !child[i] {
				roots = append(roots, i)
			}
		}
	}
	seen := map[int]bool{}
	var node func(i int) (Node, error)
	node = func(i int) (Node, error) {
		if i < 0 || i >= len(g.doc.Nodes) {
			return Node{}, fmt.Errorf("no node %d", i)
		}
		if seen[i] {
			return Node{}, fmt.Errorf("node %d is its own ancestor", i)
		}
		seen[i] = true
		defer delete(seen, i)
		d := g.doc.Nodes[i]
		id := d.Name
		if id == "" {
			id = fmt.Sprintf("node%d", i)
		}
		m := identity()
		switch {
		case len(d.Matrix) == 16:
			copy(m[:], d.Matrix)
		default:
			if len(d.Translation) == 3 {
				m = translate(V(d.Translation[0], d.Translation[1], d.Translation[2]))
			}
			if len(d.Rotation) == 4 {
				m = mul(m, quat(d.Rotation[0], d.Rotation[1], d.Rotation[2], d.Rotation[3]))
			}
			if len(d.Scale) == 3 {
				m = mul(m, scaling(V(d.Scale[0], d.Scale[1], d.Scale[2])))
			}
		}
		out := Node{ID: id, Matrix: &m}
		if d.Mesh != nil {
			if *d.Mesh < 0 || *d.Mesh >= len(g.doc.Meshes) {
				return Node{}, fmt.Errorf("no mesh %d", *d.Mesh)
			}
			for p := range g.doc.Meshes[*d.Mesh].Primitives {
				k, ok, err := g.primitive(id, *d.Mesh, p)
				if err != nil {
					return Node{}, err
				}
				if ok {
					out.Children = append(out.Children, k)
				}
			}
		}
		for _, c := range d.Children {
			k, err := node(c)
			if err != nil {
				return Node{}, err
			}
			out.Children = append(out.Children, k)
		}
		return out, nil
	}
	model := Node{}
	for _, r := range roots {
		k, err := node(r)
		if err != nil {
			return Node{}, err
		}
		model.Children = append(model.Children, k)
	}
	return model, nil
}
