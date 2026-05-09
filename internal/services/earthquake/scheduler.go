package earthquake

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"crazyzbot-go/internal/helper"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func StartScheduler(ctx context.Context, client *whatsmeow.Client, service *Service) {
	ticker := time.NewTicker(service.GetInterval())

	go func() {
		defer ticker.Stop()

		checkAndNotify := func() {
			event, err := service.GetLatest(ctx)
			if err != nil {
				log.Println("BMKG fetch error:", err)
				return
			}

			if event == nil {
				return
			}

			eventID := event.ID()
			if eventID == "" || eventID == service.GetLastEventID() {
				return
			}
			service.SetLastEventID(eventID)

			if event.Magnitude <= 4.0 {
				return
			}

			alert := Alert{Text: formatMessage(*event), ShakemapURL: buildShakemapURL(event.ShakeMap)}
			if alert.ShakemapURL != "" {
				if img, err := service.GetShakeMap(ctx, event.ShakeMap); err == nil && len(img) > 0 {
					alert.Shakemap = img
				}
			}

			targets := collectEarthquakeTargets(ctx, client)
			if len(targets) == 0 {
				return
			}

			for _, jid := range targets {
				if len(alert.Shakemap) > 0 {
					helpTextErr := helper.SendImageMessageWithCaption(client, jid, &alert.Shakemap, alert.Text, nil)
					if helpTextErr != nil {
						log.Println("Error sending shakemap:", helpTextErr)
					}
				} else {
					helper.SendTextMessage(client, jid, alert.Text, nil)
				}
			}
		}

		checkAndNotify()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkAndNotify()
			}
		}
	}()
}

func collectEarthquakeTargets(ctx context.Context, client *whatsmeow.Client) []types.JID {
	targets := make(map[string]types.JID)

	ownerNumber := strings.TrimSpace(os.Getenv("OWNER_NUMBER"))
	if ownerNumber != "" {
		jid := types.NewJID(ownerNumber, "s.whatsapp.net")
		targets[jid.String()] = jid
	}

	groups, err := client.GetJoinedGroups(ctx)
	if err != nil {
		log.Println("Error getting joined groups:", err)
		return mapToSlice(targets)
	}

	for _, group := range groups {
		if matchesEarthquakeGroupName(group.Name) {
			jid := group.JID
			targets[jid.String()] = jid
		}
	}

	return mapToSlice(targets)
}

func matchesEarthquakeGroupName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	keys := []string{"tcbb", "sutechbayo", "100% halal"}
	for _, key := range keys {
		if strings.Contains(name, key) {
			return true
		}
	}
	return false
}

func mapToSlice(m map[string]types.JID) []types.JID {
	list := make([]types.JID, 0, len(m))
	for _, jid := range m {
		list = append(list, jid)
	}
	return list
}
