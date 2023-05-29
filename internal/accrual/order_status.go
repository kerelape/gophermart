package accrual

type OrderStatus string

const (
	OrderStatusRegistered = OrderStatus("REGISTERED")
	OrderStatusInvalid    = OrderStatus("INVALID")
	OrderStatusProcessing = OrderStatus("PROCESSING")
	OrderStatusProcessed  = OrderStatus("PROCESSED")
)
