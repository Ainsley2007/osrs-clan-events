package discord

import (
	"context"
	"log"
	"sync"

	"osrs-events/internal/database"
	"osrs-events/internal/discord/services"
	"osrs-events/internal/firebase"
	"osrs-events/internal/osrs"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	Session            *discordgo.Session
	Store              database.Store
	GuildService       *services.GuildService
	AccountService     *services.AccountService
	InitializerService *services.InitializerService
	EventService       *services.EventService
	SnapshotService    *services.SnapshotService
	LeaderboardService *services.LeaderboardService
	ParticipantService *services.ParticipantService
	DonationService    *services.DonationService
	Handlers           map[string]Command

	mu             sync.Mutex
	initInProgress map[string]bool
	logger         *log.Logger
}

func New(token string, store database.Store, osrsClient *osrs.Client, firebaseClient *firebase.RemoteConfigClient) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	logger := log.Default()
	snapshotService := services.NewSnapshotService(store, osrsClient)
	eventService := services.NewEventService(store, snapshotService, firebaseClient)
	leaderboardService := services.NewLeaderboardService(store, dg, logger)

	guildService := services.NewGuildService(store)
	accountService := services.NewAccountService(store, snapshotService, leaderboardService, logger)
	participantService := services.NewParticipantService(store)
	initializerService := services.NewInitializerService(dg, store, leaderboardService)
	donationService := services.NewDonationService(store, dg, logger)

	bot := &Bot{
		Session:            dg,
		Store:              store,
		GuildService:       guildService,
		AccountService:     accountService,
		InitializerService: initializerService,
		EventService:       eventService,
		SnapshotService:    snapshotService,
		LeaderboardService: leaderboardService,
		ParticipantService: participantService,
		DonationService:    donationService,
		initInProgress:     make(map[string]bool),
		logger:             logger,
	}

	bot.setupCommands()

	bot.Session.AddHandler(bot.ready)
	bot.Session.AddHandler(bot.interactionCreate)
	bot.Session.AddHandler(bot.guildCreate)

	return bot, nil
}

func (b *Bot) RegisterCommands(guildID string) error {
	var commands []*discordgo.ApplicationCommand
	for _, cmd := range b.Handlers {
		commands = append(commands, cmd.Definition)
	}

	_, err := b.Session.ApplicationCommandBulkOverwrite(b.Session.State.User.ID, guildID, commands)
	return err
}

func (b *Bot) Start() error {
	return b.Session.Open()
}

func (b *Bot) Stop() error {
	return b.Session.Close()
}

func (b *Bot) ready(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	// Initialize all guilds on connect (handles restart, reconnect); guildCreate handles new guilds
	go b.InitializeAllGuilds()
}

func (b *Bot) interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		if cmd, ok := b.Handlers[i.ApplicationCommandData().Name]; ok {
			cmd.Handler(s, i)
		}
	case discordgo.InteractionApplicationCommandAutocomplete:
		b.handleAutocomplete(s, i)
	}
}

func (b *Bot) handleAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	switch data.Name {
	case "remove", "rename":
		b.handleRSNAutocomplete(s, i)
	}
}

func (b *Bot) handleRSNAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()

	userID := i.Member.User.ID
	if userID == "" && i.User != nil {
		userID = i.User.ID
	}

	accounts, err := b.AccountService.GetTrackedAccounts(ctx, userID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Choices: []*discordgo.ApplicationCommandOptionChoice{},
			},
		})
		return
	}

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(accounts))
	for _, acc := range accounts {
		if len(choices) >= 25 {
			break
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  acc.RSN,
			Value: acc.RSN,
		})
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
}

func (b *Bot) guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	log.Printf("Guild event received: %s (%s)", event.Guild.Name, event.Guild.ID)
	go b.initializeGuildAsync(event.Guild.ID)
}

func (b *Bot) initializeGuildAsync(guildID string) {
	b.mu.Lock()
	if b.initInProgress[guildID] {
		log.Printf("[Guild %s] Initialization already in progress, skipping", guildID)
		b.mu.Unlock()
		return
	}
	b.initInProgress[guildID] = true
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		delete(b.initInProgress, guildID)
		b.mu.Unlock()
	}()

	ctx := context.Background()
	if err := b.InitializerService.InitializeGuild(ctx, guildID); err != nil {
		log.Printf("Failed to initialize guild %s: %v", guildID, err)
	}
}

func (b *Bot) InitializeAllGuilds() {
	log.Println("Initializing all guilds...")
	for _, guild := range b.Session.State.Guilds {
		go b.initializeGuildAsync(guild.ID)
	}
}
