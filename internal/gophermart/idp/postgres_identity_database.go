package idp

import (
	"context"
	"encoding/base64"
	"github.com/jackc/pgx/v5"
	"github.com/kerelape/gophermart/internal/accrual"
	"golang.org/x/crypto/bcrypt"
)

type PostgresIdentityDatabase struct {
	dsn     string
	accrual accrual.Accrual

	conn *pgx.Conn
}

func NewPostgresIdentityDatabase(dsn string, accrual accrual.Accrual) *PostgresIdentityDatabase {
	return &PostgresIdentityDatabase{
		dsn:     dsn,
		accrual: accrual,

		conn: nil,
	}
}

func (p *PostgresIdentityDatabase) Create(ctx context.Context, username, password string) error {
	if p.conn == nil {
		panic("not yet connected to the database")
	}

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
	return NewPostgresIdentity(username, p.conn, p.accrual)
}

func (p *PostgresIdentityDatabase) Run(ctx context.Context) error {
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
	<-ctx.Done()
	return p.conn.Close(context.Background())
}
