package scene

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	_ "image/jpeg" // for DecodeConfig
	_ "image/png"
	"sync"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// assets are the meshes and pictures scenes have used, by key, for when
// the phone asks for one it lacks. The phone keeps what it's sent, so a
// scene that renders again sends nothing new, and after a dev reload Go
// still has what it needs to send again.
var assets sync.Map

// Texture is a picture to wrap on a mesh: a PNG or a JPEG.
type Texture struct {
	data          []byte
	Width, Height int // in pixels
	key           string
	once          sync.Once
}

// MaxTexture is the largest side a Texture may have, in pixels: every GPU
// on a supported phone takes this.
const MaxTexture = 4096

// NewTexture reads a PNG or a JPEG. The phone decodes it, and makes the
// smaller copies that keep it crisp from far away.
//
//nimbus:param data the file's bytes
func NewTexture(data []byte) (*Texture, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("scene: texture: %w", err)
	}
	if format != "png" && format != "jpeg" {
		return nil, fmt.Errorf("scene: texture: %s; use PNG or JPEG", format)
	}
	if cfg.Width > MaxTexture || cfg.Height > MaxTexture || cfg.Width == 0 || cfg.Height == 0 {
		return nil, fmt.Errorf("scene: texture is %d×%d; keep each side from 1 to %d", cfg.Width, cfg.Height, MaxTexture)
	}
	return &Texture{data: data, Width: cfg.Width, Height: cfg.Height}, nil
}

func (t *Texture) register() string {
	t.once.Do(func() {
		h := fnv.New64a()
		h.Write(t.data)
		t.key = "t" + base64.RawURLEncoding.EncodeToString(h.Sum(nil))
		assets.Store(t.key, t)
	})
	return t.key
}

// need is the phone asking for what it lacks.
type need struct {
	Keys []string `json:"keys"`
}

// sending keeps two scenes asking at once from sending the same thing twice.
var sending sync.Map

// call reaches the phone; tests replace it.
var call = func(ctx context.Context, method string, args any) error {
	_, err := plugin.CallJSON(ctx, "scene", method, args)
	return err
}

func (n need) send(ctx context.Context) {
	for _, k := range n.Keys {
		// Each phone or tab gets its own copy, so busy is per ctx.
		busy := struct {
			to  context.Context
			key string
		}{ctx, k}
		if _, already := sending.LoadOrStore(busy, true); already {
			continue
		}
		err := sendAsset(ctx, k)
		sending.Delete(busy)
		if err != nil {
			fmt.Println("scene:", err) // the phone shows nothing for it, and asks again next time
		}
	}
}

func sendAsset(ctx context.Context, key string) error {
	a, ok := assets.Load(key)
	if !ok {
		return errors.New("the phone asked for " + key + ", which this app hasn't made")
	}
	switch a := a.(type) {
	case *Mesh:
		return call(ctx, "mesh", map[string]any{"key": key, "vertices": a.vertices(), "indices": a.indexBytes(), "count": len(a.indices)})
	case *Texture:
		return call(ctx, "texture", map[string]any{"key": key, "data": a.data})
	}
	return nil
}
