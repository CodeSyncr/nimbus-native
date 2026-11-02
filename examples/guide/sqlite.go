package guide

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/sqlite"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Orders keeps a list of orders in SQLite, and adds one on each tap.
type Orders struct{}

type order struct {
	ID    int64
	Item  string
	Total float64
}

var ordersDB = func() *sql.DB {
	db, _ := sqlite.Open("cafe.db") // only fails without a name
	return db
}()

func loadOrders(ctx context.Context) ([]order, error) {
	if _, err := ordersDB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS orders (id INTEGER PRIMARY KEY, item TEXT, total REAL)`); err != nil {
		return nil, err
	}
	rows, err := ordersDB.QueryContext(ctx, `SELECT id, item, total FROM orders ORDER BY id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []order
	for rows.Next() {
		var o order
		if err := rows.Scan(&o.ID, &o.Item, &o.Total); err != nil {
			return nil, err
		}
		list = append(list, o)
	}
	return list, rows.Err()
}

func addOrder(ctx context.Context, item string) (int64, error) {
	res, err := ordersDB.ExecContext(ctx, `INSERT INTO orders (item, total) VALUES (?, ?)`, item, 180.0)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (Orders) Render(c *ui.Ctx) ui.Component {
	list, setList := ui.State(c, []order{})
	status, setStatus := ui.State(c, "")
	reload := func() {
		go func() {
			l, err := loadOrders(context.Background())
			if err != nil {
				setStatus(err.Error())
				return
			}
			setList(l)
		}()
	}
	ui.Effect(c, func(context.Context) func() { reload(); return nil }, ui.Once)
	kids := []ui.Component{
		ui.Button{Label: "Add a latte", OnPress: func() {
			go func() {
				if _, err := addOrder(context.Background(), "Latte"); err != nil {
					setStatus(err.Error())
				}
				reload()
			}()
		}},
		ui.Text(status, ui.Style{}),
	}
	for _, o := range list {
		kids = append(kids, ui.ListRow{Title: fmt.Sprintf("#%d %s", o.ID, o.Item), Subtitle: fmt.Sprintf("₹%.0f", o.Total)})
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12}, kids...)
}
