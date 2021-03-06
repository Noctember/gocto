package gocto

import (
	"fmt"
	"github.com/Noctember/gocto/helpers"
	"github.com/jonas747/discordgo"
	"regexp"
	"strconv"
	"strings"
)

type MonitorHandler func(bot *Bot, ctx *MonitorContext)

type Monitor struct {
	Name           string
	Enabled        bool
	Run            MonitorHandler
	GuildOnly      bool
	IgnoreWebhooks bool
	IgnoreBots     bool
	IgnoreSelf     bool
	IgnoreEdits    bool
}

func (m *Monitor) AllowBots() *Monitor {
	m.IgnoreBots = false
	return m
}

func (m *Monitor) AllowWebhooks() *Monitor {
	m.IgnoreWebhooks = false
	return m
}

func (m *Monitor) AllowSelf() *Monitor {
	m.IgnoreSelf = false
	return m
}

func (m *Monitor) SetGuildOnly(toggle bool) *Monitor {
	m.GuildOnly = toggle
	return m
}

func (m *Monitor) AllowEdits() *Monitor {
	m.IgnoreEdits = false
	return m
}

func NewMonitor(name string, monitor MonitorHandler) *Monitor {
	return &Monitor{
		Name:           name,
		Enabled:        true,
		Run:            monitor,
		GuildOnly:      false,
		IgnoreWebhooks: true,
		IgnoreBots:     true,
		IgnoreSelf:     true,
		IgnoreEdits:    true,
	}
}

type MonitorContext struct {
	Message *discordgo.Message
	Channel *discordgo.Channel
	Session *discordgo.Session
	Author  *discordgo.User
	Monitor *Monitor
	Guild   *discordgo.Guild
	Bot     *Bot
}

func monitorHandler(bot *Bot, m *discordgo.Message, edit bool) {

	if m.Author == nil {
		return
	}

	defer func() {
		if err := recover(); err != nil {
			bot.ErrorHandler(bot, err)
		}
	}()

	for _, monitor := range bot.Monitors {
		if !monitor.Enabled {
			continue
		}

		if edit && monitor.IgnoreEdits {
			continue
		}

		var guild *discordgo.Guild = nil
		if m.GuildID != 0 {
			g, err := bot.Session.State.Guild(m.GuildID)
			if err != nil {
				continue
			}
			guild = g
		}

		if monitor.GuildOnly && guild == nil {
			continue
		}

		if m.Author.ID == bot.Session.State.User.ID && monitor.IgnoreSelf {
			continue
		}

		if m.Author.Bot && monitor.IgnoreBots {
			continue
		}

		if m.WebhookID != 0 && monitor.IgnoreWebhooks {
			continue
		}

		channel, err := bot.Session.State.Channel(m.ChannelID)
		if err != nil {
			continue
		}

		go monitor.Run(bot, &MonitorContext{
			Session: bot.Session,
			Message: m,
			Author:  m.Author,
			Channel: channel,
			Monitor: monitor,
			Guild:   guild,
			Bot:     bot,
		})
	}
}

func monitorListener(bot *Bot) func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		monitorHandler(bot, m.Message, false)
	}
}

func monitorEditListener(bot *Bot) func(s *discordgo.Session, m *discordgo.MessageUpdate) {
	return func(s *discordgo.Session, m *discordgo.MessageUpdate) {
		monitorHandler(bot, m.Message, true)
	}
}

var flagsRegex = regexp.MustCompile("(?:--|—)(\\w[\\w-]+)(?:=(?:[\"]((?:[^\"\\\\]|\\\\.)*)[\"]|[']((?:[^'\\\\]|\\\\.)*)[']|[“”]((?:[^“”\\\\]|\\\\.)*)[“”]|[‘’]((?:[^‘’\\\\]|\\\\.)*)[‘’]|([\\w-]+)))?")
var delim = regexp.MustCompile("(\\s)(?:\\s)+")

func CommandHandlerMonitor(bot *Bot, ctx *MonitorContext) {
	if bot.ListHandler(bot, ctx.Message) {
		return
	}

	prefix := bot.Prefix(bot, ctx.Message, ctx.Channel.Type == discordgo.ChannelTypeDM)
	if !strings.HasPrefix(ctx.Message.Content, prefix) {
		if bot.MentionPrefix {
			// Check mention prefix.
			// Could've used regex here but it adds more complexity of compiling it at a proper time
			// Because we will need the ID so we would need to delay it until ready.
			// Let's just simplify it for now.
			mPrefix := "<@" + strconv.FormatInt(bot.Session.State.User.ID, 10) + "> "
			mNickPrefix := "<@!" + strconv.FormatInt(bot.Session.State.User.ID, 10) + "> "
			if strings.HasPrefix(ctx.Message.Content, mPrefix) {
				prefix = mPrefix
			} else if strings.HasPrefix(ctx.Message.Content, mNickPrefix) {
				prefix = mNickPrefix
			} else {
				// No prefix found.
				return
			}
		} else {
			return
		}
	}

	flags := make(map[string]string)
	content := strings.Trim(delim.ReplaceAllString(flagsRegex.ReplaceAllStringFunc(ctx.Message.Content, func(m string) string {
		sub := flagsRegex.FindStringSubmatch(m)
		for _, elem := range sub[2:] {
			if elem != "" {
				flags[sub[1]] = elem
				break
			} else {
				flags[sub[1]] = sub[1]
			}
		}
		return ""
	}), "$1"), " ")

	split := strings.Split(content[len(prefix):], " ")

	if len(split) < 1 {
		return
	}

	input := strings.ToLower(split[0])
	var args []string

	if len(split) > 1 {
		args = split[1:]
	}

	cmd := bot.GetCommand(input)
	if cmd == nil {
		return
	}

	cctx := &CommandContext{
		Bot:         bot,
		Command:     cmd,
		Message:     ctx.Message,
		Channel:     ctx.Channel,
		Session:     ctx.Session,
		Author:      ctx.Author,
		RawArgs:     args,
		Prefix:      prefix,
		Guild:       ctx.Guild,
		Flags:       flags,
		InvokedName: input,
	}

	lang := bot.Language(bot, ctx.Message, ctx.Channel.Type == discordgo.ChannelTypeDM)
	locale, ok := bot.Languages[lang]

	// Shouldn't happen unless the user made a mistake returning an invalid string, let's help them find the problem.
	if !ok {
		fmt.Printf("WARNING: bot.Language handler returned a non-existent language '%s' (command execution aborted)\n", lang)
		return
	}

	// Set the context's locale.
	cctx.Locale = locale

	// Validations.
	if !cmd.Enabled {
		return
	}

	if cmd.OwnerOnly && ctx.Author.ID != bot.OwnerID {
		return
	}

	if cmd.RequiredPermissions != 0 && !PermissionsForMember(ctx.Guild, cctx.Member(int64(ctx.Author.ID))).Has(cmd.RequiredPermissions) {
		cctx.ReplyLocale("COMMAND_MISSING_PERMS", helpers.GetPermissionsText(cmd.RequiredPermissions))
		return
	}

	if cmd.GuildOnly && ctx.Message.GuildID == 0 {
		cctx.ReplyLocale("COMMAND_GUILD_ONLY")
		return
	}

	// If parse args failed it returns false
	// We don't need to reply since ParseArgs already reports the appropriate error before returning.
	if !cctx.ParseArgs() {
		return
	}

	if bot.CommandTyping {
		ctx.Session.ChannelTyping(ctx.Message.ChannelID)
	}

	canRun, after := bot.CheckCooldown(ctx.Author.ID, cmd.Name, cmd.Cooldown)
	if !canRun {
		cctx.ReplyLocale("COMMAND_COOLDOWN", after)
		return
	}

	bot.CommandsRan++

	defer func() {
		if cmd.DeleteAfter {
			ctx.Session.ChannelMessageDelete(ctx.Channel.ID, ctx.Message.ID)
		}
		if err := recover(); err != nil {
			bot.ErrorHandler(bot, &CommandError{Err: err, Context: cctx})
		}
	}()
	cmd.Run(cctx)
}
