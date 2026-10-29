package gen

import (
	"fmt"

	"github.com/CodeSyncr/nimbus-native/wire/schema"
)

// Output is everything generated for one schema package.
type Output struct {
	Schema *schema.Schema
	Go     []byte // write to <schema dir>/wire_gen.go
	Swift  []byte // compile into the iOS app (imports NimbusNativeKit when it is a package)
	Kotlin []byte // compile into the Android host, as <Namespace>.kt
}

// Generate loads the schema in dir and generates all three languages; the
// Kotlin goes in kotlinPackage.
func Generate(dir, kotlinPackage string) (*Output, error) {
	s, err := schema.Load(dir)
	if err != nil {
		return nil, err
	}
	ns := namespace(s)
	for _, name := range typeNames(s) {
		if name == ns {
			return nil, fmt.Errorf("%s: type %s has the same name as the generated namespace; rename it or the package", dir, name)
		}
	}
	goSrc, err := Go(s)
	if err != nil {
		return nil, err
	}
	return &Output{Schema: s, Go: goSrc, Swift: SwiftApp(s), Kotlin: Kotlin(s, kotlinPackage)}, nil
}

// Namespace is the Swift enum / Kotlin object name for the schema.
func Namespace(s *schema.Schema) string { return namespace(s) }

func typeNames(s *schema.Schema) []string {
	var out []string
	for _, st := range s.Structs {
		out = append(out, st.Name)
	}
	for _, u := range s.Unions {
		out = append(out, u.Name)
	}
	for _, e := range s.Enums {
		out = append(out, e.Name)
	}
	return out
}
