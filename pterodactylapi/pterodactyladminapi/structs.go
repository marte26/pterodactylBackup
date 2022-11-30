package pterodactyladminapi

import "time"

type Response struct {
	Object string           `json:"object"`
	Data   []map[string]any `json:"data"`
	Meta   Meta             `json:"meta"`
}

type Meta struct {
	Pagination struct {
		Total       int `json:"total"`
		Count       int `json:"count"`
		PerPage     int `json:"per_page"`
		CurrentPage int `json:"current_page"`
		TotalPages  int `json:"total_pages"`
	} `json:"pagination"`
}

type Server struct {
	Attributes struct {
		ID          int    `json:"id"`
		UUID        string `json:"uuid"`
		Identifier  string `json:"identifier"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Suspended   bool   `json:"suspended"`
		Limits      struct {
			Memory      int  `json:"memory"`
			Swap        int  `json:"swap"`
			Disk        int  `json:"disk"`
			Io          int  `json:"io"`
			CPU         int  `json:"cpu"`
			Threads     int  `json:"threads"`
			OomDisabled bool `json:"oom_disabled"`
		} `json:"limits"`
		FeatureLimits struct {
			Databases   int `json:"databases"`
			Allocations int `json:"allocations"`
			Backups     int `json:"backups"`
		} `json:"feature_limits"`
		User       int       `json:"user"`
		Node       int       `json:"node"`
		Allocation int       `json:"allocation"`
		Nest       int       `json:"nest"`
		Egg        int       `json:"egg"`
		UpdatedAt  time.Time `json:"updated_at"`
		CreatedAt  time.Time `json:"created_at"`
	} `json:"attributes"`
}
