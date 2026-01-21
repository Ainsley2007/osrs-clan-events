package osrs

type PlayerStats struct {
	Name       string     `json:"name"`
	Skills     []Skill    `json:"skills"`
	Activities []Activity `json:"activities"`
}

type Skill struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Rank  int    `json:"rank"`
	Level int    `json:"level"`
	XP    int64  `json:"xp"`
}

type Activity struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Rank  int    `json:"rank"`
	Score int    `json:"score"`
}
