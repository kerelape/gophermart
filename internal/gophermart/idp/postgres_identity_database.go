package idp

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"github.com/kerelape/gophermart/internal/accrual"
	_ "github.com/lib/pq"
	"github.com/pior/runnable"
	"golang.org/x/crypto/bcrypt"
	"sync"
	"time"
)

type PostgresIdentityDatabase struct {
	dsn     string
	accrual accrual.Accrual

	db *sql.DB
}

func NewPostgresIdentityDatabase(dsn string, accrual accrual.Accrual) *PostgresIdentityDatabase {
	return &PostgresIdentityDatabase{
		dsn:     dsn,
		accrual: accrual,

		db: nil,
	}
}

func (p *PostgresIdentityDatabase) Create(ctx context.Context, username, password string) error {
	if p.db == nil {
		panic("not yet connected to the database")
	}

	passwordHash, passwordHashError := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if passwordHashError != nil {
		return passwordHashError
	}
	encodedPasswordHash := base64.StdEncoding.EncodeToString(passwordHash)

	rows, findDuplicateError := p.db.QueryContext(ctx, `SELECT ALL FROM identities where username = $1`, username)
	if findDuplicateError != nil {
		return findDuplicateError
	}
	if rows.Next() {
		return ErrDuplicateUsername
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if rows.Err() != nil {
		return rows.Err()
	}

	transaction, transactionError := p.db.Begin()
	if transactionError != nil {
		return transactionError
	}
	query := `INSERT INTO identities(username, password) VALUES($1, $2)`
	_, execError := transaction.ExecContext(ctx, query, username, encodedPasswordHash)
	if execError != nil {
		if err := transaction.Rollback(); err != nil {
			return err
		}
		return execError
	}
	return transaction.Commit()
}

func (p *PostgresIdentityDatabase) Identity(username string) Identity {
	return NewPostgresIdentity(username, p.db, p.accrual)
}

func (p *PostgresIdentityDatabase) Run(ctx context.Context) error {
	manager := runnable.NewManager()
	manager.Add(runnable.Func(p.updateStatuses))
	manager.Add(runnable.Func(p.connect))
	return manager.Build().Run(ctx)
}

func (p *PostgresIdentityDatabase) connect(ctx context.Context) error {
	db, openError := sql.Open("postgres", p.dsn)
	if openError != nil {
		return openError
	}

	transaction, transactionError := db.Begin()
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
		if _, err := transaction.ExecContext(ctx, query); err != nil {
			if err := transaction.Rollback(); err != nil {
				return err
			}
			return err
		}
	}

	if err := transaction.Commit(); err != nil {
		return transaction.Rollback()
	}

	p.db = db
	<-ctx.Done()
	return p.db.Close()
}

func (p *PostgresIdentityDatabase) updateStatuses(ctx context.Context) error {
	for {
		timeToSleep := time.Second * 2

		rows, queryError := p.db.QueryContext(
			ctx,
			`SELECT id FROM orders WHERE status != $1 and status != $2`,
			OrderStatusInvalid,
			OrderStatusProcessed,
		)
		if queryError != nil {
			return queryError
		}
		if err := rows.Close(); err != nil {
			return err
		}

		ordersToUpdate := make([]string, 0)
		for rows.Next() {
			if rows.Err() != nil {
				return rows.Err()
			}
			var order string
			if err := rows.Scan(&order); err != nil {
				return err
			}
			ordersToUpdate = append(ordersToUpdate, order)
		}

		updatedOrders := make([]Order, len(ordersToUpdate))
		errorsChannel := make(chan error, len(ordersToUpdate))
		wg := sync.WaitGroup{}
		wg.Add(len(ordersToUpdate))
		for i, orderID := range ordersToUpdate {
			go func(i int, id string) {
				defer wg.Done()
				orderInfo, err := p.accrual.OrderInfo(ctx, id)
				if err != nil {
					errorsChannel <- err
					return
				}
				updatedOrders[i] = Order{
					ID:     orderInfo.Order,
					Status: OrderStatus(orderInfo.Status),
				}
			}(i, orderID)
		}
		wg.Wait()
		for err := range errorsChannel {
			if errors.Is(err, accrual.ErrTooManyRequests) {
				timeToSleep = time.Second * 60
				break
			}
		}

		time.Sleep(timeToSleep)
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}
