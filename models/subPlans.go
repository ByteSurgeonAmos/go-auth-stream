package models

import (
	"time"
)
type Interval string

const (
    IntervalMonthly Interval = "monthly"
    IntervalAnnual  Interval = "annually"
	IntervalFree   Interval = "free"
)

type Plan struct {
    ID          string    `bson:"_id" json:"id"`
    Name        string    `bson:"name" json:"name"`
    PlanCode    string    `bson:"plan_code" json:"plan_code"`
    Description string    `bson:"description" json:"description,omitempty"`
    Amount      int       `bson:"amount" json:"amount"` // Amount in cents (e.g., 1000 = $10.00)
    Interval    Interval  `bson:"interval" json:"interval"`
    Currency    string    `bson:"currency" json:"currency"` // Default: "USD"
    CreatedAt   time.Time `bson:"created_at" json:"createdAt"`
    UpdatedAt   time.Time `bson:"updated_at" json:"updatedAt"`
}
