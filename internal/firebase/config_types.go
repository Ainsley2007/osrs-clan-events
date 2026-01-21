package firebase

import "fmt"

type BossConfig struct {
	Name          string   `json:"name"`
	BossesToTrack []string `json:"bosses_to_track"`
	PointsPerKC   float64  `json:"points_per_kc"`
	ThresholdKC   int      `json:"threshold_kc"`
}

type SkillConfig struct {
	Name        string  `json:"name"`
	XPThreshold int     `json:"xp_threshold"`
	PointsPerXP float64 `json:"points_per_xp"`
}

type OSRSConfig struct {
	Bosses []BossConfig
	Skills []SkillConfig
}

type ConfigNotFoundError struct {
	ParameterName string
}

func (e *ConfigNotFoundError) Error() string {
	return fmt.Sprintf("config parameter not found: %s", e.ParameterName)
}

type ConfigParseError struct {
	ParameterName string
	Err           error
}

func (e *ConfigParseError) Error() string {
	return fmt.Sprintf("failed to parse config parameter %s: %v", e.ParameterName, e.Err)
}
