package firebase

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"

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

func (rc *RemoteConfigClient) GetRandomBoss(ctx context.Context, excludeName string) (*BossConfig, error) {
	config, err := rc.FetchOSRSConfig(ctx)
	if err != nil {
		return nil, err
	}

	if len(config.Bosses) == 0 {
		return nil, fmt.Errorf("no bosses configured in remote config")
	}

	pool := config.Bosses
	if excludeName != "" {
		pool = make([]BossConfig, 0, len(config.Bosses))
		for _, b := range config.Bosses {
			if !strings.EqualFold(b.Name, excludeName) {
				pool = append(pool, b)
			}
		}
		if len(pool) == 0 {
			pool = config.Bosses
		}
	}

	return &pool[rand.Intn(len(pool))], nil
}

func (rc *RemoteConfigClient) GetRandomSkill(ctx context.Context, excludeName string) (*SkillConfig, error) {
	config, err := rc.FetchOSRSConfig(ctx)
	if err != nil {
		return nil, err
	}

	if len(config.Skills) == 0 {
		return nil, fmt.Errorf("no skills configured in remote config")
	}

	pool := config.Skills
	if excludeName != "" {
		pool = make([]SkillConfig, 0, len(config.Skills))
		for _, sk := range config.Skills {
			if !strings.EqualFold(sk.Name, excludeName) {
				pool = append(pool, sk)
			}
		}
		if len(pool) == 0 {
			pool = config.Skills
		}
	}

	return &pool[rand.Intn(len(pool))], nil
}
