package adminapi

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/marte26/pterodactylBackup/pterodactylapi/structs"
)

type Client struct {
	URL    string
	APIKey string
}

func (c *Client) httpRequest(method string, path string) ([]byte, error) {
	adminPath := "/api/application"

	req, err := http.NewRequest(method, c.URL+adminPath+path, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (c *Client) GetServers() ([]structs.Server, error) {
	body, err := c.httpRequest("GET", "/servers")
	if err != nil {
		return nil, err
	}

	var response structs.Response

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	var servers []structs.Server

	temp, err := json.Marshal(response.Data)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(temp, &servers)
	if err != nil {
		return nil, err
	}

	return servers, nil
}
