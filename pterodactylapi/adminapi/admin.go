package adminapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/marte26/pterodactylBackup/pterodactylapi/structs"
)

type Client struct {
	URL    string
	APIKey string
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func (c *Client) httpRequest(method string, path string) ([]byte, error) {
	adminPath := "/api/application"

	req, err := http.NewRequest(method, c.URL+adminPath+path, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %d: %s", resp.StatusCode, body)
	}

	return body, nil
}

func (c *Client) GetServers() ([]structs.Server, error) {
	var servers []structs.Server

	for page := 1; ; page++ {
		body, err := c.httpRequest("GET", "/servers?sort=id&page="+strconv.Itoa(page))
		if err != nil {
			return nil, err
		}

		var response structs.Response[structs.Server]

		err = json.Unmarshal(body, &response)
		if err != nil {
			return nil, err
		}

		servers = append(servers, response.Data...)

		if page >= response.Meta.Pagination.TotalPages {
			break
		}
	}

	return servers, nil
}
