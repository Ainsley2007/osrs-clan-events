package firebase

import (
	"context"

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

	app, err := firebase.NewApp(ctx, nil, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{App: app}, nil
}

func (c *Client) RemoteConfig(ctx context.Context) (*RemoteConfigClient, error) {
	return NewRemoteConfigClient(ctx, c.App)
}
