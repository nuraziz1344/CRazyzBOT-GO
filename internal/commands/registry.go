package commands

import (
	"context"
	"os"
	"strings"
	"sync"

	"crazyzbot-go/internal/dto"
	"crazyzbot-go/internal/logutil"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
)

type CommandHandler func(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg, args string, store storage.SubscriptionStore)

type Registry struct {
	commands map[string]CommandHandler
	aliases  map[string]string
	subStore storage.SubscriptionStore
	mu       sync.RWMutex
}

func NewRegistry(subStore storage.SubscriptionStore) *Registry {
	return &Registry{
		commands: make(map[string]CommandHandler),
		aliases:  make(map[string]string),
		subStore: subStore,
	}
}

func (r *Registry) Register(name string, handler CommandHandler, aliases ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.commands[name] = handler
	for _, alias := range aliases {
		r.aliases[alias] = name
	}
}

func (r *Registry) Handle(ctx context.Context, c *whatsmeow.Client, msg *dto.ParsedMsg) {
		// 1. Handle TagAll/Everyone special case
		// if (msg.Body == "@all" || msg.Body == "@everyone") && msg.GroupInfo != nil {
		// 	if handler, ok := r.commands["tagall"]; ok {
		// 		handler(ctx, c, msg, msg.Body, r.subStore)
		// 		return
		// 	}
		// }

		// 2. Handle Sticker/Image conversion (media-based trigger)
		if !msg.IsGroup && msg.QuotedMessage == nil && (msg.MediaType == dto.MediaSticker || msg.MediaType == dto.MediaAnimatedSticker) {
			if handler, ok := r.commands["toimg"]; ok {
				handler(ctx, c, msg, "", r.subStore)
				return
			}
		}

		// 3. Handle Text Commands
		prefix := os.Getenv("COMMAND_PREFIX")
		if prefix == "" {
			prefix = "/"
		}

		if msg.Body == "" || !strings.HasPrefix(msg.Body, prefix) {
			return
		}

		// Remove prefix
		bodyWithoutPrefix := msg.Body[len(prefix):]
		parts := strings.SplitN(bodyWithoutPrefix, " ", 2)
		cmdName := strings.ToLower(parts[0])
		args := ""
		if len(parts) > 1 {
			args = parts[1]
		}

		r.mu.RLock()
		handler, ok := r.commands[cmdName]
		if !ok {
			// Check aliases
			if realName, found := r.aliases[cmdName]; found {
				handler, ok = r.commands[realName]
			}
		}
		r.mu.RUnlock()

		if ok {
			logger := logutil.LoggerFromContext(ctx)
			logger.Info("Executing command",
				"command", cmdName,
				"args", args,
				"from", msg.From.String(),
			)
			go handler(ctx, c, msg, args, r.subStore)
		}
	}
