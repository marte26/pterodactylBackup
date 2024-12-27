package clientapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/marte26/pterodactylBackup/pterodactylapi/structs"
)

type Client struct {
	URL    string
	APIKey string
}

var clientPool = sync.Pool{
	New: func() interface{} {
		return &http.Client{}
	},
}

func (c *Client) httpRequest(method string, path string) ([]byte, error) {
	clientPath := "/api/client"

	client := clientPool.Get().(*http.Client)

	req, err := http.NewRequest(method, c.URL+clientPath+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTTP response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf(string(body))
	}

	clientPool.Put(client)

	return body, nil
}

func getData(body []byte) ([]byte, error) {
	var response structs.Response

	err := json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	data, err := json.Marshal(response.Data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (c *Client) GetFiles(serverID string, path string) ([]structs.File, error) {
	body, err := c.httpRequest("GET", "/servers/"+serverID+"/files/list?directory="+url.QueryEscape(path))
	if err != nil {
		return nil, err
	}

	data, err := getData(body)
	if err != nil {
		return nil, err
	}

	var files []structs.File

	err = json.Unmarshal(data, &files)
	if err != nil {
		return nil, err
	}

	return files, nil
}

func (c *Client) CreateBackup(serverIdentifier string, purgeBackups bool) (structs.Backup, error) {
	var backup structs.Backup

	if purgeBackups {
		server, err := c.GetServer(serverIdentifier)
		if err != nil {
			return backup, err
		}

		backups, err := c.GetBackups(serverIdentifier)
		if err != nil {
			return backup, err
		}

		if len(backups) >= server.Attributes.FeatureLimits.Backups {
			err := c.DeleteBackup(serverIdentifier, backups[0].Attributes.UUID)
			if err != nil {
				return backup, err
			}
		}
	}

	body, err := c.httpRequest("POST", "/servers/"+serverIdentifier+"/backups")
	if err != nil {
		return backup, err
	}

	err = json.Unmarshal(body, &backup)
	if err != nil {
		return backup, err
	}

	return backup, nil
}

func (c *Client) GetServer(serverIdentifier string) (structs.Server, error) {
	var server structs.Server

	body, err := c.httpRequest("GET", "/servers/"+serverIdentifier)
	if err != nil {
		return server, err
	}

	err = json.Unmarshal(body, &server)
	if err != nil {
		return server, err
	}

	return server, nil
}

func (c *Client) DeleteBackup(serverIdentifier string, backupUUID string) error {
	_, err := c.httpRequest("DELETE", "/servers/"+serverIdentifier+"/backups/"+backupUUID)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetBackup(serverIdentifier string, backupUUID string) (structs.Backup, error) {
	var backup structs.Backup

	body, err := c.httpRequest("GET", "/servers/"+serverIdentifier+"/backups/"+backupUUID)
	if err != nil {
		return backup, err
	}

	err = json.Unmarshal(body, &backup)
	if err != nil {
		return backup, err
	}

	return backup, nil
}

func (c *Client) GetBackups(serverIdentifier string) ([]structs.Backup, error) {
	body, err := c.httpRequest("GET", "/servers/"+serverIdentifier+"/backups")
	if err != nil {
		return nil, err
	}

	data, err := getData(body)
	if err != nil {
		return nil, err
	}

	var backups []structs.Backup

	err = json.Unmarshal(data, &backups)
	if err != nil {
		return nil, err
	}

	return backups, nil
}
