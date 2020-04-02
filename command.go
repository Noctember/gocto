package gocto

import (
	"fmt"
	"github.com/jonas747/discordgo"
	"io"
	"runtime"
	"strings"
)

type CommandHandler func(ctx *CommandContext)

type Command struct {
	Name                string         // The command's name. (default: required)
	Aliases             []string       // Aliases that point to this command. (default: [])
	Run                 CommandHandler // The handler that actually runs the command. (default: required)
	Enabled             bool           // Wether this command is enabled. (default: true)
	Description         string         // The command's brief description. (default: "No Description Provided.")
	Category            string         // The category this command belongs to. (default: required)
	OwnerOnly           bool           // Wether this command can only be used by the owner. (default: false)
	GuildOnly           bool           // Wether this command can only be ran on a guild. (default: false)
	UsageString         string         // Usage string for this command. (default: "")
	Usage               []*UsageTag    // Parsed usage tags for this command.
	Cooldown            int            // Command cooldown in seconds. (default: 0)
	Editable            bool           // Wether this command's response will be editable. (default: true)
	RequiredPermissions int            // Permissions the user needs to run this command. (default: 0)
	DeleteAfter         bool           // Deletes command when ran (default: false)
	BotPermissions      int            // Permissions the bot needs to perform this command. (default: 0)
	Override            bool           // Override message editting (default: true)
	AvailableTags       string         // Shows available tags in help command (default: none)
}

func NewCommand(name string, category string, run CommandHandler) *Command {
	return &Command{
		Name:                name,
		Category:            category,
		Run:                 run,
		Aliases:             []string{},
		Enabled:             true,
		Description:         "Mysterious command.",
		OwnerOnly:           false,
		GuildOnly:           false,
		UsageString:         "",
		Editable:            true,
		Cooldown:            0,
		RequiredPermissions: 0,
		BotPermissions:      0,
		DeleteAfter:         false,
		Usage:               make([]*UsageTag, 0),
		Override:            true,
		AvailableTags:       "",
	}
}

func (c *Command) AddAliases(aliases ...string) *Command {
	c.Aliases = append(c.Aliases, aliases...)
	return c
}

func (c *Command) SetDescription(description string) *Command {
	c.Description = description
	return c
}

func (c *Command) Delete() *Command {
	c.DeleteAfter = true
	return c
}

func (c *Command) SetUsage(usage string) *Command {
	c.UsageString = usage
	usg, err := ParseUsage(usage)
	if err != nil {
		panic(err)
	}
	c.Usage = usg
	return c
}

func (c *Command) Disable() *Command {
	c.Enabled = false
	return c
}

func (c *Command) Enable() *Command {
	c.Enabled = true
	return c
}

func (c *Command) NoOverride(status bool) *Command {
	c.Override = status
	return c
}

func (c *Command) SetOwnerOnly(toggle bool) *Command {
	c.OwnerOnly = toggle
	return c
}

func (c *Command) SetGuildOnly(toggle bool) *Command {
	c.GuildOnly = toggle
	return c
}

func (c *Command) SetEditable(toggle bool) *Command {
	c.Editable = toggle
	return c
}

func (c *Command) SetCooldown(cooldown int) *Command {
	c.Cooldown = cooldown
	return c
}

func (c *Command) SetPermission(permbit int) *Command {
	c.RequiredPermissions = permbit
	return c
}

type CommandContext struct {
	Command     *Command           // The currently executing command.
	Message     *discordgo.Message // The message of this command.
	Session     *discordgo.Session // The discordgo session.
	Bot         *Bot               // The sapphire Bot.
	Channel     *discordgo.Channel // The channel this command was ran on.
	Author      *discordgo.User    // Alias of Context.Message.Author
	Args        []*Argument        // List of arguments.
	Prefix      string             // The prefix used to invoke this command.
	Guild       *discordgo.Guild   // The guild this command was ran on.
	Flags       map[string]string  // Map of flags passed to the command. e.g --flag=yo
	Locale      *Language          // The current language.
	RawArgs     []string           // The raw args that may not match the usage string.
	InvokedName string             // The name this command was invoked as, this includes the used alias.
}

type CommandError struct {
	Err     interface{}     // The value passed to panic()
	Context *CommandContext // The context of the command, use this to e.g get the command's name etc.
	Line    int
	File    string
}

func (err *CommandError) Error() string {
	return fmt.Sprint(err.Err)
}

// Reply replies with a string.
// It will call Sprintf() on the content if atleast one vararg is passed.
func (ctx *CommandContext) Reply(content string, args ...interface{}) (*discordgo.Message, error) {

	if !ctx.Command.Editable {
		return ctx.ReplyNoEdit(content)
	}

	if len(args) > 0 {
		content = fmt.Sprintf(content, args...)
	}

	m, ok := ctx.Bot.CommandEdits[ctx.Message.ID]
	if !ok {
		msg, err := ctx.Session.ChannelMessageSend(ctx.Channel.ID, content)
		if err != nil {
			return nil, err
		}
		ctx.Bot.CommandEdits[ctx.Message.ID] = msg.ID
		return msg, nil
	}
	if !ctx.Command.Override {
		old, _ := ctx.Session.ChannelMessage(ctx.Channel.ID, m)
		return ctx.Session.ChannelMessageEditComplex(discordgo.NewMessageEdit(ctx.Channel.ID, m).
			SetContent(old.Content + "\n" + content))
	}
	return ctx.Session.ChannelMessageEditComplex(discordgo.NewMessageEdit(ctx.Channel.ID, m).
		SetContent(content))
}

// ReplyNoEdit replies with content but does not consider editable option of the command.
func (ctx *CommandContext) ReplyNoEdit(content string, args ...interface{}) (*discordgo.Message, error) {
	// See the comments in Reply
	if len(args) > 0 {
		content = fmt.Sprintf(content, args...)
	}
	return ctx.Session.ChannelMessageSend(ctx.Channel.ID, content)
}

// ReplyLocale sends a localized key for the current context's locale.
func (ctx *CommandContext) ReplyLocale(key string, args ...interface{}) (*discordgo.Message, error) {
	res := ctx.Locale.Get(key, args...)

	if res != "" {
		return ctx.Reply(res)
	}

	fallback := ctx.Bot.DefaultLocale.Get(key, args...)
	if fallback != "" {
		return ctx.Reply(fallback)
	}

	return ctx.Reply(ctx.Locale.GetDefault("LOCALE_NO_KEY", key,
		ctx.Bot.DefaultLocale.GetDefault("LOCALE_NO_KEY", key,
			fmt.Sprintf("No localization found for the key \"%s\" Please report this to the developers.", key))))
}

// EditLocale edits msg with a localized key
func (ctx *CommandContext) EditLocale(msg *discordgo.Message, key string, args ...interface{}) (*discordgo.Message, error) {
	res := ctx.Locale.Get(key, args...)
	if res != "" {
		return ctx.Edit(msg, res)
	}
	fallback := ctx.Bot.DefaultLocale.Get(key, args...)
	if fallback != "" {
		return ctx.Edit(msg, fallback)
	}

	return ctx.Edit(msg, ctx.Locale.GetDefault("LOCALE_NO_KEY", key,
		ctx.Bot.DefaultLocale.GetDefault("LOCALE_NO_KEY", key,
			fmt.Sprintf("No localization found for the key \"%s\" Please report this to the developers.", key))))
}

// Edit edits msg's content
// It will call Sprintf() on the content if atleast one vararg is passed.
func (ctx *CommandContext) Edit(msg *discordgo.Message, content string, args ...interface{}) (*discordgo.Message, error) {
	// See the comments in Reply
	if len(args) > 0 {
		content = fmt.Sprintf(content, args...)
	}
	return ctx.Session.ChannelMessageEdit(msg.ChannelID, msg.ID, content)
}

func (ctx *CommandContext) HasArgs() bool {
	return len(ctx.RawArgs) > 0
}

// ReplyEmbed replies with an embed.
func (ctx *CommandContext) ReplyEmbed(embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	if !ctx.Command.Editable {
		return ctx.ReplyEmbedNoEdit(embed)
	}
	m, ok := ctx.Bot.CommandEdits[ctx.Message.ID]
	if !ok {
		msg, err := ctx.Session.ChannelMessageSendEmbed(ctx.Channel.ID, embed)
		if err != nil {
			return nil, err
		}
		ctx.Bot.CommandEdits[ctx.Message.ID] = msg.ID
		return msg, nil
	}
	return ctx.Session.ChannelMessageEditComplex(discordgo.NewMessageEdit(ctx.Channel.ID, m).SetContent("").SetEmbed(embed))
}

// ReplyEmbedNoEdits replies with an embed but not considering the editable option of the command.
func (ctx *CommandContext) ReplyEmbedNoEdit(embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	return ctx.Session.ChannelMessageSendEmbed(ctx.Channel.ID, embed)
}

// BuildEmbed calls ReplyEmbed(embed.Build())
func (ctx *CommandContext) BuildEmbed(embed *Embed) (*discordgo.Message, error) {
	return ctx.ReplyEmbed(embed.Build())
}

// BuildEmbedNoEdit calls ReplyEmbedNoEdit(embed.Build())
func (ctx *CommandContext) BuildEmbedNoEdit(embed *Embed) (*discordgo.Message, error) {
	return ctx.ReplyEmbedNoEdit(embed.Build())
}

// SendFile sends a file with name
func (ctx *CommandContext) SendFile(name string, file io.Reader, content string, args ...interface{}) (*discordgo.Message, error) {
	if len(args) > 0 {
		content = fmt.Sprintf(content, args...)
	}

	return ctx.Session.ChannelFileSendWithMessage(ctx.Channel.ID, content, name, file)
}

func (ctx *CommandContext) Error(err interface{}, args ...interface{}) {
	_, file, line, _ := runtime.Caller(1)
	if len(args) > 0 {
		err = fmt.Sprintf(fmt.Sprint(err), args...)
	}

	ctx.ReplyLocale("COMMAND_ERROR")
	ctx.Bot.ErrorHandler(ctx.Bot, &CommandError{Err: err, Context: ctx, File: file, Line: line})
}

func (ctx *CommandContext) Flag(flag string) string {
	str, ok := ctx.Flags[flag]
	if ok {
		return str
	}
	return ""
}

func (ctx *CommandContext) HasFlag(flag string) bool {
	_, ok := ctx.Flags[flag]
	return ok
}

func (ctx *CommandContext) Arg(idx int) *Argument {
	if len(ctx.Args) > idx {
		return ctx.Args[idx]
	}
	return &Argument{provided: false}
}

func (ctx *CommandContext) JoinedArgs(sliced ...int) string {
	var s int = 0
	if len(sliced) > 0 {
		s = sliced[0]
	}
	return strings.Join(ctx.RawArgs[s:], " ")
}

func (ctx *CommandContext) ParseArgs() bool {
	safeGet := func(idx int) string {
		if len(ctx.RawArgs) > idx {
			return ctx.RawArgs[idx]
		}
		return ""
	}

	if ctx.Command.UsageString == "" {
		return true
	}

	ctx.Args = make([]*Argument, len(ctx.Command.Usage))

	for i, tag := range ctx.Command.Usage {
		v := safeGet(i)

		if tag.Required && v == "" {
			ctx.Reply("The argument **%s** is required.", tag.Name)
			return false
		}

		if tag.Rest {
			cut := ctx.RawArgs[i:]

			for ii, raw := range cut {
				arg, err := ParseArgument(ctx, tag, raw)

				if err != nil {
					ctx.Reply(err.Error())
					return false
				}

				if ii > len(ctx.Args)-2 {
					ctx.Args = append(ctx.Args, arg)
				} else {
					ctx.Args[i] = arg
				}
			}
		} else {
			arg, err := ParseArgument(ctx, tag, safeGet(i))

			if err != nil {
				ctx.Reply(err.Error())
				return false
			}

			ctx.Args[i] = arg
		}
	}

	return true
}

// User gets a user by id, returns nil if not found.
func (ctx *CommandContext) User(id int64) *discordgo.User {
	for _, guild := range ctx.Session.State.Guilds {
		if member, err := ctx.Session.State.Member(guild.ID, id); err == nil {
			return member.User
		}
	}
	return nil
}

// FetchUser searches the cache for the given user id and if not found, attempts to fetch it from the API.
func (ctx *CommandContext) FetchUser(id int64) (*discordgo.User, error) {
	// Try the cache first.
	user := ctx.User(id)

	if user != nil {
		return user, nil
	}

	// Call the API.
	return ctx.Session.User(id)
}

// Member gets a member by id from the current guild, returns nil if not found.
func (ctx *CommandContext) Member(id int64) *discordgo.Member {
	if ctx.Guild == nil {
		return nil
	}
	member, err := ctx.Session.State.Member(ctx.Guild.ID, id)
	if err != nil {
		return nil
	}
	return member
}

// GetFirstMentionedUser returns the first user mentioned in the message.
func (ctx *CommandContext) GetFirstMentionedUser() *discordgo.User {
	if len(ctx.Message.Mentions) < 1 {
		return nil
	}
	return ctx.Message.Mentions[0]
}

func (ctx *CommandContext) CodeBlock(lang, content string, args ...interface{}) (*discordgo.Message, error) {
	if len(args) > 0 {
		content = fmt.Sprintf(content, args...)
	}
	return ctx.Reply("```%s\n%s```", lang, content)
}

func (ctx *CommandContext) React(emoji string) error {
	return ctx.Session.MessageReactionAdd(ctx.Channel.ID, ctx.Message.ID, emoji)
}
