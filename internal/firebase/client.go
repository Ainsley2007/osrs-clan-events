package firebase

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

type Client struct {
	App *firebase.App
}

func New(ctx context.Context, credentialsFile string) (*Client, error) {
	var opts []option.ClientOption
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}

	projectID, err := extractProjectID(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to extract project ID: %w", err)
	}

	config := &firebase.Config{
		ProjectID: projectID,
	}

	app, err := firebase.NewApp(ctx, config, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{App: app}, nil
}

func extractProjectID(credentialsFile string) (string, error) {
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
