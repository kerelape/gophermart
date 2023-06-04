package accrual

import "time"

type TooManyRequestsError struct {
	RetryAfter time.Duration
}

func (t TooManyRequestsError) Error() string {
	return "too many requests"
}
