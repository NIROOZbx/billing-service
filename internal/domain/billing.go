package domain

import "time"

type Plan struct {
	ID        string
	Name      string
	Price     int64
	Currency  string
	CreatedAt time.Time
}

type Subscription struct {
	ID        string
	PlanID    string
	UserID    string
	Status    string
	CreatedAt time.Time
}

type Usage struct {
	ID        string
	UserID    string
	Amount    int64
	CreatedAt time.Time
}

