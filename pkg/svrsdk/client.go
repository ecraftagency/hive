package svrsdk

import (
	"context"
)

type Option func(*Client)

type Client struct {
	cfg      Config
	sources  []ShutdownSource
	mws      []Middleware
	final    ShutdownHandler
	notifier ShutdownNotifier
}

func Init(cfg Config, opts ...Option) *Client {
	c := &Client{cfg: cfg}
	for _, o := range opts {
		o(c)
	}
	// default notifier & final handler
	if c.notifier == nil {
		c.notifier = &AgentNotifier{}
	}
	if c.final == nil {
		c.final = func(sc *ShutdownContext) ShutdownResult {
			err := c.notifier.Notify(sc.Ctx, c.cfg, *sc.Event)
			return ShutdownResult{Decision: DecisionContinue, Err: err}
		}
	}
	return c
}

func (c *Client) UseSource(src ShutdownSource)      { c.sources = append(c.sources, src) }
func (c *Client) Use(mw Middleware)                 { c.mws = append(c.mws, mw) }
func (c *Client) SetFinalHandler(h ShutdownHandler) { c.final = h }
func (c *Client) SetNotifier(n ShutdownNotifier)    { c.notifier = n }

// build pipeline từ middleware -> final
func (c *Client) buildPipeline() ShutdownHandler {
	h := c.final
	for i := len(c.mws) - 1; i >= 0; i-- {
		h = c.mws[i](h)
	}
	return h
}

// Run khởi động tất cả sources, trả hàm stop
func (c *Client) Run() (stop func()) {
	stops := make([]func(), 0, len(c.sources))
	emit := func(ev ShutdownEvent) {
		ctx := &ShutdownContext{Ctx: context.Background(), Config: c.cfg, Event: &ev}
		handler := c.buildPipeline()
		res := handler(ctx)
		_ = res // có thể ghi log/res ở trên layer ứng dụng
	}
	for _, s := range c.sources {
		stops = append(stops, s.Start(emit))
	}
	return func() {
		for _, s := range stops {
			s()
		}
	}
}

// Helper để gửi shutdown có kèm payload chi tiết (ví dụ endgame scores)
func (c *Client) SendShutdownWithDetails(reason ShutdownReason, details map[string]any) error {
	ev := NewEvent(reason)
	ev.Details = details
	ctx := &ShutdownContext{Ctx: context.Background(), Config: c.cfg, Event: &ev}
	res := c.buildPipeline()(ctx)
	return res.Err
}
