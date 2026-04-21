package repositories

type usageRepository struct{}

func NewUsageRepository() *usageRepository {
	return &usageRepository{}
}

