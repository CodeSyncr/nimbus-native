// Package api calls a JSON server — a Nimbus app — with typed endpoints.
// Declare each endpoint once, in a package both the server and the app
// import, and every call is checked by the compiler on both sides:
//
//	// shared/api.go
//	var ListOrders  = api.GET[ListOrdersReq, []Order]("/api/orders")
//	var GetOrder    = api.GET[OrderRef, Order]("/api/orders/{id}")
//	var CreateOrder = api.POST[NewOrder, Order]("/api/orders")
//
//	// in the app
//	client := api.New("https://example.com")
//	orders, err := api.Call(ctx, client, ListOrders, ListOrdersReq{Status: "paid"})
//
// The package depends only on the standard library, so it is safe to import
// from shared code that also builds into the app.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
)

// Endpoint is one route: its method and path, and the Go types of its
// request and response. Path parameters are written {name} and filled from
// the request field tagged `path:"name"`.
type Endpoint[Req, Resp any] struct {
	Method string
	Path   string
}

func GET[Req, Resp any](path string) Endpoint[Req, Resp] {
	return Endpoint[Req, Resp]{http.MethodGet, path}
}
func POST[Req, Resp any](path string) Endpoint[Req, Resp] {
	return Endpoint[Req, Resp]{http.MethodPost, path}
}
func PUT[Req, Resp any](path string) Endpoint[Req, Resp] {
	return Endpoint[Req, Resp]{http.MethodPut, path}
}
func PATCH[Req, Resp any](path string) Endpoint[Req, Resp] {
	return Endpoint[Req, Resp]{http.MethodPatch, path}
}
func DELETE[Req, Resp any](path string) Endpoint[Req, Resp] {
	return Endpoint[Req, Resp]{http.MethodDelete, path}
}

// None is the request or response type of an endpoint that has none.
type None struct{}

// Client sends requests to one server.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	// Token, when set, is sent as "Authorization: Bearer <token>" (e.g. a
	// Nimbus session or API token).
	Token func() string
	// Header adds headers to every request.
	Header http.Header
}

// New is a client for the server at baseURL, with a 30-second timeout.
func New(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// Error is a response with a non-2xx status. Fields holds a Nimbus 422's
// validation errors, so ui.FieldErrorsOf(err) puts them on a form.
type Error struct {
	Status  int
	Message string
	Fields  map[string][]string
	Body    []byte
}

func (e *Error) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("api: %d %s", e.Status, e.Message)
	}
	return fmt.Sprintf("api: %d %s", e.Status, http.StatusText(e.Status))
}

// ToMap returns the field errors (the shape Nimbus validation uses).
func (e *Error) ToMap() map[string][]string { return e.Fields }

// Call sends req to the endpoint and decodes the response. GET and DELETE
// send the request as query parameters (by json name, without the path
// parameters); the others as a JSON body — tag path fields json:"-" to keep
// them out of it.
func Call[Req, Resp any](ctx context.Context, c *Client, e Endpoint[Req, Resp], req Req) (Resp, error) {
	var zero Resp
	path, used, err := fillPath(e.Path, req)
	if err != nil {
		return zero, err
	}
	u := c.BaseURL + path
	var body io.Reader
	if e.Method == http.MethodGet || e.Method == http.MethodDelete {
		if q := query(req, used); len(q) > 0 {
			u += "?" + q.Encode()
		}
	} else if _, none := any(req).(None); !none {
		b, err := json.Marshal(req)
		if err != nil {
			return zero, fmt.Errorf("api: encoding %T: %w", req, err)
		}
		body = bytes.NewReader(b)
	}

	r, err := http.NewRequestWithContext(ctx, e.Method, u, body)
	if err != nil {
		return zero, err
	}
	r.Header.Set("Accept", "application/json")
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	for k, vs := range c.Header {
		for _, v := range vs {
			r.Header.Add(k, v)
		}
	}
	if c.Token != nil {
		if t := c.Token(); t != "" {
			r.Header.Set("Authorization", "Bearer "+t)
		}
	}

	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(r)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return zero, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return zero, parseError(resp.StatusCode, data)
	}
	if _, none := any(zero).(None); none || len(bytes.TrimSpace(data)) == 0 {
		return zero, nil
	}
	var out Resp
	if err := json.Unmarshal(data, &out); err != nil {
		return zero, fmt.Errorf("api: decoding %s %s into %T: %w", e.Method, e.Path, out, err)
	}
	return out, nil
}

// parseError reads Nimbus's error bodies: {"message", "errors": {field: [..]}}
// for validation, {"error": "..."} (sometimes with "message") otherwise.
func parseError(status int, data []byte) *Error {
	e := &Error{Status: status, Body: data}
	var body struct {
		Message string          `json:"message"`
		Error   string          `json:"error"`
		Errors  json.RawMessage `json:"errors"`
	}
	if json.Unmarshal(data, &body) == nil {
		e.Message = body.Message
		if e.Message == "" {
			e.Message = body.Error
		}
		var many map[string][]string
		var one map[string]string
		switch {
		case json.Unmarshal(body.Errors, &many) == nil && len(many) > 0:
			e.Fields = many
		case json.Unmarshal(body.Errors, &one) == nil && len(one) > 0:
			e.Fields = map[string][]string{}
			for k, v := range one {
				e.Fields[k] = []string{v}
			}
		}
	}
	return e
}

// fillPath replaces {name} with the request field tagged path:"name".
func fillPath(path string, req any) (string, map[string]bool, error) {
	used := map[string]bool{}
	if !strings.Contains(path, "{") {
		return path, used, nil
	}
	fields := fieldsByTag(req, "path")
	var b strings.Builder
	for {
		i := strings.IndexByte(path, '{')
		if i < 0 {
			b.WriteString(path)
			break
		}
		j := strings.IndexByte(path[i:], '}')
		if j < 0 {
			return "", nil, fmt.Errorf("api: unclosed { in path %q", path)
		}
		name := path[i+1 : i+j]
		v, ok := fields[name]
		if !ok {
			return "", nil, fmt.Errorf("api: path %q needs a field tagged path:%q on %T", path, name, req)
		}
		b.WriteString(path[:i])
		b.WriteString(url.PathEscape(fmt.Sprint(v.Interface())))
		used[name] = true
		path = path[i+j+1:]
	}
	return b.String(), used, nil
}

// query encodes a request's fields as query parameters by json name,
// skipping path parameters, zero values and json:"-".
func query(req any, pathParams map[string]bool) url.Values {
	q := url.Values{}
	v := reflect.ValueOf(req)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return q
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() || f.Tag.Get("path") != "" {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		fv := v.Field(i)
		if fv.IsZero() || pathParams[name] {
			continue
		}
		if fv.Kind() == reflect.Slice {
			for j := 0; j < fv.Len(); j++ {
				q.Add(name, fmt.Sprint(fv.Index(j).Interface()))
			}
			continue
		}
		q.Set(name, fmt.Sprint(fv.Interface()))
	}
	return q
}

func fieldsByTag(req any, tag string) map[string]reflect.Value {
	out := map[string]reflect.Value{}
	v := reflect.ValueOf(req)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return out
	}
	for i := 0; i < v.NumField(); i++ {
		if name := v.Type().Field(i).Tag.Get(tag); name != "" {
			out[name] = v.Field(i)
		}
	}
	return out
}
