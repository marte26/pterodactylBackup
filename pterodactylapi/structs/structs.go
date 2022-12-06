package structs

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
	Object     string `json:"object"`
	Attributes struct {
		ID          int    `json:"id"`
		ExternalID  int    `json:"external_id"`
		UUID        string `json:"uuid"`
		Identifier  string `json:"identifier"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      string `json:"status"`
		Suspended   bool   `json:"suspended"`
		Limits      struct {
			Memory      int  `json:"memory"`
			Swap        int  `json:"swap"`
			Disk        int  `json:"disk"`
			IO          int  `json:"io"`
			CPU         int  `json:"cpu"`
			Threads     int  `json:"threads"`
			OomDisabled bool `json:"oom_disabled"`
		} `json:"limits"`
		FeatureLimits struct {
			Databases   int `json:"databases"`
			Allocations int `json:"allocations"`
			Backups     int `json:"backups"`
		} `json:"feature_limits"`
		User       int `json:"user"`
		Node       int `json:"node"`
		Allocation int `json:"allocation"`
		Nest       int `json:"nest"`
		Egg        int `json:"egg"`
		Container  struct {
			StartupCommand string                 `json:"startup_command"`
			Image          string                 `json:"image"`
			Installed      int                    `json:"installed"`
			Environment    map[string]interface{} `json:"environment"`
		} `json:"container"`
		UpdatedAt time.Time `json:"updated_at"`
		CreatedAt time.Time `json:"created_at"`
	} `json:"attributes"`
}

type File struct {
	Attributes struct {
		Name       string    `json:"name"`
		Mode       string    `json:"mode"`
		ModeBits   string    `json:"mode_bits"`
		Size       int       `json:"size"`
		IsFile     bool      `json:"is_file"`
		IsSymlink  bool      `json:"is_symlink"`
		Mimetype   string    `json:"mimetype"`
		CreatedAt  time.Time `json:"created_at"`
		ModifiedAt time.Time `json:"modified_at"`
	} `json:"attributes"`
}

type Backup struct {
	Attributes struct {
		UUID         string    `json:"uuid"`
		Name         string    `json:"name"`
		IgnoredFiles []string  `json:"ignored_files"`
		Sha256Hash   string    `json:"sha256_hash"`
		Bytes        int       `json:"bytes"`
		CreatedAt    time.Time `json:"created_at"`
		CompletedAt  time.Time `json:"completed_at"`
	} `json:"attributes"`
}
