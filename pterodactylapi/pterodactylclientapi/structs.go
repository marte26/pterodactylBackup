package pterodactylclientapi

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
