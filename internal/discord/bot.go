package discord

import (
	"context"
	"log"

	"osrs-events/internal/database"
	"osrs-events/internal/discord/services"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	Session            *discordgo.Session
	Store              database.Store
	GuildService       *services.GuildService
	AccountService     *services.AccountService
	InitializerService *services.InitializerService
	Handlers           map[string]Command
}

func New(token string, store database.Store) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	bot := &Bot{
		Session:        dg,
		Store:          store,
		GuildService:   services.NewGuildService(store),
		AccountService: services.NewAccountService(store),
	}

	bot.InitializerService = services.NewInitializerService(dg, store)

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
}

func (b *Bot) interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type == discordgo.InteractionApplicationCommand {
		if cmd, ok := b.Handlers[i.ApplicationCommandData().Name]; ok {
			cmd.Handler(s, i)
		}
	}
}

func (b *Bot) guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	log.Printf("Guild event received: %s (%s)", event.Guild.Name, event.Guild.ID)
	go b.initializeGuildAsync(event.Guild.ID)
}

func (b *Bot) initializeGuildAsync(guildID string) {
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
