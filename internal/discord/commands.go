package discord

import "github.com/bwmarrin/discordgo"

type Command struct {
	Definition *discordgo.ApplicationCommand
	Handler    func(s *discordgo.Session, i *discordgo.InteractionCreate)
}

func (b *Bot) setupCommands() {
	b.Handlers = make(map[string]Command)

	// Register commands here
	// To add a new command:
	// 1. Create a new file (e.g. cmd_profile.go)
	// 2. Define a method on Bot that returns a Command
	// 3. Call b.addCommand(b.profileCommand()) here
	b.addCommand(b.setupLoggingChannelCommand())
}

func (b *Bot) addCommand(cmd Command) {
	b.Handlers[cmd.Definition.Name] = cmd
}
