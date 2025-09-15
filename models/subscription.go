package models

type Status string

const (
	StatusActive   Status = "active"
	StatusCanceled Status = "canceled"
	StatusPastDue  Status = "past_due"
)

type Subscription struct {
	ID string `bson:"_id" json:"id"`
	UserID string `bson:"user_id" json:"user_id"`
	PlanID string `bson:"plan_id" json:"plan_id"`
	Status Status `bson:"status" json:"status"`
	StartDate string `bson:"start_date" json:"start_date"`
	EndDate string `bson:"end_date" json:"end_date"`
}