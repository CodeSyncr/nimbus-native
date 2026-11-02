package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

// phone stands in for the native side: it records what Go sent, and
// answers queries with rows given as the phone would encode them.
type phone struct {
	sent []request
	rows string // the JSON answer to a query
	fail string // an exec whose SQL starts with this fails
}

func (p *phone) serve(t *testing.T) {
	old := call
	t.Cleanup(func() { call = old })
	call = func(ctx context.Context, method string, args any) (json.RawMessage, error) {
		b, _ := json.Marshal(args)
		switch method {
		case "open":
			return json.RawMessage(`{"id":"db1"}`), nil
		case "close", "delete":
			return nil, nil
		}
		var r request
		if err := json.Unmarshal(b, &r); err != nil {
			t.Fatal(err)
		}
		p.sent = append(p.sent, r)
		if p.fail != "" && strings.HasPrefix(r.SQL, p.fail) {
			return nil, &Error{Message: "UNIQUE constraint failed: orders.id"}
		}
		if method == "query" {
			return json.RawMessage(p.rows), nil
		}
		return json.RawMessage(`{"changes":1,"lastId":{"i":"9007199254740993"}}`), nil
	}
}

func TestExecAndArgs(t *testing.T) {
	p := &phone{}
	p.serve(t)
	db, err := Open("cafe.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	when := time.Date(2026, 11, 2, 9, 41, 0, 0, time.FixedZone("IST", 5*3600+1800))
	res, err := db.Exec(`INSERT INTO t VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		int64(math.MaxInt64), 1.5, "chai", []byte{0, 1, 2}, nil, true, when, 7)
	if err != nil {
		t.Fatal(err)
	}
	if id, _ := res.LastInsertId(); id != 9007199254740993 {
		t.Fatalf("last id = %d (lost past 2^53?)", id)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatalf("changes = %d", n)
	}
	got := p.sent[0].Args
	want := []any{int64(math.MaxInt64), 1.5, "chai", []byte{0, 1, 2}, nil, int64(1), "2026-11-02T04:11:00Z", int64(7)}
	for i := range want {
		if !reflect.DeepEqual(got[i].v, want[i]) {
			t.Errorf("arg %d = %#v, want %#v", i, got[i].v, want[i])
		}
	}
	b, _ := json.Marshal(p.sent[0])
	if !strings.Contains(string(b), `{"i":"9223372036854775807"}`) || !strings.Contains(string(b), `{"b":"AAEC"}`) {
		t.Fatalf("wire = %s", b)
	}
}

func TestQuery(t *testing.T) {
	p := &phone{rows: `{"columns":["id","item","total","photo","note"],"rows":[
		[{"i":"9007199254740993"},"Latte",180.5,{"b":"/9j/"},null],
		[{"i":"2"},"Chai",60,null,"no sugar"]]}`}
	p.serve(t)
	db, _ := Open("cafe.db")
	defer db.Close()
	rows, err := db.Query(`SELECT id, item, total, photo, note FROM orders WHERE total > ?`, 50)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if cols, _ := rows.Columns(); len(cols) != 5 {
		t.Fatalf("columns = %v", cols)
	}
	type order struct {
		id    int64
		item  string
		total float64
		photo []byte
		note  *string
	}
	var got []order
	for rows.Next() {
		var o order
		if err := rows.Scan(&o.id, &o.item, &o.total, &o.photo, &o.note); err != nil {
			t.Fatal(err)
		}
		got = append(got, o)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].id != 9007199254740993 || got[0].item != "Latte" || got[0].total != 180.5 ||
		string(got[0].photo) != "\xff\xd8\xff" || got[0].note != nil || got[1].note == nil || *got[1].note != "no sugar" {
		t.Fatalf("rows = %+v", got)
	}
}

func TestTransaction(t *testing.T) {
	p := &phone{}
	p.serve(t)
	db, _ := Open("cafe.db")
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	tx.Exec(`INSERT INTO orders (item) VALUES (?)`, "Latte")
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx, _ = db.Begin()
	tx.Rollback()
	var sqls []string
	for _, r := range p.sent {
		sqls = append(sqls, r.SQL)
	}
	if strings.Join(sqls, "|") != "BEGIN|INSERT INTO orders (item) VALUES (?)|COMMIT|BEGIN|ROLLBACK" {
		t.Fatalf("sent %v", sqls)
	}
	for _, r := range p.sent {
		if r.ID != "db1" {
			t.Fatalf("on connection %q", r.ID)
		}
	}
}

func TestErrors(t *testing.T) {
	p := &phone{fail: "INSERT"}
	p.serve(t)
	db, _ := Open("cafe.db")
	defer db.Close()
	_, err := db.Exec(`INSERT INTO orders (id) VALUES (1)`)
	var e *Error
	if !errors.As(err, &e) || !strings.Contains(e.Message, "UNIQUE") {
		t.Fatalf("err = %v", err)
	}
	if _, err := db.Exec(`SELECT :x`, sql.Named("x", 1)); err == nil {
		t.Fatal("named parameter passed")
	}
	if _, err := db.Exec(`INSERT INTO t VALUES (?)`, struct{}{}); err == nil {
		t.Fatal("a struct passed")
	}
	if _, err := Open(""); err == nil {
		t.Fatal("no name passed")
	}
	bad := &phone{rows: `{"columns":["x"],"rows":[[{"q":"1"}]]}`}
	bad.serve(t)
	if _, err := db.Query(`SELECT 1`); err == nil {
		t.Fatal("a strange cell passed")
	}
}

func TestOneConnection(t *testing.T) {
	(&phone{}).serve(t)
	db, _ := Open("cafe.db")
	defer db.Close()
	if db.Stats().MaxOpenConnections != 1 {
		t.Fatalf("max open = %d", db.Stats().MaxOpenConnections)
	}
}
