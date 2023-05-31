package idp

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"github.com/kerelape/gophermart/internal/accrual"
	"golang.org/x/crypto/bcrypt"
	"time"
)

type PostgresIdentity struct {
	username string
	db       *sql.DB
	accrual  accrual.Accrual
}

// NewPostgresIdentity creates a new PostgresIdentity.
func NewPostgresIdentity(username string, db *sql.DB, accrual accrual.Accrual) PostgresIdentity {
	return PostgresIdentity{
		username: username,
		db:       db,
		accrual:  accrual,
	}
}

func (p PostgresIdentity) AddOrder(ctx context.Context, id string) error {
	order, orderError := p.accrual.OrderInfo(ctx, id)
	if orderError != nil {
		if errors.Is(orderError, accrual.ErrUnknownOrder) {
			return ErrOrderInvalid
		}
		return orderError
	}

	insertResult, insertError := p.db.ExecContext(
		ctx,
		`
		INSERT INTO orders(id, owner, status, time, accrual) VALUES($1, $2, $3, $4, $5) ON CONFLICT do nothing`,
		id,
		p.username,
		string(order.Status),
		time.Now().Unix(),
		order.Accrual,
	)
	if insertError != nil {
		return insertError
	}

	rowsAffected, rowsAffectedError := insertResult.RowsAffected()
	if rowsAffectedError != nil {
		return rowsAffectedError
	}
	if rowsAffected == 0 {
		duplicateResult, duplicateError := p.db.QueryContext(ctx, `SELECT owner FROM orders WHERE id = $1`, id)
		if duplicateError != nil {
			return duplicateError
		}

		var duplicateOwner string
		scanDuplicateError := duplicateResult.Scan(&duplicateOwner)
		if scanDuplicateError != nil {
			return scanDuplicateError
		}

		if err := duplicateResult.Close(); err != nil {
			return err
		}
		if duplicateOwner == p.username {
			return ErrOrderDuplicate
		}
		return ErrOrderUnowned
	}

	return nil
}

func (p PostgresIdentity) Orders(ctx context.Context) ([]Order, error) {
	//TODO implement me
	panic("implement me")
}

func (p PostgresIdentity) Balance(ctx context.Context) (Balance, error) {
	//TODO implement me
	panic("implement me")
}

func (p PostgresIdentity) Withdraw(ctx context.Context, order string, amount float64) error {
	//TODO implement me
	panic("implement me")
}

func (p PostgresIdentity) Withdrawals(ctx context.Context) ([]Withdrawal, error) {
	//TODO implement me
	panic("implement me")
}

func (p PostgresIdentity) ComparePassword(ctx context.Context, password string) (bool, error) {
	query := `SELECT password FROM identities WHERE username = $1`
	row := p.db.QueryRowContext(ctx, query, p.username)
	if row.Err() != nil {
		return false, row.Err()
	}

	var encodedPasswordHash string
	if err := row.Scan(&encodedPasswordHash); err != nil {
		return false, err
	}

	passwordHash, decodePasswordHashError := base64.StdEncoding.DecodeString(encodedPasswordHash)
	if decodePasswordHashError != nil {
		return false, decodePasswordHashError
	}

	return bcrypt.CompareHashAndPassword(passwordHash, []byte(password)) == nil, nil
}
