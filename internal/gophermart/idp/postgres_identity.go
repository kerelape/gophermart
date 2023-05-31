package idp

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/kerelape/gophermart/internal/accrual"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
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
	duplicateResult, duplicateError := p.conn.Query(ctx, `SELECT owner FROM orders WHERE id = $1`, id)
	if duplicateError != nil {
		return duplicateError
	}
	if duplicateResult.Next() {
		var owner string
		if err := duplicateResult.Scan(&owner); err != nil {
			return err
		}
		if owner == p.username {
			return ErrOrderDuplicate
		} else {
			return ErrOrderUnowned
		}
	}
	duplicateResult.Close()
	if err := duplicateResult.Err(); err != nil {
		return err
	}

	order, orderError := p.accrual.OrderInfo(ctx, id)
	if orderError != nil {
		if errors.Is(orderError, accrual.ErrUnknownOrder) {
			return ErrOrderInvalid
		}
		return orderError
	}

	_, insertError := p.conn.Exec(
		ctx,
		`INSERT INTO orders(id, owner, status, time, accrual) VALUES($1, $2, $3, $4, $5)`,
		id,
		p.username,
		string(order.Status),
		time.Now().Unix(),
		order.Accrual,
	)
	return insertError
}

func (p PostgresIdentity) Orders(ctx context.Context) ([]Order, error) {
	if err := p.updateOrders(ctx); err != nil {
		return nil, err
	}

	result, queryError := p.conn.Query(ctx, `SELECT (id, status, time, accrual) FROM orders WHERE owner = $1`, p.username)
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

	_, execError := p.conn.Exec(
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
	result, queryError := p.conn.Query(ctx, `SELECT (orderID, sum, time) FROM withdrawals WHERE owner = $1`, p.username)
	if queryError != nil {
		return nil, queryError
	}
	defer result.Close()
	if result.Err() != nil {
		return []Withdrawal{}, result.Err()
	}

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

func (p PostgresIdentity) updateOrders(ctx context.Context) error {
	rows, queryError := p.conn.Query(
		ctx,
		`SELECT id FROM orders WHERE owner = $1 AND status IN ($1, $2)`,
		OrderStatusNew,
		OrderStatusProcessing,
	)
	if queryError != nil {
		return queryError
	}
	ordersToUpdate := make([]string, 0)
	for rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		var orderID string
		if err := rows.Scan(&orderID); err != nil {
			return err
		}
		ordersToUpdate = append(ordersToUpdate, orderID)
	}
	rows.Close()

	wg, wgContext := errgroup.WithContext(ctx)
	wg.SetLimit(len(ordersToUpdate))
	for _, orderID := range ordersToUpdate {
		wg.Go(func(id string) func() error {
			return func() error {
				orderInfo, err := p.accrual.OrderInfo(wgContext, id)
				if err != nil {
					return err
				}
				_, execError := p.conn.Exec(
					wgContext,
					`UPDATE orders SET status = $1 WHERE id = $2`,
					string(orderInfo.Status),
					id,
				)
				return execError
			}
		}(orderID))
	}
	return wg.Wait()
}
