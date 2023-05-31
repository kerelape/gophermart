package idp

import (
	"context"
	"github.com/kerelape/gophermart/internal/accrual"
)

type UpdateOrdersTask struct {
	accrual  accrual.Accrual
	database IdentityDatabase
}

func NewUpdateOrdersTask(accrual accrual.Accrual, database IdentityDatabase) UpdateOrdersTask {
	return UpdateOrdersTask{
		accrual:  accrual,
		database: database,
	}
}

func (u UpdateOrdersTask) Run(ctx context.Context) error {
	unupdatedOrders, ordersError := u.database.Orders(ctx, OrderStatusNew, OrderStatusProcessing)
	if ordersError != nil {
		return ordersError
	}

	for _, unupdatedOrder := range unupdatedOrders {
		updatedOrder, updatedOrderError := u.accrual.OrderInfo(ctx, unupdatedOrder.ID)
		if updatedOrderError != nil {
			return updatedOrderError
		}
		id := updatedOrder.Order
		status := MakeOrderStatus(updatedOrder.Status)
		accrual := updatedOrder.Accrual
		if err := u.database.UpdateOrder(ctx, id, status, accrual); err != nil {
			return err
		}
	}
	return nil
}
