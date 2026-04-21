package repositories

type subscriptionRepository struct{}

func NewSubscriptionRepository() *subscriptionRepository {
	return &subscriptionRepository{}
}
