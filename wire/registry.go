package wire

import (
	"encoding/binary"
	"hash/fnv"
	"slices"
	"sync"
)

var (
	regMu   sync.Mutex
	schemas = map[uint64]string{} // hash → package name, for messages
)

// Register records a schema linked into the app. Generated code calls it
// from init; apps never do.
func Register(pkg string, hash uint64) {
	regMu.Lock()
	defer regMu.Unlock()
	schemas[hash] = pkg
}

// Combined is the hash of every registered schema, the value a host must
// pass to nn_start. It is 0 when no schema is linked.
func Combined() uint64 {
	regMu.Lock()
	hashes := make([]uint64, 0, len(schemas))
	for h := range schemas {
		hashes = append(hashes, h)
	}
	regMu.Unlock()
	return Combine(hashes)
}

// Combine folds schema hashes into one value; order does not matter and no
// hashes is 0. The generators bake the same value into host code.
func Combine(hashes []uint64) uint64 {
	if len(hashes) == 0 {
		return 0
	}
	sorted := slices.Clone(hashes)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)
	h := fnv.New64a()
	for _, v := range sorted {
		h.Write(binary.LittleEndian.AppendUint64(nil, v))
	}
	return h.Sum64()
}
