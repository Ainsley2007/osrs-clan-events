package firebase

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/remoteconfig"
)

// RemoteConfigClient manages Firebase Remote Config operations for OSRS configurations.
//
// Setup Instructions:
// 1. Go to https://console.firebase.google.com/
// 2. Select your project (or create one)
// 3. Click gear icon > Project Settings > Service Accounts tab
// 4. Click "Generate New Private Key" and save JSON file
// 5. Add to .env: FIREBASE_CREDENTIALS=firebase-credentials.json
// 6. Enable Remote Config in Firebase Console left sidebar
// 7. Ensure parameters exist: osrs_bosses, osrs_skills
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
		return nil, fmt.Errorf("failed to fetch remote config template: %w", err)
	}

	config, err := template.Evaluate(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate remote config template: %w", err)
	}

	bossesJSON := config.GetString("osrs_bosses")
	if bossesJSON == "" {
		return nil, &ConfigNotFoundError{ParameterName: "osrs_bosses"}
	}

	skillsJSON := config.GetString("osrs_skills")
	if skillsJSON == "" {
		return nil, &ConfigNotFoundError{ParameterName: "osrs_skills"}
	}

	var bosses []BossConfig
	if err := json.Unmarshal([]byte(bossesJSON), &bosses); err != nil {
		return nil, &ConfigParseError{
			ParameterName: "osrs_bosses",
			Err:           err,
		}
	}

	var skills []SkillConfig
	if err := json.Unmarshal([]byte(skillsJSON), &skills); err != nil {
		return nil, &ConfigParseError{
			ParameterName: "osrs_skills",
			Err:           err,
		}
	}

	return &OSRSConfig{
		Bosses: bosses,
		Skills: skills,
	}, nil
}

func (rc *RemoteConfigClient) GetRandomBoss(ctx context.Context) (*BossConfig, error) {
	config, err := rc.FetchOSRSConfig(ctx)
	if err != nil {
		return nil, err
	}

	if len(config.Bosses) == 0 {
		return nil, fmt.Errorf("no bosses configured in remote config")
	}

	randomIndex := rand.Intn(len(config.Bosses))
	return &config.Bosses[randomIndex], nil
}

func (rc *RemoteConfigClient) GetRandomSkill(ctx context.Context) (*SkillConfig, error) {
	config, err := rc.FetchOSRSConfig(ctx)
	if err != nil {
		return nil, err
	}

	if len(config.Skills) == 0 {
		return nil, fmt.Errorf("no skills configured in remote config")
	}

	randomIndex := rand.Intn(len(config.Skills))
	return &config.Skills[randomIndex], nil
}
