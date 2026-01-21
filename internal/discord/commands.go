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
	// 3. Call b.registerCommand(b.profileCommand()) here
	b.registerCommand(b.setupLoggingChannelCommand())
	b.registerCommand(b.exitCommand())
	b.registerCommand(b.addAccountCommand())
	b.registerCommand(b.removeCommand())
	b.registerCommand(b.trackedCommand())
	b.registerCommand(b.renameCommand())
}

func (b *Bot) registerCommand(cmd Command) {
	b.Handlers[cmd.Definition.Name] = cmd
}
