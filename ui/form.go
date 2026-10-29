package ui

import (
	"errors"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// FieldErrors are validation errors by field name — the shape Nimbus's
// validation.ValidationErrors has, so server rules can validate app forms.
type FieldErrors map[string][]string

// Form is a form's state: its value, which fields the user has touched, and
// the validation errors. Get one with UseForm.
type Form[T any] struct {
	value     T
	update    func(func(T) T) // functional updates: quick edits never overwrite each other
	touched   map[string]bool
	touch     func(func(map[string]bool) map[string]bool)
	errs      FieldErrors
	formError error
}

// UseForm keeps a form's value in state and validates it on every change.
// validate returns nil when the value is valid; field errors come from a
// FieldErrors, from Nimbus's validation.ValidationErrors, or from any error
// with ToMap() map[string][]string. Pass validation.ValidateStruct to share
// your server's rules:
//
//	form := ui.UseForm(c, Signup{}, validation.ValidateStruct)
func UseForm[T any](c *Ctx, initial T, validate func(T) error) *Form[T] {
	value, update := Reducer(c, initial)
	touched, touch := Reducer(c, map[string]bool{})
	f := &Form[T]{value: value, update: update, touched: touched, touch: touch}
	if validate != nil {
		f.errs, f.formError = splitErrors(validate(value))
	}
	return f
}

// Value is the form's current value.
func (f *Form[T]) Value() T { return f.value }

// Valid reports whether the value passes validation.
func (f *Form[T]) Valid() bool { return len(f.errs) == 0 && f.formError == nil }

// Errors are all field errors, touched or not.
func (f *Form[T]) Errors() FieldErrors { return f.errs }

// Update changes the value; it re-renders and re-validates.
func (f *Form[T]) Update(change func(*T)) {
	f.update(func(v T) T {
		change(&v)
		return v
	})
}

// Field binds one text field of the value: name is the validation key (as
// in the rules, usually the json name) and at points at the field.
func (f *Form[T]) Field(name string, at func(*T) *string) Field {
	v := f.value
	fd := Field{Name: name, Value: *at(&v)}
	if f.touched[name] {
		fd.Error = first(f.errs[name])
	}
	fd.OnChange = func(s string) {
		f.update(func(v T) T {
			*at(&v) = s
			return v
		})
		f.touch(func(t map[string]bool) map[string]bool {
			if t[name] {
				return t
			}
			t = copyTouched(t)
			t[name] = true
			return t
		})
	}
	return fd
}

// Submit returns a handler for the submit button: when the value is valid
// it calls onValid with it; otherwise every field shows its error.
func (f *Form[T]) Submit(onValid func(T)) func() {
	return func() {
		if f.Valid() {
			onValid(f.value)
			return
		}
		errs := f.errs
		f.touch(func(t map[string]bool) map[string]bool {
			t = copyTouched(t)
			for name := range errs {
				t[name] = true
			}
			return t
		})
	}
}

// FormError is the validation error that belongs to no single field.
func (f *Form[T]) FormError() error { return f.formError }

// Field is one bound text field: its value, the error to show (empty until
// the user has touched it or tried to submit), and its change handler.
type Field struct {
	Name     string
	Value    string
	Error    string
	OnChange func(string)
}

// FieldOptions configure a TextField.
type FieldOptions struct {
	Label       string
	Placeholder string
	Keyboard    render.Keyboard
	Secure      bool
	Style       Style // for the input box
}

// TextField is a labelled input for a form Field, with its error below.
//
//	ui.TextField(form.Field("email", func(s *Signup) *string { return &s.Email }),
//		ui.FieldOptions{Label: "Email", Keyboard: render.KeyboardEmail})
func TextField(f Field, o FieldOptions) *Element { return C(textField{f, o}) }

type textField struct {
	f Field
	o FieldOptions
}

func (t textField) Render(c *Ctx) *Element {
	f, o, th := t.f, t.o, UseTheme(c)
	box := Style{BorderWidth: 1, BorderColor: th.Border, Radius: th.Radius - 2, FontSize: th.Body, Background: th.Surface}.With(o.Style)
	if f.Error != "" {
		box.BorderColor = th.Danger
	}
	in := TextInput(f.Value, f.OnChange, box).Placeholder(o.Placeholder).Keyboard(o.Keyboard).TestID("field-" + f.Name)
	if o.Secure {
		in.Secure()
	}
	var label, msg *Element
	if o.Label != "" {
		label = Text(o.Label, Style{FontSize: th.Caption, FontWeight: 600, Color: th.TextMuted})
	}
	if f.Error != "" {
		msg = Text(f.Error, Style{FontSize: th.Caption, Color: th.Danger}).TestID("error-" + f.Name)
	}
	return View(Style{Gap: 6}, label, in, msg)
}

// FieldErrorsOf reads field errors out of a validation error — a
// FieldErrors, Nimbus's validation.ValidationErrors (for example from a
// server's 422 response), or any error with ToMap() map[string][]string.
// It is nil when err carries none.
func FieldErrorsOf(err error) FieldErrors {
	fe, _ := splitErrors(err)
	return fe
}

// splitErrors turns a validation error into field errors and a form-level
// error.
func splitErrors(err error) (FieldErrors, error) {
	if err == nil {
		return nil, nil
	}
	var fe FieldErrors
	if errors.As(err, &fe) {
		return fe, nil
	}
	var m interface{ ToMap() map[string][]string }
	if errors.As(err, &m) {
		return FieldErrors(m.ToMap()), nil
	}
	return nil, err
}

// Error lets FieldErrors be returned as an error.
func (e FieldErrors) Error() string {
	for name, msgs := range e {
		if len(msgs) > 0 {
			return name + ": " + msgs[0]
		}
	}
	return "invalid"
}

func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

func copyTouched(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}
