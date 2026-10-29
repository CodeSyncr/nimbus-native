// Package gen turns a wire schema into code: Go (methods on the schema's own
// types), Swift (NimbusNativeKit hosts) and Kotlin (Android hosts). All three
// produce byte-identical encodings; conformance_test.go proves it.
package gen

import (
	"strings"
	"unicode"

	"github.com/CodeSyncr/nimbus-native/wire/schema"
)

// lowerCamel turns a Go exported name into a field or case name:
// "ID" → "id", "UserID" → "userID", "HTTPServer" → "httpServer".
func lowerCamel(s string) string {
	r := []rune(s)
	i := 0
	for i < len(r) && unicode.IsUpper(r[i]) {
		i++
	}
	switch {
	case i == 0:
		return s
	case i == 1 || i == len(r):
		// "Name" → "name", "ID" → "id"
	default:
		// "HTTPServer": keep the S that starts "Server"
		i--
	}
	for j := 0; j < i; j++ {
		r[j] = unicode.ToLower(r[j])
	}
	return string(r)
}

// upperSnake turns a Go name into a Kotlin constant name: "BigText" → "BIG_TEXT".
func upperSnake(s string) string {
	var b strings.Builder
	r := []rune(s)
	for i, c := range r {
		if i > 0 && unicode.IsUpper(c) && (unicode.IsLower(r[i-1]) || i+1 < len(r) && unicode.IsLower(r[i+1])) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToUpper(c))
	}
	return b.String()
}

// enumConst is an enum value's name without its type prefix:
// KindBigText of Kind → "BigText".
func enumConst(e *schema.EnumDef, v schema.EnumValue) string {
	if rest, ok := strings.CutPrefix(v.Name, e.Name); ok && rest != "" {
		return rest
	}
	return v.Name
}

// namespace is the Swift enum / Kotlin object that holds a schema's types:
// package "proto" → "Proto".
func namespace(s *schema.Schema) string {
	r := []rune(s.Package)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

var swiftKeywords = set("associatedtype", "class", "deinit", "enum", "extension", "fileprivate", "func", "import",
	"init", "inout", "internal", "let", "open", "operator", "private", "protocol", "public", "rethrows", "static",
	"struct", "subscript", "typealias", "var", "break", "case", "continue", "default", "defer", "do", "else",
	"fallthrough", "for", "guard", "if", "in", "repeat", "return", "switch", "where", "while", "as", "Any", "catch",
	"false", "is", "nil", "super", "self", "Self", "throw", "throws", "true", "try", "Type")

var kotlinKeywords = set("as", "break", "class", "continue", "do", "else", "false", "for", "fun", "if", "in",
	"interface", "is", "null", "object", "package", "return", "super", "this", "throw", "true", "try", "typealias",
	"typeof", "val", "var", "when", "while")

func swiftName(goName string) string {
	n := lowerCamel(goName)
	if swiftKeywords[n] {
		return "`" + n + "`"
	}
	return n
}

func kotlinName(goName string) string {
	n := lowerCamel(goName)
	if kotlinKeywords[n] {
		return "`" + n + "`"
	}
	return n
}

func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}
