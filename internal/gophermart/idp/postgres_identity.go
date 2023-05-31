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
	if err := goluhn.Validate(id); err != nil {
		return ErrOrderInvalid
	}

	duplicateRow := p.conn.QueryRow(ctx, `SELECT owner FROM orders WHERE id = $1`, id)
	var owner string
	scanDuplicateError := duplicateRow.Scan(&owner)
	if scanDuplicateError == nil {
		if owner == p.username {
			return ErrOrderDuplicate
		} else {
			return ErrOrderUnowned
		}
	} else if !errors.Is(scanDuplicateError, pgx.ErrNoRows) {
		return scanDuplicateError
	}

	_, insertError := p.conn.Exec(
		ctx,
		`INSERT INTO orders VALUES($1, $2, $3, $4, $5)`,
		id,
		p.username,
		time.Now().Unix(),
		string(OrderStatusNew),
		0.0,
	)

	return insertError
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

	for i, order := range orders {
		if order.Status.IsFinal() {
			continue
		}
		orderInfo, orderInfoError := p.accrual.OrderInfo(ctx, order.ID)
		if orderInfoError != nil {
			if errors.Is(orderInfoError, accrual.ErrUnknownOrder) {
				orders[i].Status = OrderStatusInvalid
			} else {
				return nil, orderInfoError
			}
		} else {
			orders[i].Status = MakeOrderStatus(orderInfo.Status)
		}
		orders[i].Accrual = orderInfo.Accrual
		_, err := p.conn.Exec(
			ctx,
			`UPDATE orders SET status = $1, accrual = $2 WHERE id = $3`,
			string(orders[i].Status),
			orderInfo.Accrual,
			orderInfo.Order,
		)
		if err != nil {
			return nil, err
		}
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
