package repositories


type planRepository struct {
	// DB connection
}

func NewPlanRepository() *planRepository{
	return &planRepository{}
}
