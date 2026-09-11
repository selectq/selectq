package core

import (
	"context"
	"database/sql/driver"
	"fmt"
)

// SQLite foreign-key enforcement is connection-scoped. Initialize every pooled
// connection so address history remains protected when the pool grows.
type foreignKeyConnector struct{ driver.Connector }

func (c foreignKeyConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	exec, ok := conn.(driver.ExecerContext)
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("SQLite connection does not support context execution")
	}
	if _, err = exec.ExecContext(ctx, "PRAGMA foreign_keys=ON", nil); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}
