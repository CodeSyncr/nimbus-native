package gen_test

import (
	"fmt"
	"testing"

	"github.com/CodeSyncr/nimbus-native/wire/internal/kitchen"
)

// A 1,000-node tree, roughly one screen of UI ops.
func tree() *kitchen.Node {
	root := &kitchen.Node{ID: 1, Label: "root"}
	for i := 0; i < 1000; i++ {
		root.Children = append(root.Children, &kitchen.Node{
			ID: uint32(i + 2), Kind: kitchen.Kind(i % 3), Label: fmt.Sprintf("Order #%d", 10000+i),
			Props: map[string]string{"color": "blue", "size": "14"},
		})
	}
	return root
}

func BenchmarkEncodeTree(b *testing.B) {
	t := tree()
	buf := kitchen.AppendMessage(nil, t)
	b.SetBytes(int64(len(buf)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf = kitchen.AppendMessage(buf[:0], t)
	}
}

func BenchmarkDecodeTree(b *testing.B) {
	buf := kitchen.Encode(tree())
	b.SetBytes(int64(len(buf)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := kitchen.Decode(buf); err != nil {
			b.Fatal(err)
		}
	}
}
