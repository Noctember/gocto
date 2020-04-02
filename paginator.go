package gocto

import (
	"fmt"
	"github.com/jonas747/discordgo"
	"sync"
	"time"
)

const (
	EmojiLeft  = "◀️"
	EmojiRight = "▶️"
	EmojiFirst = "⏪"
	EmojiLast  = "⏩"
	EmojiStop  = "⏹️"
)

type Paginator struct {
	Running   bool
	Session   *discordgo.Session
	ChannelID int64
	Template  func() *Embed
	Pages     []*discordgo.MessageEmbed
	index     int
	Message   *discordgo.Message
	AuthorID  int64
	StopChan  chan bool
	Extra     string
	Timeout   time.Duration
	lock      sync.Mutex
	delete    bool
}

func NewPaginator(session *discordgo.Session, channel, author int64) *Paginator {
	return &Paginator{
		Session:   session,
		ChannelID: channel,
		Running:   false,
		index:     0,
		Message:   nil,
		AuthorID:  author,
		StopChan:  make(chan bool),
		Timeout:   time.Minute * 5,
		Extra:     "",
		Template:  func() *Embed { return NewEmbed() },
		delete:    false,
	}
}

func NewPaginatorForContext(ctx *CommandContext) *Paginator {
	return NewPaginator(ctx.Session, ctx.Channel.ID, ctx.Author.ID)
}

func (p *Paginator) SetTemplate(em func() *Embed) {
	p.Template = em
}

func (p *Paginator) GetIndex() int {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.index
}

func (p *Paginator) Delete() {
	p.delete = true
}

func (p *Paginator) AddPage(fn func(em *Embed) *Embed) {
	p.Pages = append(p.Pages, fn(p.Template()).Build())
}

func (p *Paginator) AddPageString(str string) {
	p.AddPage(func(em *Embed) *Embed {
		return em.SetDescription(str)
	})
}

func (p *Paginator) addReactions() {
	if p.Message == nil {
		return
	}
	p.Session.MessageReactionAdd(p.ChannelID, p.Message.ID, EmojiFirst)
	p.Session.MessageReactionAdd(p.ChannelID, p.Message.ID, EmojiLeft)
	p.Session.MessageReactionAdd(p.ChannelID, p.Message.ID, EmojiStop)
	p.Session.MessageReactionAdd(p.ChannelID, p.Message.ID, EmojiRight)
	p.Session.MessageReactionAdd(p.ChannelID, p.Message.ID, EmojiLast)
}

// Stops the paginator by sending the signal to the Stop Channel.
func (p *Paginator) Stop() {
	p.StopChan <- true
}

func (p *Paginator) SetExtra(extra string) {
	p.Extra = extra
}

// Retrieves the next index for the next page
// returns 0 to go back to first page if we are on last page already.
func (p *Paginator) getNextIndex() int {
	index := p.GetIndex()
	if index >= len(p.Pages)-1 {
		return 0
	}
	return index + 1
}

// Retrieves the previous index for the previous page
// returns the last page if we are already on the first page.
func (p *Paginator) getPreviousIndex() int {
	index := p.GetIndex()
	if index == 0 {
		return len(p.Pages) - 1
	}
	return index - 1
}

// Sets the footers of all pages to their page number out of total pages.
// Called by Run to initialize.
func (p *Paginator) SetFooter() {
	for index, embed := range p.Pages {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Page %d/%d %s", index+1, len(p.Pages), p.Extra),
		}
	}
}

// Switches pages, index is assumed to be a valid index. (can panic if it's not)
// Edits the current message to the given page and updates the index.
func (p *Paginator) Goto(index int) {
	page := p.Pages[index]
	p.Session.ChannelMessageEditEmbed(p.ChannelID, p.Message.ID, page)
	p.lock.Lock()
	p.index = index
	p.lock.Unlock()
}

// Switches to next page, this is safer than raw Goto as it compares indices
// and switch to first page if we are already on last one.
func (p *Paginator) NextPage() {
	p.Goto(p.getNextIndex())
}

// Switches to the previous page, this is safer than raw Goto as it compares indices
// and switch to last page if we are already on the first one.
func (p *Paginator) PreviousPage() {
	p.Goto(p.getPreviousIndex())
}

func (p *Paginator) nextReaction() chan *discordgo.MessageReactionAdd {
	channel := make(chan *discordgo.MessageReactionAdd)
	p.Session.AddHandlerOnce(func(_ *discordgo.Session, r *discordgo.MessageReactionAdd) {
		channel <- r
	})
	return channel
}

func (p *Paginator) Run() {
	if p.Running {
		return
	}
	if len(p.Pages) == 0 {
		return
	}
	p.SetFooter()
	msg, err := p.Session.ChannelMessageSendEmbed(p.ChannelID, p.Pages[0])
	if err != nil {
		return
	}
	p.Message = msg
	if len(p.Pages) != 1 {
		p.addReactions()
	}

	p.Running = true
	start := time.Now()
	var r *discordgo.MessageReaction

	defer func() {
		p.Running = false
	}()

	for {
		select {
		case e := <-p.nextReaction():
			r = e.MessageReaction
		case <-time.After(start.Add(p.Timeout).Sub(time.Now())):
			p.Session.MessageReactionsRemoveAll(p.ChannelID, p.Message.ID)
			return
		case <-p.StopChan:
			p.Session.ChannelMessageDelete(p.ChannelID, p.Message.ID)
			return
		}

		if r.MessageID != p.Message.ID {
			continue
		}
		if p.AuthorID != 0 && r.UserID != p.AuthorID {
			continue
		}

		go func() {
			switch r.Emoji.Name {
			case EmojiStop:
				p.Stop()
				err := p.Session.MessageReactionsRemoveAll(p.ChannelID, p.Message.ID)
				if err != nil {
					p.Session.MessageReactionRemoveMe(p.ChannelID, p.Message.ID, EmojiFirst)
					p.Session.MessageReactionRemoveMe(p.ChannelID, p.Message.ID, EmojiLeft)
					p.Session.MessageReactionRemoveMe(p.ChannelID, p.Message.ID, EmojiStop)
					p.Session.MessageReactionRemoveMe(p.ChannelID, p.Message.ID, EmojiRight)
					p.Session.MessageReactionRemoveMe(p.ChannelID, p.Message.ID, EmojiLast)
				}
			case EmojiRight:
				p.NextPage()
			case EmojiLeft:
				p.PreviousPage()
			case EmojiFirst:
				p.Goto(0)
			case EmojiLast:
				p.Goto(len(p.Pages) - 1)
			}
		}()
		go func() {
			time.Sleep(time.Millisecond * 250)
			p.Session.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)
		}()
	}
}
