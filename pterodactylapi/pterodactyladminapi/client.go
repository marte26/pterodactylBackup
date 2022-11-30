package pterodactyladminapi

import (
	"encoding/json"
	"io"
	"net/http"
)

type Client struct {
	URL    string
	APIKey string
}

func (c *Client) addHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
}

func (c *Client) httpRequest(method string, path string) (*http.Request, error) {
	adminPath := "/api/application"

	return http.NewRequest(method, c.URL+adminPath+path, nil)
}

func (c *Client) GetServers() ([]Server, error) {
	req, err := c.httpRequest("GET", "/servers")
	if err != nil {
		return nil, err
	}

	c.addHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response Response

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	var servers []Server

	temp, err := json.Marshal(response.Data)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(temp, &servers)
	if err != nil {
		return nil, err
	}

	_ = resp.Close

	return servers, nil
}
