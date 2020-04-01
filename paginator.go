package gocto

import (
	"context"
	"fmt"
	"github.com/andersfylling/disgord"
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
	Running  bool
	Client   *disgord.Client
	Channel  disgord.Snowflake
	Template func() *Embed
	Pages    []*disgord.Embed
	index    int
	Message  *disgord.Message
	AuthorID *disgord.Snowflake
	StopChan chan bool
	Extra    string
	Timeout  time.Duration
	lock     sync.Mutex
	delete   bool
}

func NewPaginator(client *disgord.Client, channel, author disgord.Snowflake) *Paginator {
	return &Paginator{
		Client:   client,
		Channel:  channel,
		Running:  false,
		index:    0,
		Message:  nil,
		AuthorID: &author,
		StopChan: make(chan bool),
		Timeout:  time.Minute * 5,
		Extra:    "",
		Template: func() *Embed { return NewEmbed() },
		delete:   false,
	}
}

func NewPaginatorForContext(ctx *CommandContext) *Paginator {
	return NewPaginator(ctx.Client, ctx.Channel.ID, ctx.Author.ID)
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
	p.Client.CreateReaction(context.Background(), p.Channel, p.Message.ID, EmojiFirst)
	p.Client.CreateReaction(context.Background(), p.Channel, p.Message.ID, EmojiLeft)
	p.Client.CreateReaction(context.Background(), p.Channel, p.Message.ID, EmojiStop)
	p.Client.CreateReaction(context.Background(), p.Channel, p.Message.ID, EmojiRight)
	p.Client.CreateReaction(context.Background(), p.Channel, p.Message.ID, EmojiLast)
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
		embed.Footer = &disgord.EmbedFooter{
			Text: fmt.Sprintf("Page %d/%d %s", index+1, len(p.Pages), p.Extra),
		}
	}
}

// Switches pages, index is assumed to be a valid index. (can panic if it's not)
// Edits the current message to the given page and updates the index.
func (p *Paginator) Goto(index int) {
	page := p.Pages[index]
	p.Client.UpdateMessage(context.Background(), p.Channel, p.Message.ID).SetEmbed(page)
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

func (p *Paginator) nextReaction() chan *disgord.MessageReactionAdd {
	channel := make(chan *disgord.MessageReactionAdd)
	ctrl := &disgord.Ctrl{Runs: 1}
	p.Client.On(disgord.EvtMessageReactionAdd, func(_ disgord.Session, r *disgord.MessageReactionAdd) {
		channel <- r
		ctrl.CloseChannel()
	}, ctrl)
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
	msg, err := p.Client.SendMsg(context.Background(), p.Channel, p.Pages[0])
	if err != nil {
		return
	}
	p.Message = msg
	if len(p.Pages) != 1 {
		p.addReactions()
	}

	p.Running = true
	start := time.Now()
	var r *disgord.MessageReactionAdd

	defer func() {
		p.Running = false
	}()

	for {
		select {
		case e := <-p.nextReaction():
			r = e
		case <-time.After(start.Add(p.Timeout).Sub(time.Now())):
			p.Client.DeleteAllReactions(context.Background(), p.Channel, p.Message.ID)
			return
		case <-p.StopChan:
			p.Client.DeleteMessage(context.Background(), p.Channel, p.Message.ID)
			return
		}

		if r.MessageID != p.Message.ID {
			continue
		}
		if !p.AuthorID.IsZero() && r.UserID.String() != p.AuthorID.String() {
			continue
		}

		go func() {
			switch r.PartialEmoji.Name {
			case EmojiStop:
				p.Stop()
				err := p.Client.DeleteAllReactions(context.Background(), p.Channel, p.Message.ID)
				if err != nil {

					p.Client.DeleteOwnReaction(context.Background(), p.Channel, p.Message.ID, EmojiFirst)
					p.Client.DeleteOwnReaction(context.Background(), p.Channel, p.Message.ID, EmojiLeft)
					p.Client.DeleteOwnReaction(context.Background(), p.Channel, p.Message.ID, EmojiStop)
					p.Client.DeleteOwnReaction(context.Background(), p.Channel, p.Message.ID, EmojiRight)
					p.Client.DeleteOwnReaction(context.Background(), p.Channel, p.Message.ID, EmojiLast)
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
			p.Client.DeleteUserReaction(context.Background(), r.ChannelID, r.MessageID, r.PartialEmoji.ID, r.UserID)
		}()
	}
}
