package pterodactylclientapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

type Client struct {
	URL    string
	APIKey string
}

func (c *Client) httpRequest(method string, path string) ([]byte, error) {
	clientPath := "/api/client"

	req, err := http.NewRequest(method, c.URL+clientPath+path, nil)
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

func (c *Client) GetFiles(ID string, path string) ([]File, error) {
	body, err := c.httpRequest("GET", "/servers/"+ID+"/files/list?directory="+url.QueryEscape(path))
	if err != nil {
		return nil, err
	}

	var response Response

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	var files []File

	temp, err := json.Marshal(response.Data)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(temp, &files)
	if err != nil {
		return nil, err
	}

	return files, nil
}
