package services

import "github.com/bwmarrin/discordgo"

const (
	SubmissionRulesGroupName  = "Submission Rules"
	SubmissionRulesGroupOrder = 0
)

func submissionRulesGroup() *pbCategoryGroup {
	return &pbCategoryGroup{
		Name:      SubmissionRulesGroupName,
		Order:     SubmissionRulesGroupOrder,
		RulesOnly: true,
	}
}

const submissionRulesEmbedDescription = "**1.** All submissions must be posted using /submit-pb.\n" +
	"The command has these fields:\n\n" +
	"• **Category** — PB leaderboard category (e.g. Vorkath, Yama). Pick from the list.\n" +
	"• **Attachment** — evidence screenshot of the PB.\n" +
	"• **Time** — your PB time, exactly as shown in in-game chat (MM:SS.xx or H:MM:SS.xx).\n" +
	"• **Teammates** (optional) — @ tag clan members who participated (up to 4).\n\n" +
	"**2.** Submissions must be obtained whilst in the Clan!\n\n" +
	"**3.** **Precise Timing** must be enabled in RuneScape. If it was not used, assume the most pessimistic value (e.g. 58.8 instead of 58.2 seconds).\n\n" +
	"**4.** Group PBs must use a team of Clan Members only. If someone later leaves the Clan, the submission is still honored if a majority of the team remains.\n\n" +
	"**5.** Evidence must include everything needed to validate the run. For group bosses or raids, show the time and a clear view or list of participants.\n\n" +
	"**6.** All submissions must include your RSN and be a full-screen picture, especially for team submissions."

func (s *PBService) buildSubmissionRulesEmbed() *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "Submission Rules",
		Description: submissionRulesEmbedDescription,
		Color:       0x3B82F6,
	}
}
