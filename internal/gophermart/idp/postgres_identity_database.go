package idp

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kerelape/gophermart/internal/accrual"
	"golang.org/x/crypto/bcrypt"
	"sync"
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
