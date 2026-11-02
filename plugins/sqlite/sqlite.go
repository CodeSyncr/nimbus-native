// Package sqlite is a SQLite database on the phone, through Go's own
// database/sql. It uses the SQLite that comes with iOS and Android, so it
// adds nothing to the app's size and needs no cgo.
//
//	nimbus native plugin add sqlite
//
//	db, err := sqlite.Open("cafe.db")
//	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS orders (id INTEGER PRIMARY KEY, item TEXT, total REAL)`)
//	_, err = db.ExecContext(ctx, `INSERT INTO orders (item, total) VALUES (?, ?)`, "Latte", 180.0)
//	rows, err := db.QueryContext(ctx, `SELECT id, item FROM orders ORDER BY id DESC`)
//
// The database is a file in the app's own folder on the phone, so it stays
// there under `nimbus native dev` too, while Go runs on your Mac.
package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// DriverName is the name the driver is registered under, for sql.Open:
// sql.Open(sqlite.DriverName, "cafe.db") is Open without its one-connection
// limit, which only matters if you change it.
const DriverName = "nimbus-sqlite"

func init() { sql.Register(DriverName, Driver{}) }

// Open opens the database in the app's data folder, making it if needed.
// name is a file name such as "cafe.db", or ":memory:" for one that's gone
// when it closes. The *sql.DB holds one connection, so statements run one at
// a time and a transaction has the database to itself: that's how SQLite
// works best on a phone.
//
//nimbus:param name the database's file name, or ":memory:"
func Open(name string) (*sql.DB, error) {
	if name == "" {
		return nil, errors.New("sqlite: Open needs a file name")
	}
	db := sql.OpenDB(connector{name})
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	return db, nil
}

// Delete removes a database's file, and the files SQLite keeps beside it.
// Close it first.
//
//nimbus:param name the database's file name
func Delete(ctx context.Context, name string) error {
	_, err := callAs[struct{}](ctx, "delete", map[string]string{"name": name})
	return err
}

// Driver is the database/sql driver. Open and sql.Open use it; you don't
// need to.
type Driver struct{}

// Open opens a connection to the named database.
//
//nimbus:param name the database's file name, or ":memory:"
func (Driver) Open(name string) (driver.Conn, error) {
	return connector{name}.Connect(context.Background())
}

type connector struct{ name string }

func (c connector) Connect(ctx context.Context) (driver.Conn, error) {
	r, err := callAs[struct {
		ID string `json:"id"`
	}](ctx, "open", map[string]string{"name": c.name})
	if err != nil {
		return nil, err
	}
	return &conn{id: r.ID}, nil
}

func (connector) Driver() driver.Driver { return Driver{} }

// call reaches the phone; tests replace it.
var call = func(ctx context.Context, method string, args any) (json.RawMessage, error) {
	raw, err := plugin.CallJSON(ctx, "sqlite", method, args)
	var e *plugin.Error
	if errors.As(err, &e) && e.Code == plugin.Failed {
		return nil, &Error{Message: e.Message} // SQLite's own words
	}
	return raw, err
}

// callAs decodes the answer to a call.
func callAs[T any](ctx context.Context, method string, args any) (T, error) {
	var out T
	raw, err := call(ctx, method, args)
	if err != nil || len(raw) == 0 || string(raw) == "null" {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("sqlite: %s: %w", method, err)
	}
	return out, nil
}

// Error is SQLite refusing a statement: a syntax error, a constraint, a
// missing table.
type Error struct {
	Message string // SQLite's own words, such as "UNIQUE constraint failed: orders.item"
}

func (e *Error) Error() string { return "sqlite: " + e.Message }

type conn struct {
	id     string
	closed bool
}

func (c *conn) Prepare(query string) (driver.Stmt, error) { return &stmt{c, query}, nil }

func (c *conn) PrepareContext(_ context.Context, query string) (driver.Stmt, error) {
	return &stmt{c, query}, nil
}

func (c *conn) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	_, err := callAs[struct{}](context.Background(), "close", map[string]string{"id": c.id})
	return err
}

func (c *conn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *conn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	if o.ReadOnly {
		return nil, errors.New("sqlite: read-only transactions aren't supported")
	}
	if _, err := c.exec(ctx, "BEGIN", nil); err != nil {
		return nil, err
	}
	return tx{c}, nil
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.exec(ctx, query, args)
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.query(ctx, query, args)
}

// CheckNamedValue lets bool and time.Time through as they are; the default
// would refuse a few types SQLite has a place for.
func (c *conn) CheckNamedValue(v *driver.NamedValue) error {
	switch x := v.Value.(type) {
	case nil, int64, float64, string, []byte, bool, time.Time:
		return nil
	case int:
		v.Value = int64(x)
		return nil
	default:
		var err error
		v.Value, err = driver.DefaultParameterConverter.ConvertValue(v.Value)
		return err
	}
}

func (c *conn) ResetSession(context.Context) error {
	if c.closed {
		return driver.ErrBadConn
	}
	return nil
}

type tx struct{ c *conn }

func (t tx) Commit() error {
	_, err := t.c.exec(context.Background(), "COMMIT", nil)
	return err
}

func (t tx) Rollback() error {
	_, err := t.c.exec(context.Background(), "ROLLBACK", nil)
	return err
}

type stmt struct {
	c     *conn
	query string
}

func (s *stmt) Close() error  { return nil }
func (s *stmt) NumInput() int { return -1 } // SQLite checks the count itself

func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.c.exec(context.Background(), s.query, named(args))
}

func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.c.query(context.Background(), s.query, named(args))
}

func (s *stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return s.c.exec(ctx, s.query, args)
}

func (s *stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return s.c.query(ctx, s.query, args)
}

func named(args []driver.Value) []driver.NamedValue {
	out := make([]driver.NamedValue, len(args))
	for i, a := range args {
		out[i] = driver.NamedValue{Ordinal: i + 1, Value: a}
	}
	return out
}

// request is a statement for the phone.
type request struct {
	ID   string `json:"id"`
	SQL  string `json:"sql"`
	Args []cell `json:"args"`
}

func (c *conn) request(query string, args []driver.NamedValue) (request, error) {
	r := request{ID: c.id, SQL: query, Args: make([]cell, len(args))}
	for i, a := range args {
		if a.Name != "" {
			return r, fmt.Errorf("sqlite: named parameter %q: use ? and pass values in order", a.Name)
		}
		v, err := encode(a.Value)
		if err != nil {
			return r, err
		}
		r.Args[i] = v
	}
	return r, nil
}

type result struct {
	Changes int64 `json:"changes"`
	LastID  cell  `json:"lastId"`
}

func (r result) LastInsertId() (int64, error) {
	if r.LastID.v == nil {
		return 0, nil
	}
	return r.LastID.v.(int64), nil
}

func (r result) RowsAffected() (int64, error) { return r.Changes, nil }

func (c *conn) exec(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.closed {
		return nil, driver.ErrBadConn
	}
	req, err := c.request(query, args)
	if err != nil {
		return nil, err
	}
	r, err := callAs[result](ctx, "exec", req)
	if err != nil {
		return nil, err
	}
	if _, ok := r.LastID.v.(int64); r.LastID.v != nil && !ok {
		return nil, errors.New("sqlite: the phone sent a strange row id")
	}
	return r, nil
}

func (c *conn) query(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.closed {
		return nil, driver.ErrBadConn
	}
	req, err := c.request(query, args)
	if err != nil {
		return nil, err
	}
	r, err := callAs[struct {
		Columns []string `json:"columns"`
		Rows    [][]cell `json:"rows"`
	}](ctx, "query", req)
	if err != nil {
		return nil, err
	}
	return &rows{columns: r.Columns, rows: r.Rows}, nil
}

// rows are all a query's rows: the phone sends them at once, so a query
// for thousands of rows should say LIMIT.
type rows struct {
	columns []string
	rows    [][]cell
	at      int
}

func (r *rows) Columns() []string { return r.columns }
func (r *rows) Close() error      { r.rows = nil; return nil }

func (r *rows) Next(dest []driver.Value) error {
	if r.at >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.at]
	r.at++
	for i := range dest {
		if i < len(row) {
			dest[i] = row[i].v
		} else {
			dest[i] = nil
		}
	}
	return nil
}

// cell is one value on the way to or from the phone. JSON numbers lose
// whole numbers past 2^53, so integers travel as {"i":"…"} and blobs as
// {"b":"base64"}; text is a string, a real a number, NULL null.
type cell struct{ v driver.Value }

func encode(v driver.Value) (cell, error) {
	switch x := v.(type) {
	case nil, int64, float64, string, []byte:
		return cell{x}, nil
	case bool:
		if x {
			return cell{int64(1)}, nil
		}
		return cell{int64(0)}, nil
	case time.Time:
		return cell{x.UTC().Format(time.RFC3339Nano)}, nil
	default:
		return cell{}, fmt.Errorf("sqlite: can't store a %T", v)
	}
}

func (c cell) MarshalJSON() ([]byte, error) {
	switch x := c.v.(type) {
	case nil:
		return []byte("null"), nil
	case int64:
		return json.Marshal(map[string]string{"i": strconv.FormatInt(x, 10)})
	case []byte:
		return json.Marshal(map[string]string{"b": base64.StdEncoding.EncodeToString(x)})
	default:
		return json.Marshal(x)
	}
}

func (c *cell) UnmarshalJSON(b []byte) error {
	var x any
	if err := json.Unmarshal(b, &x); err != nil {
		return err
	}
	switch v := x.(type) {
	case nil:
		c.v = nil
	case float64, string:
		c.v = v
	case map[string]any:
		if s, ok := v["i"].(string); ok {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return fmt.Errorf("sqlite: integer %q: %w", s, err)
			}
			c.v = n
		} else if s, ok := v["b"].(string); ok {
			d, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return fmt.Errorf("sqlite: blob: %w", err)
			}
			c.v = d
		} else {
			return fmt.Errorf("sqlite: strange value %s", b)
		}
	default:
		return fmt.Errorf("sqlite: strange value %s", b)
	}
	return nil
}

var (
	_ driver.ConnBeginTx        = (*conn)(nil)
	_ driver.ExecerContext      = (*conn)(nil)
	_ driver.QueryerContext     = (*conn)(nil)
	_ driver.ConnPrepareContext = (*conn)(nil)
	_ driver.NamedValueChecker  = (*conn)(nil)
	_ driver.SessionResetter    = (*conn)(nil)
	_ driver.StmtExecContext    = (*stmt)(nil)
	_ driver.StmtQueryContext   = (*stmt)(nil)
	_ driver.Connector          = connector{}
)
