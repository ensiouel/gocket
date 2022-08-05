package gocket

import "context"

type Context struct {
	context.Context

	socket *Socket
}

func (c *Context) Socket() *Socket {
	return c.socket
}

func NewContext(ctx context.Context, socket *Socket) *Context {
	return &Context{ctx, socket}
}
