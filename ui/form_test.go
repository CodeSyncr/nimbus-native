package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

type signup struct{ Name, Email string }

// nimbusErrors stands in for Nimbus's validation.ValidationErrors, which the
// form reads through its ToMap method without importing Nimbus.
type nimbusErrors map[string][]string

func (e nimbusErrors) Error() string              { return "invalid" }
func (e nimbusErrors) ToMap() map[string][]string { return e }
func validateSignup(s signup) error {
	errs := nimbusErrors{}
	if len(strings.TrimSpace(s.Name)) < 2 {
		errs["name"] = []string{"Name must be at least 2 characters"}
	}
	if !strings.Contains(s.Email, "@") {
		errs["email"] = []string{"Enter a valid email"}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

var submitted []signup

type signupForm struct{}

func (signupForm) Render(c *Ctx) *Element {
	f := UseForm(c, signup{}, validateSignup)
	return View(Style{},
		TextField(f.Field("name", func(s *signup) *string { return &s.Name }), FieldOptions{Label: "Name"}),
		TextField(f.Field("email", func(s *signup) *string { return &s.Email }), FieldOptions{Label: "Email"}),
		Pressable(f.Submit(func(s signup) { submitted = append(submitted, s) }), Style{Height: Pt(44)}).TestID("submit"),
	)
}

func TestFormValidation(t *testing.T) {
	submitted = nil
	h := start(t, signupForm{})
	if h.tagOf("error-name") != 0 {
		t.Fatal("errors shown before the user did anything")
	}

	h.send(&render.Press{Tag: h.tagOf("submit")})
	if len(submitted) != 0 || h.tagOf("error-name") == 0 || h.tagOf("error-email") == 0 {
		t.Fatal("an invalid submit should show every error and not submit")
	}

	h.send(&render.Changed{Tag: h.tagOf("field-name"), Text: "Asha"})
	if h.tagOf("error-name") != 0 || h.tagOf("error-email") == 0 {
		t.Fatal("fixing the name should clear only its error")
	}

	// Two edits before a render must both land (functional updates).
	h.rt.receive(render.Encode(&render.Changed{Tag: h.tagOf("field-email"), Text: "asha@"}))
	h.rt.receive(render.Encode(&render.Changed{Tag: h.tagOf("field-email"), Text: "asha@example.com"}))
	h.rt.drain()
	h.send(&render.Press{Tag: h.tagOf("submit")})
	if len(submitted) != 1 || submitted[0] != (signup{"Asha", "asha@example.com"}) {
		t.Fatalf("submitted %v", submitted)
	}
}

func TestSplitErrors(t *testing.T) {
	if fe, err := splitErrors(FieldErrors{"a": {"bad"}}); fe["a"][0] != "bad" || err != nil {
		t.Fatal("FieldErrors not split")
	}
	if fe, err := splitErrors(nimbusErrors{"b": {"worse"}}); fe["b"][0] != "worse" || err != nil {
		t.Fatal("a ToMap error not split")
	}
	plain := errors.New("server unreachable")
	if fe, err := splitErrors(plain); fe != nil || err != plain {
		t.Fatal("a plain error should stay a form error")
	}
}

type serverForm struct{}

func (serverForm) Render(c *Ctx) *Element {
	f := UseForm(c, signup{}, nil) // no local rules: the server decides
	return View(Style{},
		TextField(f.Field("email", func(s *signup) *string { return &s.Email }), FieldOptions{}),
		Pressable(func() { f.SetErrors(nimbusErrors{"email": {"email is taken"}}) }, Style{Height: Pt(44)}).TestID("reject"),
	)
}

func TestFormShowsServerErrors(t *testing.T) {
	h := start(t, serverForm{})
	h.send(&render.Press{Tag: h.tagOf("reject")})
	if h.lastText(h.tagOf("error-email")) == nil || h.lastText(h.tagOf("error-email")).Text != "email is taken" {
		t.Fatal("server error not shown on its field")
	}
	h.send(&render.Changed{Tag: h.tagOf("field-email"), Text: "other@example.com"})
	if h.tagOf("error-email") != 0 {
		t.Fatal("editing the field should clear its server error")
	}
}

type saveButton struct{ onPress func() }

func (b saveButton) Render(c *Ctx) *Element {
	return Pressable(b.onPress, Style{Height: Pt(44)}, Text("Save", Style{}))
}

type ownID struct{}

func (ownID) Render(c *Ctx) *Element { return View(Style{}).TestID("inner") }

type idScreen struct{ onSave func() }

func (s idScreen) Render(c *Ctx) *Element {
	return View(Style{}, C(saveButton{s.onSave}).TestID("save"), C(ownID{}).TestID("outer"))
}

// A TestID on a component names what it renders, unless that has its own.
func TestComponentTestID(t *testing.T) {
	pressed := 0
	h := start(t, idScreen{func() { pressed++ }})
	h.send(&render.Press{Tag: h.tagOf("save")})
	if pressed != 1 {
		t.Fatal("the component's TestID did not reach its Pressable")
	}
	if h.tagOf("inner") == 0 || h.tagOf("outer") != 0 {
		t.Fatal("an element's own TestID should win over its component's")
	}
}
