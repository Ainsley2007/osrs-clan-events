package discord

import (
	"log"
	"sync"

	"osrs-events/internal/database"
	"osrs-events/internal/discord/services"
	"osrs-events/internal/firebase"
	"osrs-events/internal/osrs"
	"osrs-events/internal/proofstorage"
	"osrs-events/internal/scheduler"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	session            *discordgo.Session
	guildService       *services.GuildService
	accountService     *services.AccountService
	initializerService *services.InitializerService
	eventService       *services.EventService
	snapshotService    *services.SnapshotService
	leaderboardService *services.LeaderboardService
	participantService *services.ParticipantService
	donationService    *services.DonationService
	pbService          *services.PBService
	statsService       *services.StatsService
	handlers           map[string]Command
	notifier           scheduler.Notifier

	mu             sync.Mutex
	initInProgress map[string]bool
	logger         services.Logger
}

func New(token string, store database.Store, osrsClient *osrs.Client, firebaseClient *firebase.RemoteConfigClient, proofStore proofstorage.Store, logger services.Logger) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger = log.Default()
	}
	snapshotService := services.NewSnapshotService(store, osrsClient, logger)
	eventService := services.NewEventService(store, snapshotService, firebaseClient, logger)
	leaderboardService := services.NewLeaderboardService(store, dg, logger)

	guildService := services.NewGuildService(store)
	accountService := services.NewAccountService(store, snapshotService, leaderboardService, logger)
	participantService := services.NewParticipantService(store)
	donationService := services.NewDonationService(store, dg, logger)
	pbService := services.NewPBService(store, proofStore, dg, logger)
	initializerService := services.NewInitializerService(dg, store, leaderboardService, pbService)
	statsService := services.NewStatsService(store)

	bot := &Bot{
		session:            dg,
		guildService:       guildService,
		accountService:     accountService,
		initializerService: initializerService,
		eventService:       eventService,
		snapshotService:    snapshotService,
		leaderboardService: leaderboardService,
		participantService: participantService,
		donationService:    donationService,
		pbService:          pbService,
		statsService:       statsService,
		notifier:           NewSessionNotifier(dg),
		initInProgress:     make(map[string]bool),
		logger:             logger,
	}

	bot.setupCommands()

	bot.session.AddHandler(bot.ready)
	bot.session.AddHandler(bot.interactionCreate)
	bot.session.AddHandler(bot.messageReactionAdd)
	bot.session.AddHandler(bot.guildCreate)
	bot.session.AddHandler(bot.guildDelete)

	return bot, nil
}

// Scheduler wires background jobs to the bot's services and Discord notifier.
func (b *Bot) Scheduler(store scheduler.Store) *scheduler.Scheduler {
	return scheduler.New(store, b.eventService, b.snapshotService, b.leaderboardService, b.initializerService, b.notifier)
}

func (b *Bot) RegisterCommands(guildID string) error {
	var commands []*discordgo.ApplicationCommand
	for _, cmd := range b.handlers {
		commands = append(commands, cmd.Definition)
	}

	_, err := b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, guildID, commands)
	return err
}

func (b *Bot) Start() error {
	return b.session.Open()
}

func (b *Bot) Stop() error {
	return b.session.Close()
}

func (b *Bot) ready(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	goSafe("guild-cleanup", func() { b.cleanupRemovedGuilds(event) })
	goSafe("initialize-all-guilds", b.initializeAllGuilds)
}

func (b *Bot) interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		cmdName := i.ApplicationCommandData().Name
		if cmd, ok := b.handlers[cmdName]; ok {
			// Log command used: guild (ID + name when in cache), user (ID + username)
			guildID := i.GuildID
			guildName := ""
			if g, err := s.State.Guild(guildID); err == nil && g != nil {
				guildName = g.Name
			}
			userID := ""
			username := ""
			if i.Member != nil && i.Member.User != nil {
				userID = i.Member.User.ID
				username = i.Member.User.Username
			} else if i.User != nil {
				userID = i.User.ID
				username = i.User.Username
			}
			if guildName != "" {
				b.logger.Printf("command=%s guild=%s (%s) user=%s (%s)", cmdName, guildID, guildName, userID, username)
			} else {
				b.logger.Printf("command=%s guild=%s user=%s (%s)", cmdName, guildID, userID, username)
			}
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
	case "submit-pb":
		b.handlePBCategoryAutocomplete(s, i)
	}
}

func (b *Bot) handleRSNAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := cmdContext()
	defer cancel()

	userID, ok := interactionActorID(i)
	if !ok {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Choices: []*discordgo.ApplicationCommandOptionChoice{},
			},
		})
		return
	}

	accounts, err := b.accountService.GetTrackedAccounts(ctx, userID)
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
	goSafe("guild-init", func() { b.initializeGuildAsync(event.Guild.ID) })
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

	ctx, cancel := guildInitContext()
	defer cancel()
	if err := b.initializerService.InitializeGuild(ctx, guildID); err != nil {
		log.Printf("Failed to initialize guild %s: %v", guildID, err)
	}
}

func (b *Bot) initializeAllGuilds() {
	log.Println("Initializing all guilds...")
	for _, guild := range b.session.State.Guilds {
		goSafe("guild-init", func() { b.initializeGuildAsync(guild.ID) })
	}
}
