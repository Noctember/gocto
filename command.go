package gocto

import (
	"context"
	"fmt"
	"github.com/Noctember/disgord"
	"io"
	"runtime"
	"strings"
)

type CommandHandler func(ctx *CommandContext)

type Command struct {
	Name                string
	Aliases             []string
	Run                 CommandHandler
	Enabled             bool
	Description         string
	Category            string
	OwnerOnly           bool
	GuildOnly           bool
	UsageString         string
	Usage               []*UsageTag
	Cooldown            int
	Editable            bool
	RequiredPermissions int
	DeleteAfter         bool
	BotPermissions      int
	Override            bool
	AvailableTags       string
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
	Command     *Command
	Message     *disgord.Message
	Client      *disgord.Client
	Bot         *Bot
	Channel     *disgord.Channel
	Author      *disgord.User
	Args        []*Argument
	Prefix      string
	Guild       *disgord.Guild
	Flags       map[string]string
	Locale      *Language
	RawArgs     []string
	InvokedName string
}

type CommandError struct {
	Err     interface{}
	Context *CommandContext
	Line    int
	File    string
}

func (err *CommandError) Error() string {
	return fmt.Sprint(err.Err)
}

func (ctx *CommandContext) Reply(content string, args ...interface{}) (*disgord.Message, error) {

	if !ctx.Command.Editable {
		return ctx.ReplyNoEdit(content)
	}

	if len(args) > 0 {
		content = fmt.Sprintf(content, args...)
	}

	m, ok := ctx.Bot.CommandEdits[ctx.Message.ID]
	if !ok {
		msg, err := ctx.Client.SendMsg(context.Background(), ctx.Channel.ID, content)
		if err != nil {
			return nil, err
		}
		ctx.Bot.CommandEdits[ctx.Message.ID] = msg.ID
		return msg, nil
	}
	if !ctx.Command.Override {
		old, _ := ctx.Client.GetMessage(context.Background(), ctx.Channel.ID, m)

		return ctx.Client.UpdateMessage(context.Background(), ctx.Channel.ID, m).
			SetContent(old.Content + "\n" + content).Execute()
	}

	return ctx.Client.UpdateMessage(context.Background(), ctx.Channel.ID, m).
		SetContent(content).Execute()
}

func (ctx *CommandContext) ReplyNoEdit(content string, args ...interface{}) (*disgord.Message, error) {
	if len(args) > 0 {
		content = fmt.Sprintf(content, args...)
	}
	return ctx.Client.SendMsg(context.Background(), ctx.Channel.ID, content)
}

func (ctx *CommandContext) ReplyLocale(key string, args ...interface{}) (*disgord.Message, error) {
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

func (ctx *CommandContext) EditLocale(msg *disgord.Message, key string, args ...interface{}) (*disgord.Message, error) {
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

func (ctx *CommandContext) Edit(msg *disgord.Message, content string, args ...interface{}) (*disgord.Message, error) {
	if len(args) > 0 {
		content = fmt.Sprintf(content, args...)
	}
	return ctx.Client.UpdateMessage(context.Background(), msg.ChannelID, msg.ID).SetContent(content).Execute()
}

func (ctx *CommandContext) HasArgs() bool {
	return len(ctx.RawArgs) > 0
}

func (ctx *CommandContext) ReplyEmbed(embed *disgord.Embed) (*disgord.Message, error) {
	if !ctx.Command.Editable {
		return ctx.ReplyEmbedNoEdit(embed)
	}
	m, ok := ctx.Bot.CommandEdits[ctx.Message.ID]
	if !ok {
		msg, err := ctx.Client.CreateMessage(context.Background(), ctx.Channel.ID, &disgord.CreateMessageParams{Embed: embed})
		if err != nil {
			return nil, err
		}
		ctx.Bot.CommandEdits[ctx.Message.ID] = msg.ID
		return msg, nil
	}
	return ctx.Client.UpdateMessage(context.Background(), ctx.Channel.ID, m).SetContent("").SetEmbed(embed).Execute()
}

func (ctx *CommandContext) ReplyEmbedNoEdit(embed *disgord.Embed) (*disgord.Message, error) {
	return ctx.Client.CreateMessage(context.Background(), ctx.Channel.ID, &disgord.CreateMessageParams{Embed: embed})
}

func (ctx *CommandContext) BuildEmbed(embed *Embed) (*disgord.Message, error) {
	return ctx.ReplyEmbed(embed.Build())
}

func (ctx *CommandContext) BuildEmbedNoEdit(embed *Embed) (*disgord.Message, error) {
	return ctx.ReplyEmbedNoEdit(embed.Build())
}

func (ctx *CommandContext) SendFile(name string, file io.Reader, content string, args ...interface{}) (*disgord.Message, error) {
	if len(args) > 0 {
		content = fmt.Sprintf(content, args...)
	}

	return ctx.Client.CreateMessage(context.Background(), ctx.Channel.ID, &disgord.CreateMessageParams{Content: content, Files: []disgord.CreateMessageFileParams{{
		Reader:   file,
		FileName: name}}})
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

func (ctx *CommandContext) User(id int64) *disgord.User {
	u, _ := ctx.Client.Cache().Get(disgord.UserCache, disgord.NewSnowflake(uint64(id)))
	return u.(*disgord.User)
}

func (ctx *CommandContext) FetchUser(id int64) (*disgord.User, error) {
	user := ctx.User(id)

	if user != nil {
		return user, nil
	}

	return ctx.Client.GetUser(context.Background(), disgord.NewSnowflake(uint64(id)))
}

func (ctx *CommandContext) Member(id int64) *disgord.Member {
	if ctx.Guild == nil {
		return nil
	}
	member, err := ctx.Client.GetMember(context.Background(), ctx.Guild.ID, disgord.NewSnowflake(uint64(id)))
	if err != nil {
		return nil
	}
	return member
}

func (ctx *CommandContext) GetFirstMentionedUser() *disgord.User {
	if len(ctx.Message.Mentions) < 1 {
		return nil
	}
	return ctx.Message.Mentions[0]
}

func (ctx *CommandContext) CodeBlock(lang, content string, args ...interface{}) (*disgord.Message, error) {
	if len(args) > 0 {
		content = fmt.Sprintf(content, args...)
	}
	return ctx.Reply("```%s\n%s```", lang, content)
}

func (ctx *CommandContext) React(emoji string) error {
	return ctx.Client.CreateReaction(context.Background(), ctx.Channel.ID, ctx.Message.ID, emoji)
}
