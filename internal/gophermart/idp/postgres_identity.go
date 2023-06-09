package idp

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/ShiraazMoollatjie/goluhn"
	"github.com/jackc/pgx/v5"
	"github.com/kerelape/gophermart/internal/accrual"
	"golang.org/x/crypto/bcrypt"
	"time"
)

type PostgresIdentity struct {
	username string
	conn     *pgx.Conn
	accrual  accrual.Accrual
}

// NewPostgresIdentity creates a new PostgresIdentity.
func NewPostgresIdentity(username string, conn *pgx.Conn, accrual accrual.Accrual) PostgresIdentity {
	return PostgresIdentity{
		username: username,
		conn:     conn,
		accrual:  accrual,
	}
}

func (p PostgresIdentity) AddOrder(ctx context.Context, id string) error {
	order := Order{
		ID:      id,
		Time:    time.Now(),
		Accrual: 0.0,
		Status:  OrderStatusNew,
	}

	if err := goluhn.Validate(id); err != nil {
		return ErrOrderInvalid
	}

	_, insertError := p.conn.Exec(
		ctx,
		`INSERT INTO orders VALUES($1, $2, $3, $4, $5)`,
		order.ID,
		p.username,
		order.Time.UnixMilli(),
		string(order.Status),
		order.Accrual,
	)

	if insertError != nil {
		duplicateRow := p.conn.QueryRow(ctx, `SELECT owner FROM orders WHERE id = $1`, id)
		var owner string
		if err := duplicateRow.Scan(&owner); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return insertError
			}
			return err
		}
		if owner == p.username {
			return ErrOrderDuplicate
		} else {
			return ErrOrderUnowned
		}
	}
	return nil
}

func (p PostgresIdentity) Orders(ctx context.Context) ([]Order, error) {
	result, queryError := p.conn.Query(ctx, `SELECT id,status,time,accrual FROM orders WHERE owner = $1`, p.username)
	if queryError != nil {
		return nil, queryError
	}

	orders := make([]Order, 0)
	for result.Next() {
		if err := result.Err(); err != nil {
			return nil, err
		}

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
checkLock:
	lock := p.conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, 1)
	var locked bool
	if err := lock.Scan(&locked); err != nil {
		return err
	}
	if !locked {
		time.Sleep(time.Second)
		goto checkLock
	}
	defer p.conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, 1)

	balance, balanceError := p.Balance(ctx)
	if balanceError != nil {
		return balanceError
	}
	if balance.Current < amount {
		return ErrBalanceTooLow
	}

	_, execError := p.conn.Exec(
		ctx,
		`INSERT INTO withdrawals VALUES($1, $2, $3, $4)`,
		order,
		amount,
		time.Now().UnixMilli(),
		p.username,
	)
	return execError
}

func (p PostgresIdentity) Withdrawals(ctx context.Context) ([]Withdrawal, error) {
	result, queryError := p.conn.Query(ctx, `SELECT orderID,sum,time FROM withdrawals WHERE owner = $1`, p.username)
	if queryError != nil {
		return nil, queryError
	}
	defer result.Close()

	withdrawals := make([]Withdrawal, 0)
	for result.Next() {
		if err := result.Err(); err != nil {
			return nil, err
		}

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
	row := p.conn.QueryRow(ctx, `SELECT password FROM identities WHERE username = $1`, p.username)

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
