package idp

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/kerelape/gophermart/internal/accrual"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"sync"
	"time"
)

type PostgresIdentityDatabase struct {
	dsn     string
	accrual accrual.Accrual

	conn  *pgx.Conn
	ready *sync.WaitGroup
}

func NewPostgresIdentityDatabase(dsn string, accrual accrual.Accrual) *PostgresIdentityDatabase {
	wg := sync.WaitGroup{}
	wg.Add(1)
	return &PostgresIdentityDatabase{
		dsn:     dsn,
		accrual: accrual,

		conn:  nil,
		ready: &wg,
	}
}

func (p *PostgresIdentityDatabase) Create(ctx context.Context, username, password string) error {
	p.ready.Wait()
	passwordHash, passwordHashError := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if passwordHashError != nil {
		return passwordHashError
	}
	encodedPasswordHash := base64.StdEncoding.EncodeToString(passwordHash)

	rows, findDuplicateError := p.conn.Query(ctx, `SELECT ALL FROM identities where username = $1`, username)
	if findDuplicateError != nil {
		return findDuplicateError
	}
	if rows.Next() {
		return ErrDuplicateUsername
	}
	rows.Close()
	if rows.Err() != nil {
		return rows.Err()
	}

	transaction, transactionError := p.conn.Begin(ctx)
	if transactionError != nil {
		return transactionError
	}
	query := `INSERT INTO identities(username, password) VALUES($1, $2)`
	_, execError := transaction.Exec(ctx, query, username, encodedPasswordHash)
	if execError != nil {
		if err := transaction.Rollback(ctx); err != nil {
			return err
		}
		return execError
	}
	return transaction.Commit(ctx)
}

func (p *PostgresIdentityDatabase) Identity(username string) Identity {
	p.ready.Wait()
	return NewPostgresIdentity(username, p.conn, p.accrual)
}

func (p *PostgresIdentityDatabase) UpdateOrder(ctx context.Context, id string, status OrderStatus, accrual float64) error {
	p.ready.Wait()
	_, err := p.conn.Exec(
		ctx,
		`UPDATE orders SET status = $1, accrual = $2 WHERE id = $3`,
		string(status),
		accrual,
		id,
	)
	return err
}

func (p *PostgresIdentityDatabase) Orders(ctx context.Context, statuses ...OrderStatus) ([]Order, error) {
	p.ready.Wait()
	if len(statuses) == 0 {
		statuses = []OrderStatus{
			OrderStatusInvalid,
			OrderStatusProcessed,
			OrderStatusProcessing,
			OrderStatusNew,
		}
	}
	stringStatuses := make([]string, 0, len(statuses))
	for _, status := range statuses {
		stringStatuses = append(stringStatuses, string(status))
	}
	rows, queryError := p.conn.Query(
		ctx,
		`SELECT (id, status, accrual, time) FROM orders WHERE status IN ($1)`,
		strings.Join(stringStatuses, ", "),
	)
	if queryError != nil {
		return nil, queryError
	}

	orders := make([]Order, 0)
	for rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		order := Order{}
		var (
			orderTime int64
			status    string
		)
		if err := rows.Scan(&order.ID, &status, &order.Accrual, &orderTime); err != nil {
			return nil, err
		}
		order.Status = OrderStatus(status)
		order.Time = time.UnixMilli(orderTime)
		orders = append(orders, order)
	}
	rows.Close()
	return orders, nil
}

func (p *PostgresIdentityDatabase) Run(ctx context.Context) error {
	if p.conn != nil {
		return errors.New("connection is already initialized")
	}

	conn, connectError := pgx.Connect(ctx, p.dsn)
	if connectError != nil {
		return connectError
	}

	transaction, transactionError := conn.Begin(ctx)
	if transactionError != nil {
		return transactionError
	}

	queries := []string{
		// Create identities table.
		`
		CREATE TABLE IF NOT EXISTS identities(
			username TEXT PRIMARY KEY UNIQUE,
			password TEXT
		)
		`,
		// Create orders table.
		`
		CREATE TABLE IF NOT EXISTS orders(
			id TEXT PRIMARY KEY UNIQUE,
		    owner TEXT,
		    time BIGINT,
		    status TEXT,
		    accrual DECIMAL
		)
		`,
		// Create withdrawals table.
		`
		CREATE TABLE IF NOT EXISTS withdrawals(
		    orderID TEXT UNIQUE PRIMARY KEY,
			sum DECIMAL,
			time BIGINT,
		    owner TEXT
		)
		`,
	}

	for _, query := range queries {
		if _, err := transaction.Exec(ctx, query); err != nil {
			if err := transaction.Rollback(ctx); err != nil {
				return err
			}
			return err
		}
	}

	if err := transaction.Commit(ctx); err != nil {
		return transaction.Rollback(ctx)
	}

	p.conn = conn
	p.ready.Done()
	<-ctx.Done()
	return p.conn.Close(context.Background())
}
