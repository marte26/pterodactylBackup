package clientapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/marte26/pterodactylBackup/pterodactylapi/structs"
)

type Client struct {
	URL    string
	APIKey string
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// ErrNoRoom is returned by PruneBackups when backups it may not delete alone exceed keep
var ErrNoRoom = errors.New("not enough deletable backups")

// HTTPError is returned when the panel responds with an unexpected status code
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("unexpected HTTP status %d: %s", e.StatusCode, e.Body)
}

func (c *Client) httpRequest(method string, path string) ([]byte, error) {
	clientPath := "/api/client"

	req, err := http.NewRequest(method, c.URL+clientPath+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTTP response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	return body, nil
}

func (c *Client) GetFiles(serverID string, path string) ([]structs.File, error) {
	body, err := c.httpRequest("GET", "/servers/"+serverID+"/files/list?directory="+url.QueryEscape(path))
	if err != nil {
		return nil, err
	}

	var response structs.Response[structs.File]

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	return response.Data, nil
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
	var backups []structs.Backup

	for page := 1; ; page++ {
		body, err := c.httpRequest("GET", "/servers/"+serverIdentifier+"/backups?per_page=50&page="+strconv.Itoa(page))
		if err != nil {
			return nil, err
		}

		var response structs.Response[structs.Backup]

		err = json.Unmarshal(body, &response)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON response: %w", err)
		}

		backups = append(backups, response.Data...)

		if page >= response.Meta.Pagination.TotalPages {
			break
		}
	}

	return backups, nil
}

// PruneBackups deletes the oldest unlocked successful backups until at most keep
// backups count towards the server's backup limit. The backup with UUID exclude is
// never deleted. If keep cannot be reached it deletes nothing and returns ErrNoRoom.
func (c *Client) PruneBackups(serverIdentifier string, keep int, exclude string) error {
	backups, err := c.GetBackups(serverIdentifier)
	if err != nil {
		return err
	}

	// the panel counts successful and in progress backups towards the limit
	var counted int
	var deletable []structs.Backup

	for _, backup := range backups {
		if backup.Attributes.IsSuccessful || backup.Attributes.CompletedAt.IsZero() {
			counted++
		}

		if backup.Attributes.IsSuccessful && !backup.Attributes.IsLocked && backup.Attributes.UUID != exclude {
			deletable = append(deletable, backup)
		}
	}

	// don't delete anything if it cannot free enough room anyway
	if counted-len(deletable) > keep {
		return fmt.Errorf("%w: %d locked or in progress backups, %d allowed", ErrNoRoom, counted-len(deletable), keep)
	}

	slices.SortFunc(deletable, func(a, b structs.Backup) int {
		return a.Attributes.CreatedAt.Compare(b.Attributes.CreatedAt)
	})

	for _, backup := range deletable {
		if counted <= keep {
			break
		}

		err := c.DeleteBackup(serverIdentifier, backup.Attributes.UUID)
		if err != nil {
			return fmt.Errorf("failed to delete backup %v: %w", backup.Attributes.UUID, err)
		}

		counted--
	}

	return nil
}
