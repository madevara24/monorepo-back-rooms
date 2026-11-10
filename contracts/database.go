package contracts

// CreateDatabaseRequest is the POST /v1/databases request body, asking
// the agent to provision (or, if it already exists, confirm) a tenant
// container.
type CreateDatabaseRequest struct {
	ID       string `json:"id"`
	SizeGB   int    `json:"size_gb"`
	Password string `json:"password"`
}

// Database is the agent's view of a tenant container, returned by
// GET /v1/databases and POST /v1/databases.
type Database struct {
	ID     string `json:"id"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Status string `json:"status"`
}
