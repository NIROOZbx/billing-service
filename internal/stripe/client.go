package stripe

type Client struct {
	ApiKey string
}

func NewClient(apiKey string) *Client {
	return &Client{
		ApiKey: apiKey,
	}
}

// Add Stripe wrapper methods here
func (c *Client) CreateCustomer() error {
	return nil
}
