package firebase

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	firebase "firebase.google.com/go/v4"
)

type Client struct {
	App *firebase.App
}

func New(ctx context.Context, credentialsFile string) (*Client, error) {
	projectID, err := extractProjectIDFromFile(credentialsFile)
	if err != nil {
		return nil, err
	}

	config := &firebase.Config{
		ProjectID: projectID,
	}

	app, err := firebase.NewApp(ctx, config)
	if err != nil {
		return nil, err
	}

	return &Client{App: app}, nil
}

func extractProjectIDFromFile(credentialsFile string) (string, error) {
	if credentialsFile == "" {
		return "", fmt.Errorf("credentials file not provided")
	}

	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return "", fmt.Errorf("failed to read credentials file: %w", err)
	}

	var creds struct {
		ProjectID string `json:"project_id"`
	}

	if err := json.Unmarshal(data, &creds); err != nil {
		return "", fmt.Errorf("failed to parse credentials file: %w", err)
	}

	if creds.ProjectID == "" {
		return "", fmt.Errorf("project_id not found in credentials file")
	}

	return creds.ProjectID, nil
}

func (c *Client) RemoteConfig(ctx context.Context) (*RemoteConfigClient, error) {
	return NewRemoteConfigClient(ctx, c.App)
}
