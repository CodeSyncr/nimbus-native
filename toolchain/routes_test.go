package toolchain

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func pagesModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "go.mod"), "module shop\n\ngo 1.21\n\nrequire github.com/CodeSyncr/nimbus-native v0.0.0\n\nreplace github.com/CodeSyncr/nimbus-native => "+repo+"\n")
	page := func(dir, pkg, fields string) {
		write(t, filepath.Join(root, "pages", dir, "page.go"), "package "+pkg+`

import "github.com/CodeSyncr/nimbus-native/ui"

type Page struct{ `+fields+` }

func (Page) Render(c *ui.Ctx) *ui.Element { return nil }
`)
	}
	page("", "pages", "")
	page("orders", "orders", "")
	page("orders/new", "neworder", "")
	page("orders/_id", "order", "ID int `param:\"id\"`")
	page("orders/_id/items/_item", "item", "Order int64 `param:\"id\"`; Item string `param:\"item\"`")
	page("order-history", "history", "")
	page("ui", "uipage", "") // must not clash with the ui import
	return root
}

func TestGenRoutes(t *testing.T) {
	root := pagesModule(t)
	if err := (&Env{Out: os.Stderr}).GenRoutes(root); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(root, "pages", RoutesFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`p_orders_id "shop/pages/orders/_id"`,
		`{Pattern: "/orders/:id/items/:item"`,
		`return p_orders_id_items_item.Page{Order: v0, Item: p.Get("item")}, true`,
		`p_order_history "shop/pages/order-history"`,
		`p_ui "shop/pages/ui"`,
		`return Page{}, true`,
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("routes_gen.go lacks %s:\n%s", want, src)
		}
	}

	// The table compiles and matches: a tiny program in the module uses it.
	write(t, filepath.Join(root, "cmd/check/main.go"), `package main

import (
	"fmt"

	"github.com/CodeSyncr/nimbus-native/router"

	"shop/pages"
	item "shop/pages/orders/_id/items/_item"
	order "shop/pages/orders/_id"
	"shop/pages/orders/new"
)

func main() {
	router.Register(pages.Routes)
	a, _, _ := router.Match("/orders/42")
	b, _, _ := router.Match("/orders/new")
	c, _, _ := router.Match("/orders/7/items/x-1")
	_, _, bad := router.Match("/orders/nope/items/x")
	fmt.Println(a == order.Page{ID: 42}, b == neworder.Page{}, c == item.Page{Order: 7, Item: "x-1"}, !bad)
}
`)
	cmd := exec.Command("go", "run", "./cmd/check")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "true true true true" {
		t.Fatalf("check: %v\n%s", err, out)
	}
}

func TestGenRoutesErrors(t *testing.T) {
	for name, c := range map[string]struct {
		dir, fields, want string
	}{
		"missing param":   {"orders/_id", "", `needs a field tagged ` + "`param:\"id\"`"},
		"unknown param":   {"orders", "ID int `param:\"id\"`", `has no _id folder`},
		"bad param type":  {"orders/_id", "ID float64 `param:\"id\"`", `must be a string, int or int64`},
		"bad folder name": {"Orders", "", `lower-case letters`},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write(t, filepath.Join(root, "go.mod"), "module shop\n")
			write(t, filepath.Join(root, "pages", c.dir, "page.go"), "package p\n\ntype Page struct{ "+c.fields+" }\n")
			err := (&Env{Out: os.Stderr}).GenRoutes(root)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error containing %q", err, c.want)
			}
		})
	}

	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module shop\n")
	write(t, filepath.Join(root, "pages/orders/_id/page.go"), "package p\n\ntype Page struct{ ID string `param:\"id\"` }\n")
	write(t, filepath.Join(root, "pages/orders/_slug/page.go"), "package p\n\ntype Page struct{ S string `param:\"slug\"` }\n")
	if err := (&Env{Out: os.Stderr}).GenRoutes(root); err == nil || !strings.Contains(err.Error(), "only one parameter folder") {
		t.Fatalf("sibling parameter folders: %v", err)
	}

	if err := (&Env{Out: os.Stderr}).GenRoutes(t.TempDir()); err != nil {
		t.Fatalf("no pages/ should be a no-op: %v", err)
	}
}
