package handler

import (
	"crazyzbot-go/internal/dto"

	"go.mau.fi/whatsmeow"
)

type CommandHandler func(c *whatsmeow.Client, msg *dto.ParsedMsg, args []string)

type Registry struct {
	commands map[string]CommandHandler
	aliases  map[string]string
}

func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]CommandHandler),
		aliases:  make(map[string]string),
	}
}

func (r *Registry) Register(name string, handler CommandHandler, aliases ...string) {
	r.commands[name] = handler
	for _, alias := range aliases {
		r.aliases[alias] = name
	}
}

func (r *Registry) Get(name string) (CommandHandler, bool) {
	if handler, ok := r.commands[name]; ok {
		return handler, true
	}
	if realName, ok := r.aliases[name]; ok {
		return r.commands[realName], true
	}
	return nil, false
}
