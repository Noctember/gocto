package gocto

import (
	"context"
	"fmt"
	"github.com/Noctember/gocto/helpers"
	"github.com/andersfylling/disgord"
	"regexp"
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
	Message *disgord.Message
	Channel *disgord.Channel
	Client  *disgord.Client
	Author  *disgord.User
	Monitor *Monitor
	Guild   *disgord.Guild
	Bot     *Bot
}

func monitorHandler(bot *Bot, m *disgord.Message, edit bool) {

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

		var guild *disgord.Guild = nil
		if m.GuildID != 0 {
			g, err := bot.Client.GetGuild(context.Background(), m.GuildID)
			if err != nil {
				continue
			}
			guild = g
		}

		if monitor.GuildOnly && guild == nil {
			continue
		}

		if m.Author.ID == bot.BotID && monitor.IgnoreSelf {
			continue
		}

		if m.Author.Bot && monitor.IgnoreBots {
			continue
		}

		if m.WebhookID != 0 && monitor.IgnoreWebhooks {
			continue
		}

		channel, err := bot.Client.GetChannel(context.Background(), m.ChannelID)
		if err != nil {
			continue
		}

		go monitor.Run(bot, &MonitorContext{
			Client:  bot.Client,
			Message: m,
			Author:  m.Author,
			Channel: channel,
			Monitor: monitor,
			Guild:   guild,
			Bot:     bot,
		})
	}
}

func monitorListener(bot *Bot) func(s disgord.Session, m *disgord.MessageCreate) {
	return func(s disgord.Session, m *disgord.MessageCreate) {
		monitorHandler(bot, m.Message, false)
	}
}

func monitorEditListener(bot *Bot) func(s disgord.Session, m *disgord.MessageUpdate) {
	return func(s disgord.Session, m *disgord.MessageUpdate) {
		monitorHandler(bot, m.Message, true)
	}
}

var flagsRegex = regexp.MustCompile("(?:--|—)(\\w[\\w-]+)(?:=(?:[\"]((?:[^\"\\\\]|\\\\.)*)[\"]|[']((?:[^'\\\\]|\\\\.)*)[']|[“”]((?:[^“”\\\\]|\\\\.)*)[“”]|[‘’]((?:[^‘’\\\\]|\\\\.)*)[‘’]|([\\w-]+)))?")
var delim = regexp.MustCompile("(\\s)(?:\\s)+")

func CommandHandlerMonitor(bot *Bot, ctx *MonitorContext) {
	if bot.ListHandler(bot, ctx.Message) {
		return
	}

	prefix := bot.Prefix(bot, ctx.Message, ctx.Channel.Type == disgord.ChannelTypeDM)
	if !strings.HasPrefix(ctx.Message.Content, prefix) {
		if bot.MentionPrefix {
			mPrefix := "<@" + bot.BotID.String() + "> "
			mNickPrefix := "<@!" + bot.BotID.String() + "> "
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
		Client:      ctx.Client,
		Author:      ctx.Author,
		RawArgs:     args,
		Prefix:      prefix,
		Guild:       ctx.Guild,
		Flags:       flags,
		InvokedName: input,
	}

	lang := bot.Language(bot, ctx.Message, ctx.Channel.Type == disgord.ChannelTypeDM)
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

	if cmd.OwnerOnly && ctx.Author.ID != bot.Owner {
		return
	}

	if cmd.RequiredPermissions != 0 && !PermissionsForMember(ctx.Guild, cctx.Member(int64(ctx.Author.ID))).Has(cmd.RequiredPermissions) {
		cctx.ReplyLocale("COMMAND_MISSING_PERMS", helpers.GetPermissionsText(disgord.PermissionBit(cmd.RequiredPermissions)))
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
		ctx.Client.TriggerTypingIndicator(context.Background(), ctx.Message.ChannelID)
	}

	canRun, after := bot.CheckCooldown(int64(ctx.Author.ID), cmd.Name, cmd.Cooldown)
	if !canRun {
		cctx.ReplyLocale("COMMAND_COOLDOWN", after)
		return
	}

	bot.CommandsRan++

	defer func() {
		if cmd.DeleteAfter {
			ctx.Client.DeleteMessage(context.Background(), ctx.Channel.ID, ctx.Message.ID)
		}
		if err := recover(); err != nil {
			bot.ErrorHandler(bot, &CommandError{Err: err, Context: cctx})
		}
	}()
	cmd.Run(cctx)
}
