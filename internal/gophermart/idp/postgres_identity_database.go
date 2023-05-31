package idp

import (
	"context"
	"database/sql"
	"encoding/base64"
	"github.com/kerelape/gophermart/internal/accrual"
	"golang.org/x/crypto/bcrypt"

	_ "github.com/lib/pq"
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
