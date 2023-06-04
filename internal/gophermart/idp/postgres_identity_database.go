package idp

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kerelape/gophermart/internal/accrual"
	"github.com/pior/runnable"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
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

	_, insertError := p.conn.Exec(
		ctx,
		`INSERT INTO identities(username, password) VALUES($1, $2)`,
		username,
		encodedPasswordHash,
	)
	if err := new(pgconn.PgError); errors.As(insertError, &err) {
		if err.Code == "23505" { // unique violation error
			return ErrDuplicateUsername
		}
	}
	return insertError
}

func (p *PostgresIdentityDatabase) Identity(username string) Identity {
	p.ready.Wait()
	return NewPostgresIdentity(username, p.conn, p.accrual)
}

func (p *PostgresIdentityDatabase) Run(ctx context.Context) error {
	manager := runnable.NewManager()
	manager.Add(runnable.Func(p.connect))
	manager.Add(runnable.Every(runnable.Func(p.update), time.Second))
	return manager.Build().Run(ctx)
}

func (p *PostgresIdentityDatabase) connect(ctx context.Context) error {
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

func (p *PostgresIdentityDatabase) update(ctx context.Context) error {
	p.ready.Wait()
	rows, queryOrdersError := p.conn.Query(
		ctx,
		`SELECT id FROM orders WHERE status = $1 OR status = $2`,
		string(OrderStatusNew), string(OrderStatusProcessing),
	)
	if queryOrdersError != nil {
		return queryOrdersError
	}

	ids := make([]string, 0)
	for rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}

	eg, egctx := errgroup.WithContext(ctx)
	eg.SetLimit(len(ids))
	for _, id := range ids {
		eg.Go(func(ctx context.Context, id string) func() error {
			return func() error {
				orderInfo, orderInfoError := p.accrual.OrderInfo(ctx, id)
				var status OrderStatus
				if orderInfoError != nil {
					if errors.Is(orderInfoError, accrual.ErrUnknownOrder) {
						status = OrderStatusInvalid
					} else {
						return orderInfoError
					}
				} else {
					status = MakeOrderStatus(orderInfo.Status)
				}

				_, err := p.conn.Exec(
					ctx,
					`UPDATE orders SET status = $1, accrual = $2 WHERE id = $3`,
					string(status),
					orderInfo.Accrual,
					orderInfo.Order,
				)
				return err
			}
		}(egctx, id))
	}

	if err := eg.Wait(); err != nil {
		tooManyRequestsError := new(accrual.TooManyRequestsError)
		if errors.As(err, &tooManyRequestsError) {
			time.Sleep(tooManyRequestsError.RetryAfter)
			return nil
		}
		return err
	}

	return nil
}
