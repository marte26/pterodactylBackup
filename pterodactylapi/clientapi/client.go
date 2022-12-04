package clientapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/marte26/pterodactylBackup/pterodactylapi/structs"
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf(string(body))
	}

	return body, nil
}

func getData(body []byte) ([]byte, error) {
	var response structs.Response

	err := json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
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

func (c *Client) CreateBackup(serverIdentifier string) (structs.Backup, error) {
	var backup structs.Backup

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

func (c *Client) DeleteBackup(serverIdentifier string, backupUUID string) error {
	_, err := c.httpRequest("DELETE", "/servers/"+serverIdentifier+"/backups/"+backupUUID)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetBackupDetails(serverIdentifier string, backupUUID string) (structs.Backup, error) {
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
