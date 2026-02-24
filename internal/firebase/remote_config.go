package firebase

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/remoteconfig"
)

type RemoteConfigClient struct {
	client *remoteconfig.Client
}

func NewRemoteConfigClient(ctx context.Context, app *firebase.App) (*RemoteConfigClient, error) {
	client, err := app.RemoteConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize remote config client: %w", err)
	}

	return &RemoteConfigClient{
		client: client,
	}, nil
}

func (rc *RemoteConfigClient) FetchOSRSConfig(ctx context.Context) (*OSRSConfig, error) {
	template, err := rc.client.GetServerTemplate(ctx, nil)
	if err != nil {
		log.Printf("Firebase Remote Config: fetch failed: %v", err)
		return nil, fmt.Errorf("failed to fetch remote config template: %w", err)
	}

	config, err := template.Evaluate(nil)
	if err != nil {
		log.Printf("Firebase Remote Config: fetch failed: %v", err)
		return nil, fmt.Errorf("failed to evaluate remote config template: %w", err)
	}

	bossesJSON := config.GetString("osrs_bosses")
	if bossesJSON == "" {
		err := &ConfigNotFoundError{ParameterName: "osrs_bosses"}
		log.Printf("Firebase Remote Config: fetch failed: %v", err)
		return nil, err
	}

	skillsJSON := config.GetString("osrs_skills")
	if skillsJSON == "" {
		err := &ConfigNotFoundError{ParameterName: "osrs_skills"}
		log.Printf("Firebase Remote Config: fetch failed: %v", err)
		return nil, err
	}

	var bosses []BossConfig
	if err := json.Unmarshal([]byte(bossesJSON), &bosses); err != nil {
		cfgErr := &ConfigParseError{
			ParameterName: "osrs_bosses",
			Err:           err,
		}
		log.Printf("Firebase Remote Config: fetch failed: %v", cfgErr)
		return nil, cfgErr
	}

	var skills []SkillConfig
	if err := json.Unmarshal([]byte(skillsJSON), &skills); err != nil {
		cfgErr := &ConfigParseError{
			ParameterName: "osrs_skills",
			Err:           err,
		}
		log.Printf("Firebase Remote Config: fetch failed: %v", cfgErr)
		return nil, cfgErr
	}

	log.Println("Firebase Remote Config: fetch success")
	return &OSRSConfig{
		Bosses: bosses,
		Skills: skills,
	}, nil
}

