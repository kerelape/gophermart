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
		if duplicateResult.Err() != nil {
			return ErrOrderDuplicate
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
	result, queryError := p.db.QueryContext(ctx, `SELECT (id, status, time, accrual) FROM orders WHERE owner = $1`, p.username)
	if queryError != nil {
		return nil, queryError
	}
	defer result.Close()
	if result.Err() != nil {
		return nil, result.Err()
	}

	orders := make([]Order, 0)
	for result.Next() {
		order := Order{}
		var status string
		var orderTime int64
		if err := result.Scan(&order.ID, &status, &orderTime, &order.Accrual); err != nil {
			return nil, err
		}
		order.Status = OrderStatus(status)
		order.Time = time.UnixMilli(orderTime)
		orders = append(orders, order)
	}

	return orders, nil
}

func (p PostgresIdentity) Balance(ctx context.Context) (Balance, error) {
	orders, ordersError := p.Orders(ctx)
	if ordersError != nil {
		return Balance{}, ordersError
	}

	withdrawals, withdrawalsError := p.Withdrawals(ctx)
	if withdrawalsError != nil {
		return Balance{}, withdrawalsError
	}

	balance := Balance{}
	for _, order := range orders {
		if order.Status == OrderStatusProcessed {
			balance.Current += order.Accrual
		}
	}
	for _, withdrawal := range withdrawals {
		balance.Current -= withdrawal.Sum
		balance.Withdrawn += withdrawal.Sum
	}

	return balance, nil
}

func (p PostgresIdentity) Withdraw(ctx context.Context, order string, amount float64) error {
	balance, balanceError := p.Balance(ctx)
	if balanceError != nil {
		return balanceError
	}
	if balance.Current < amount {
		return ErrBalanceTooLow
	}

	_, orderInfoError := p.accrual.OrderInfo(ctx, order)
	if orderInfoError != nil {
		return ErrOrderInvalid
	}

	_, execError := p.db.ExecContext(
		ctx,
		`INSERT INTO withdrawals(order, sum, time, owner) VALUES($1, $2, $3, $4)`,
		order,
		amount,
		time.Now().UnixMilli(),
		p.username,
	)
	return execError
}

func (p PostgresIdentity) Withdrawals(ctx context.Context) ([]Withdrawal, error) {
	result, queryError := p.db.QueryContext(ctx, `SELECT (orderID, sum, time) FROM withdrawals WHERE owner = $1`, p.username)
	if queryError != nil {
		return nil, queryError
	}
	defer result.Close()

	withdrawals := make([]Withdrawal, 0)
	for result.Next() {
		withdrawal := Withdrawal{}
		var withdrawalTime int64
		if err := result.Scan(&withdrawal.Order, &withdrawal.Sum, &withdrawalTime); err != nil {
			return nil, err
		}
		withdrawal.Time = time.UnixMilli(withdrawalTime)
		withdrawals = append(withdrawals, withdrawal)
	}

	return withdrawals, nil
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
