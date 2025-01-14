// Package mysqllock provides a distributed locking mechanism leveraging MySQL internal functions.
package mysqllock

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nexmoinc/gosrvlib/pkg/logging"
	"go.uber.org/zap"
)

// ReleaseFunc is an alias for a release lock function.
type ReleaseFunc func() error

var (
	// ErrTimeout is an error when the lock is not acquired within the timeout.
	ErrTimeout = errors.New("acquire lock timeout")

	// ErrFailed is an error when the lock is not acquired.
	ErrFailed = errors.New("failed to acquire a lock")
)

const (
	resLockError    = -1
	resLockTimeout  = 0
	resLockAcquired = 1

	sqlGetLock     = "SELECT COALESCE(GET_LOCK(?, ?), ?)"
	sqlReleaseLock = "DO RELEASE_LOCK(?)"

	keepAliveInterval = 30 * time.Second
	keepAliveSQLQuery = "SELECT 1"
)

// MySQLLock represents a locker.
type MySQLLock struct {
	db *sql.DB
}

// New creates a new instance of the locker.
func New(db *sql.DB) *MySQLLock {
	return &MySQLLock{db: db}
}

// Acquire attempts to acquire a database lock.
func (l *MySQLLock) Acquire(ctx context.Context, key string, timeout time.Duration) (ReleaseFunc, error) {
	conn, err := l.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get mysql connection: %w", err)
	}

	row := conn.QueryRowContext(ctx, sqlGetLock, key, int(timeout.Seconds()), resLockError)

	var res int
	if err = row.Scan(&res); err != nil {
		closeConnection(ctx, conn)
		return nil, fmt.Errorf("unable to scan mysql lock: %w", err)
	}

	if res != resLockAcquired {
		closeConnection(ctx, conn)

		if res == resLockTimeout {
			return nil, ErrTimeout
		}

		return nil, ErrFailed
	}

	releaseCtx, cancelReleaseCtx := context.WithCancel(context.Background())
	releaseCtx = logging.WithLogger(releaseCtx, logging.FromContext(ctx))

	releaseFunc := func() error {
		defer closeConnection(releaseCtx, conn)
		defer cancelReleaseCtx()

		if _, err := conn.ExecContext(releaseCtx, sqlReleaseLock, key); err != nil {
			return fmt.Errorf("unable to release mysql lock: %w", err)
		}

		return nil
	}

	go keepConnectionAlive(releaseCtx, conn, keepAliveInterval)

	return releaseFunc, nil
}

func keepConnectionAlive(ctx context.Context, conn *sql.Conn, interval time.Duration) {
	for {
		select {
		case <-time.After(interval):
			//nolint:rowserrcheck
			rows, err := conn.QueryContext(ctx, keepAliveSQLQuery)
			if err != nil {
				logging.FromContext(ctx).Error("error while keeping mysqllock connection alive", zap.Error(err))
				return
			}

			logging.Close(ctx, rows, "failed closing SQL rows")
		case <-ctx.Done():
			return
		}
	}
}

func closeConnection(ctx context.Context, conn *sql.Conn) {
	logging.Close(ctx, conn, "error closing mysql lock connection")
}
