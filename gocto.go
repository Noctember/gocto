package gocto

import (
	"context"
	"fmt"
	"github.com/andersfylling/disgord"
	"github.com/dustin/go-humanize"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const VERSION = "0.0.1"

const COLOR = 0x7F139E

type PrefixHandler func(b *Bot, m *disgord.Message, dm bool) string
type ListHandler func(b *Bot, m *disgord.Message) bool
type LocaleHandler func(b *Bot, m *disgord.Message, dm bool) string
type ErrorHandler func(b *Bot, err interface{})

type Bot struct {
	Client           *disgord.Client
	Prefix           PrefixHandler
	Language         LocaleHandler
	Commands         map[string]*Command
	CommandsRan      int
	Monitors         map[string]*Monitor
	aliases          map[string]string
	CommandCooldowns map[int64]map[string]time.Time
	CommandEdits     map[disgord.Snowflake]disgord.Snowflake
	Owner            disgord.Snowflake
	BotID            disgord.Snowflake
	InvitePerms      int
	Languages        map[string]*Language
	DefaultLocale    *Language
	CommandTyping    bool
	ErrorHandler     ErrorHandler
	ListHandler      ListHandler
	MentionPrefix    bool
	sweepTicker      *time.Ticker
	Uptime           time.Time
	Color            int
}

func New(s *disgord.Client) *Bot {
	bot := &Bot{
		Client: s,
		Prefix: func(_ *Bot, _ *disgord.Message, _ bool) string {
			return "!"
		},
		Language: func(_ *Bot, _ *disgord.Message, _ bool) string {
			return "en-US"
		},
		ListHandler: func(b *Bot, m *disgord.Message) bool {
			return true
		},
		ErrorHandler: func(_ *Bot, err interface{}) {
			fmt.Printf("Panic recovered: %v\n", err)
		},
		Commands:         make(map[string]*Command),
		aliases:          make(map[string]string),
		Languages:        make(map[string]*Language),
		CommandsRan:      0,
		InvitePerms:      3072,
		CommandCooldowns: make(map[int64]map[string]time.Time),
		CommandEdits:     make(map[disgord.Snowflake]disgord.Snowflake),
		Monitors:         make(map[string]*Monitor),
		CommandTyping:    true,
		sweepTicker:      time.NewTicker(2 * time.Hour),
		MentionPrefix:    true,
		Color:            COLOR,
	}
	bot.AddLanguage(English)
	bot.SetDefaultLocale("en-US")
	bot.AddMonitor(NewMonitor("commandHandler", CommandHandlerMonitor).AllowEdits())
	s.On(disgord.EvtMessageCreate, monitorListener(bot))
	s.On(disgord.EvtMessageUpdate, monitorEditListener(bot))
	s.On(disgord.EvtReady, func(s disgord.Session, ready *disgord.Ready) {
		bot.Uptime = time.Now()

		go func() {
			<-bot.sweepTicker.C
			bot.CommandCooldowns = make(map[int64]map[string]time.Time)
			bot.CommandEdits = make(map[disgord.Snowflake]disgord.Snowflake)
		}()
	}, &disgord.Ctrl{Runs: 1})
	return bot
}

func (bot *Bot) SetMentionPrefix(toggle bool) *Bot {
	bot.MentionPrefix = toggle
	return bot
}

func (bot *Bot) SetInvitePerms(bits int) *Bot {
	bot.InvitePerms = bits
	return bot
}

func (bot *Bot) SetErrorHandler(fn ErrorHandler) *Bot {
	bot.ErrorHandler = fn
	return bot
}

func (bot *Bot) SetDefaultLocale(locale string) *Bot {
	if lang, ok := bot.Languages[locale]; !ok {
		panic(fmt.Sprintf("The language '%s' cannot be found.", locale))
	} else {
		bot.DefaultLocale = lang
	}
	return bot
}

func (bot *Bot) SetLocaleHandler(handler LocaleHandler) *Bot {
	bot.Language = handler
	return bot
}

func (bot *Bot) SetListHandler(list ListHandler) *Bot {
	bot.ListHandler = list
	return bot
}

func (bot *Bot) SetPrefixHandler(prefix PrefixHandler) *Bot {
	bot.Prefix = prefix
	return bot
}

func (bot *Bot) SetPrefix(prefix string) *Bot {
	bot.Prefix = func(_ *Bot, _ *disgord.Message, _ bool) string {
		return prefix
	}
	return bot
}

func (bot *Bot) Wait() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	bot.Client.Disconnect()
	bot.sweepTicker.Stop()
}

func (bot *Bot) AddCommand(cmd *Command) *Bot {
	c, ok := bot.Commands[cmd.Name]
	if ok {
		for _, a := range c.Aliases {
			delete(bot.aliases, a)
		}
	}
	bot.Commands[cmd.Name] = cmd
	for _, alias := range cmd.Aliases {
		bot.aliases[alias] = cmd.Name
	}
	return bot
}

func (bot *Bot) GetCommand(name string) *Command {
	cmd, ok := bot.Commands[name]
	if ok {
		return cmd
	}
	alias, ok := bot.aliases[name]
	if ok {
		return bot.Commands[alias]
	}
	return nil
}

func (bot *Bot) Connect() error {
	return bot.Client.Connect(context.Background())
}

// MustConnect is like Connect but panics if there is an error.
func (bot *Bot) MustConnect() {
	if err := bot.Connect(); err != nil {
		panic(err)
	}
}

// AddLanguage adds the specified language.
func (bot *Bot) AddLanguage(lang *Language) *Bot {
	bot.Languages[lang.Name] = lang
	return bot
}

func (bot *Bot) AddMonitor(m *Monitor) *Bot {
	bot.Monitors[m.Name] = m
	return bot
}

func (bot *Bot) CheckCooldown(userID int64, command string, cooldownSec int) (bool, int) {
	if cooldownSec == 0 {
		return true, 0
	}

	cooldown := time.Duration(cooldownSec) * time.Second
	user, ok := bot.CommandCooldowns[userID]

	if !ok {
		bot.CommandCooldowns[userID] = make(map[string]time.Time)
		user = bot.CommandCooldowns[userID]
	}

	last, ok := user[command]

	if !ok {
		user[command] = time.Now()
		return true, 0
	}

	if !time.Now().After(last.Add(cooldown)) {
		return false, int(time.Until(last.Add(cooldown)).Seconds())
	}

	user[command] = time.Now()
	return true, 0
}

func (bot *Bot) LoadBuiltins() *Bot {
	bot.AddCommand(NewCommand("ping", "General", func(ctx *CommandContext) {
		bottime := time.Now()
		msg, err := ctx.ReplyLocale("COMMAND_PING")
		if err != nil {
			return
		}
		taken := time.Duration(time.Now().UnixNano() - bottime.UnixNano())
		started := time.Now()
		ctx.EditLocale(msg, "COMMAND_PING_PONG", taken.String(), "Loading")
		httpPing := time.Since(started)

		ctx.EditLocale(msg, "COMMAND_PING_PONG", taken.String(), httpPing.String())
	}).SetDescription("Pong! Responds with Bot latency."))

	bot.AddCommand(NewCommand("help", "General", func(ctx *CommandContext) {
		if ctx.HasArgs() {
			cmd := bot.GetCommand(ctx.Args[0].AsString())
			if cmd == nil {
				ctx.Reply("Unknown Command.")
				return
			}
			var aliases string = "None"

			if len(cmd.Aliases) > 0 {
				aliases = strings.Join(cmd.Aliases, ", ")
			}

			extra := ""
			if cmd.AvailableTags != "" {
				extra = "Flags: " + cmd.AvailableTags
			}
			ctx.BuildEmbed(NewEmbed().
				SetDescription(fmt.Sprintf("**Name:** %s\n**Description:** %s\n**Category:** %s\n**Aliases:** %s\n**Usage:** %s \n%s",
					cmd.Name,
					cmd.Description,
					cmd.Category,
					aliases,
					fmt.Sprintf("%s%s %s", ctx.Prefix, cmd.Name, HumanizeUsage(cmd.UsageString)),
					extra,
				)).SetColor(bot.Color).SetTitle("Command Help"))
			return
		}

		categories := make(map[string][]string)
		for _, v := range bot.Commands {
			_, ok := categories[v.Category]
			if !ok {
				categories[v.Category] = []string{}
			}
			if !v.OwnerOnly || ctx.Author.ID == ctx.Bot.Owner {
				categories[v.Category] = append(categories[v.Category], v.Name)
			}
		}

		for k, v := range categories {
			if len(v) == 0 {
				delete(categories, k)
			}
		}
		av, _ := ctx.Author.AvatarURL(256, true)
		var embed = &disgord.Embed{
			Title:  "Commands",
			Color:  bot.Color,
			Footer: &disgord.EmbedFooter{Text: "For more info on a command use: " + ctx.Prefix + "help <command>"},
			Author: &disgord.EmbedAuthor{IconURL: av, Name: ctx.Author.Username},
		}

		for cat, cmds := range categories {
			var field = &disgord.EmbedField{Name: cat, Value: ""}
			field.Value = strings.Join(cmds, ", ")
			field.Inline = true
			embed.Fields = append(embed.Fields, field)
		}
		ctx.ReplyEmbed(embed)
	}).SetDescription("Shows a list of all commands.").SetUsage("[command:string]").AddAliases("h", "cmds", "commands"))

	bot.AddCommand(NewCommand("stats", "General", func(ctx *CommandContext) {
		stats := &runtime.MemStats{}
		runtime.ReadMemStats(stats)

		guildsTmp, _ := ctx.Client.GetGuilds(context.Background(), &disgord.GetCurrentUserGuildsParams{After: disgord.NewSnowflake(0)})
		var guilds, channels int
		var users uint
		guilds = len(guildsTmp)
		for _, guild := range guildsTmp {
			users += guild.MemberCount
			channels += len(guild.Channels)
		}
		self := ctx.User(int64(ctx.Bot.BotID))
		av, _ := self.AvatarURL(256, true)
		ctx.BuildEmbed(NewEmbed().
			SetTitle("Stats").
			SetAuthor(self.Username, av).
			SetColor(bot.Color).
			AddField("**Go Version**", strings.TrimPrefix(runtime.Version(), "go")).
			AddField("**DiscordGo Version**", disgord.Version).
			AddField("**Command Stats**", fmt.Sprintf("Total Commands: %d\nCommands Ran: %d", len(bot.Commands), bot.CommandsRan)).
			AddField("**Bot Stats**", fmt.Sprintf("Guilds: %d\nUsers: %d\nChannels: %d\nUptime: %s", guilds, users, channels, humanize.RelTime(bot.Uptime, time.Now(), "", ""))).
			AddField("**Memory Stats**", fmt.Sprintf("Used: %s / %s\nGarbage Collected: %s\nGC Cycles: %d\nForced GC Cycles: %d\nLast GC: %s\nNext GC Target: %s\nGoroutines: %d",
				humanize.Bytes(stats.Alloc),
				humanize.Bytes(stats.Sys),
				humanize.Bytes(stats.TotalAlloc-stats.Alloc),
				stats.NumGC,
				stats.NumForcedGC,
				humanize.Time(time.Unix(0, int64(stats.LastGC))),
				humanize.Bytes(stats.NextGC),
				runtime.NumGoroutine(),
			)).
			AddField("**Technical Info**", fmt.Sprintf("CPU Cores: %d\nOS/Arch: %s/%s",
				runtime.NumCPU(),
				runtime.GOOS,
				runtime.GOARCH,
			)).InlineAllFields())
	}).SetDescription("Stats for nerds.").AddAliases("botstats", "info"))

	bot.AddCommand(NewCommand("invite", "General", func(ctx *CommandContext) {
		ctx.ReplyLocale("COMMAND_INVITE", fmt.Sprintf("https://discordapp.com/oauth2/authorize?client_id=%s&permissions=%d&scope=bot",
			ctx.Bot.BotID.String(), bot.InvitePerms))
	}).SetDescription("Invite me to your server!").AddAliases("inv"))

	bot.AddCommand(NewCommand("enable", "Owner", func(ctx *CommandContext) {
		command := ctx.Bot.GetCommand(ctx.Arg(0).AsString())
		if command == nil {
			ctx.ReplyLocale("COMMAND_NOT_FOUND", ctx.Arg(0))
			return
		}
		if command.Enabled {
			ctx.ReplyLocale("COMMAND_ENABLE_ALREADY")
			return
		}
		command.Enable()
		ctx.ReplyLocale("COMMAND_ENABLE_SUCCESS", ctx.Arg(0))
	}).SetDescription("Enables a disabled command.").SetOwnerOnly(true).SetUsage("<command:string>"))

	bot.AddCommand(NewCommand("disable", "Owner", func(ctx *CommandContext) {
		command := ctx.Bot.GetCommand(ctx.Arg(0).AsString())
		if command == nil {
			ctx.ReplyLocale("COMMAND_NOT_FOUND", ctx.Arg(0).AsString())
			return
		}

		if !command.Enabled {
			ctx.ReplyLocale("COMMAND_DISABLE_ALREADY")
			return
		}
		command.Disable()
		ctx.ReplyLocale("COMMAND_DISABLE_SUCCESS", ctx.Arg(0).AsString())
	}).SetDescription("Disables an enabled command.").SetOwnerOnly(true).SetUsage("<command:string>"))

	bot.AddCommand(NewCommand("gc", "Owner", func(ctx *CommandContext) {
		before := &runtime.MemStats{}
		runtime.ReadMemStats(before)

		bot.CommandCooldowns = make(map[int64]map[string]time.Time)
		bot.CommandEdits = make(map[disgord.Snowflake]disgord.Snowflake)
		runtime.GC()
		after := &runtime.MemStats{}
		runtime.ReadMemStats(after)
		ctx.Reply("Forced Garbage Collection.\n  - Freed **%s**\n  - %d Objects Collected.\n  - Took **%d**μs",
			humanize.Bytes(before.Alloc-after.Alloc), after.Frees-before.Frees, after.PauseTotalNs-before.PauseTotalNs)
	}).SetDescription("Forces a garbage collection cycle.").AddAliases("garbagecollect", "forcegc", "runtime.GC()").SetOwnerOnly(true))
	return bot
}
