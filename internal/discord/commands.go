package discord

import "github.com/bwmarrin/discordgo"

type Command struct {
	Definition *discordgo.ApplicationCommand
	Handler    func(s *discordgo.Session, i *discordgo.InteractionCreate)
}

func (b *Bot) setupCommands() {
	b.handlers = make(map[string]Command)

	b.registerCommand(b.setupLoggingChannelCommand())
	b.registerCommand(b.exitCommand())
	b.registerCommand(b.addAccountCommand())
	b.registerCommand(b.removeCommand())
	b.registerCommand(b.trackedCommand())
	b.registerCommand(b.renameCommand())
	b.registerCommand(b.startCommand())
	b.registerCommand(b.stopCommand())
	b.registerCommand(b.addPointsCommand())
	b.registerCommand(b.setupDonationChannelCommand())
	b.registerCommand(b.addDonationCommand())
	b.registerCommand(b.useFundsCommand())
	b.registerCommand(b.statsCommand())
	b.registerCommand(b.submitPBCommand())
}

func (b *Bot) registerCommand(cmd Command) {
	b.handlers[cmd.Definition.Name] = cmd
}
