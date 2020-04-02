package gocto

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/jonas747/discordgo"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const VERSION = "0.0.1"

const COLOR = 0x7F139E

type PrefixHandler func(b *Bot, m *discordgo.Message, dm bool) string
type ListHandler func(b *Bot, m *discordgo.Message) bool
type LocaleHandler func(b *Bot, m *discordgo.Message, dm bool) string
type ErrorHandler func(b *Bot, err interface{})

type Bot struct {
	Session          *discordgo.Session  // The discordgo session.
	Prefix           PrefixHandler       // The handler called to get the prefix. (default: !)
	Language         LocaleHandler       // The handler called to get the language (default: en-US)
	Commands         map[string]*Command // Map of commands.
	CommandsRan      int                 // Commands ran.
	Monitors         map[string]*Monitor // Map of monitors.
	aliases          map[string]string
	CommandCooldowns map[int64]map[string]time.Time
	CommandEdits     map[int64]int64
	OwnerID          int64                // Bot owner's ID (default: fetched from application info)
	InvitePerms      int                  // Permissions bits to use for the invite link. (default: 3072)
	Languages        map[string]*Language // Map of languages.
	DefaultLocale    *Language            // Default locale to fallback. (default: en-US)
	CommandTyping    bool                 // Wether to start typing when a command is being ran. (default: true)
	ErrorHandler     ErrorHandler         // The handler to catch panics in monitors (which includes commands).
	ListHandler      ListHandler
	MentionPrefix    bool // Wether to allow @mention of the bot to be used as a prefix too. (default: true)
	sweepTicker      *time.Ticker
	Application      *discordgo.Application // The bot's application.
	Uptime           time.Time              // The time the bot hit ready event.
	Color            int                    // The color used in builtin commands's embeds.
}

// New creates a new sapphire bot, pass in a discordgo instance configured with your token.
func New(s *discordgo.Session) *Bot {
	bot := &Bot{
		Session: s,
		Prefix: func(_ *Bot, _ *discordgo.Message, _ bool) string {
			return "!" // A very common prefix, sigh, so we will make it the default.
		},
		Language: func(_ *Bot, _ *discordgo.Message, _ bool) string {
			return "en-US"
		},
		ListHandler: func(b *Bot, m *discordgo.Message) bool {
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
		CommandEdits:     make(map[int64]int64),
		Monitors:         make(map[string]*Monitor),
		CommandTyping:    true,
		sweepTicker:      time.NewTicker(1 * time.Hour),
		Application:      nil,
		MentionPrefix:    true,
		Color:            COLOR,
	}
	bot.AddLanguage(English)
	bot.SetDefaultLocale("en-US")
	bot.AddMonitor(NewMonitor("commandHandler", CommandHandlerMonitor).AllowEdits())
	s.AddHandler(monitorListener(bot))
	s.AddHandler(monitorEditListener(bot))
	s.AddHandlerOnce(func(s *discordgo.Session, ready *discordgo.Ready) {
		bot.Uptime = time.Now()

		go func() {
			<-bot.sweepTicker.C
			bot.CommandCooldowns = make(map[int64]map[string]time.Time)
			bot.CommandEdits = make(map[int64]int64)
		}()

		// TODO: for some reason it says bots cannot use this endpoint, i've seen a similar usecase before
		// try to figure out a way.
		/*app, err := s.Application(ready.User.ID)
		  if err != nil {
		    bot.ErrorHandler(bot, err)
		    return
		  p}
		  bot.Application = app
		  if bot.OwnerID == "" { bot.OwnerID = app.Owner.ID }*/
	})
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
	bot.Prefix = func(_ *Bot, _ *discordgo.Message, _ bool) string {
		return prefix
	}
	return bot
}

func (bot *Bot) Wait() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	// Cleanly close down the Discord session.
	bot.Session.Close()
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
	return bot.Session.Open()
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
		ctx.EditLocale(msg, "COMMAND_PING_PONG", taken.String(), -1)
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

		var embed = &discordgo.MessageEmbed{
			Title:  "Commands",
			Color:  bot.Color,
			Footer: &discordgo.MessageEmbedFooter{Text: "For more info on a command use: " + ctx.Prefix + "help <command>"},
			Author: &discordgo.MessageEmbedAuthor{IconURL: ctx.Author.AvatarURL("256"), Name: ctx.Author.Username},
		}

		for cat, cmds := range categories {
			var field = &discordgo.MessageEmbedField{Name: cat, Value: ""}
			field.Value = strings.Join(cmds, ", ")
			field.Inline = true
			embed.Fields = append(embed.Fields, field)
		}
		ctx.ReplyEmbed(embed)
	}).SetDescription("Shows a list of all commands.").SetUsage("[command:string]").AddAliases("h", "cmds", "commands"))

	bot.AddCommand(NewCommand("stats", "General", func(ctx *CommandContext) {
		stats := &runtime.MemStats{}
		runtime.ReadMemStats(stats)
		// Counters
		var guilds, users, channels int
		guilds = len(ctx.Session.State.Guilds)
		for _, guild := range ctx.Session.State.Guilds {
			users += guild.MemberCount
			channels += len(guild.Channels)
		}

		ctx.BuildEmbed(NewEmbed().
			SetTitle("Stats").
			SetAuthor(ctx.Session.State.User.Username, ctx.Session.State.User.AvatarURL("256")).
			SetColor(bot.Color).
			AddField("**Go Version**", strings.TrimPrefix(runtime.Version(), "go")).
			AddField("**DiscordGo Version**", discordgo.VERSION).
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
		ctx.ReplyLocale("COMMAND_INVITE", fmt.Sprintf("https://discordapp.com/oauth2/authorize?client_id=%d&permissions=%d&scope=bot",
			ctx.Session.State.User.ID, bot.InvitePerms))
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
		bot.CommandEdits = make(map[int64]int64)
		runtime.GC()
		after := &runtime.MemStats{}
		runtime.ReadMemStats(after)
		ctx.Reply("Forced Garbage Collection.\n  - Freed **%s**\n  - %d Objects Collected.\n  - Took **%d**μs",
			humanize.Bytes(before.Alloc-after.Alloc), after.Frees-before.Frees, after.PauseTotalNs-before.PauseTotalNs)
	}).SetDescription("Forces a garbage collection cycle.").AddAliases("garbagecollect", "forcegc", "runtime.GC()").SetOwnerOnly(true))
	return bot
}
