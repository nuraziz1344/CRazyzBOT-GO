package prayer

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/logutil"
	"crazyzbot-go/internal/services"
	"crazyzbot-go/internal/storage"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// maxSleep caps how long the scheduler ever sleeps in one go, so a newly added
// subscription is picked up within this window rather than waiting out whatever
// the previously-earliest subscriber's prayer time was.
const maxSleep = 5 * time.Minute

type prayerTime struct {
	Name string
	At   time.Time
}

// cityIDCache resolves and caches city name → numeric ID mappings.
type cityIDCache struct {
	mu      sync.RWMutex
	entries map[string]string // cityName -> numericID
}

func (c *cityIDCache) resolve(ctx context.Context, service *Service, nameOrID string) (string, error) {
	// If it's already a numeric ID, use it directly
	if _, err := strconv.Atoi(nameOrID); err == nil {
		return nameOrID, nil
	}

	c.mu.RLock()
	id, ok := c.entries[nameOrID]
	c.mu.RUnlock()
	if ok {
		return id, nil
	}

	id, err := service.GetCityID(ctx, nameOrID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve city %q: %w", nameOrID, err)
	}

	c.mu.Lock()
	c.entries[nameOrID] = id
	c.mu.Unlock()

	return id, nil
}

// StartScheduler runs the prayer notification loop. Unlike a naive per-subscriber loop, it
// computes the next prayer time for every subscriber up front, sleeps once until the earliest
// one is due (capped at maxSleep), then sends to everyone due at that wake — so one subscriber's
// far-off prayer time never delays another's.
func StartScheduler(ctx context.Context, client *whatsmeow.Client, service *Service, store storage.Store) {
	cache := &cityIDCache{entries: make(map[string]string)}
	lastSent := make(map[string]time.Time) // jid -> prayer time already notified, dedupes early wakes

	go func() {
		for {
			prayerSubs, err := store.ListPrayerSubscriptions(ctx)
			if err != nil {
				logutil.Error(ctx, "Prayer scheduler: failed to get prayer subscriptions", "error", err)
				if !sleepOrDone(ctx, maxSleep) {
					return
				}
				continue
			}

			if len(prayerSubs) == 0 {
				if !sleepOrDone(ctx, maxSleep) {
					return
				}
				continue
			}

			type due struct {
				jidStr string
				prayer *prayerTime
			}
			var upcoming []due
			earliest := time.Time{}

			for jidStr, rawCity := range prayerSubs {
				cityID, err := cache.resolve(ctx, service, rawCity)
				if err != nil {
					logutil.Error(ctx, "Prayer scheduler error", "jid", jidStr, "city", rawCity, "error", err)
					continue
				}

				nextPrayer, err := getNextPrayerForJID(ctx, service, jidStr, cityID)
				if err != nil {
					logutil.Error(ctx, "Prayer scheduler error", "jid", jidStr, "error", err)
					continue
				}
				if nextPrayer == nil {
					continue
				}

				upcoming = append(upcoming, due{jidStr: jidStr, prayer: nextPrayer})
				if earliest.IsZero() || nextPrayer.At.Before(earliest) {
					earliest = nextPrayer.At
				}
			}

			if len(upcoming) == 0 {
				if !sleepOrDone(ctx, maxSleep) {
					return
				}
				continue
			}

			wait := time.Until(earliest)
			if wait < time.Second {
				wait = time.Second
			}
			if wait > maxSleep {
				wait = maxSleep
			}

			logutil.Info(ctx, "Prayer scheduler: sleeping until next due prayer",
				"at", earliest.Format(time.RFC3339),
				"subscribers", len(upcoming),
			)

			if !sleepOrDone(ctx, wait) {
				return
			}

			now := time.Now()
			for _, d := range upcoming {
				if d.prayer.At.After(now) {
					continue // not due yet — will be picked up on a later pass
				}
				if lastSent[d.jidStr].Equal(d.prayer.At) {
					continue // already notified for this exact prayer time
				}

				jid, err := types.ParseJID(d.jidStr)
				if err != nil {
					logutil.Error(ctx, "Invalid prayer subscriber JID", "jid", d.jidStr, "error", err)
					continue
				}

				message := buildPrayerMessage(d.prayer, "")
				helper.SendTextMessage(ctx, client, jid, message, nil)
				lastSent[d.jidStr] = d.prayer.At
			}
		}
	}()
}

func getNextPrayerForJID(ctx context.Context, service *Service, jidStr string, cityID string) (*prayerTime, error) {
	now := time.Now()

	pt, err := getNextPrayerForDate(ctx, service, cityID, now)
	if err == nil && pt != nil {
		return pt, nil
	}

	tomorrow := now.AddDate(0, 0, 1)
	pt, err = getNextPrayerForDate(ctx, service, cityID, tomorrow)
	if err != nil {
		return nil, err
	}
	if pt == nil {
		return nil, fmt.Errorf("no prayer time found")
	}
	return pt, nil
}

func getNextPrayerForDate(ctx context.Context, service *Service, cityID string, date time.Time) (*prayerTime, error) {
	sched, err := service.GetScheduleByDate(ctx, cityID, date)
	if err != nil {
		return nil, err
	}

	loc := time.Now().Location()
	prayers := buildPrayerTimes(sched, date, loc)
	if len(prayers) == 0 {
		return nil, nil
	}

	sort.Slice(prayers, func(i, j int) bool { return prayers[i].At.Before(prayers[j].At) })

	now := time.Now()
	for _, p := range prayers {
		if p.At.After(now) {
			return p, nil
		}
	}

	return nil, nil
}

func buildPrayerTimes(sched *services.PrayerSchedule, date time.Time, loc *time.Location) []*prayerTime {
	var list []*prayerTime

	add := func(name, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		parts := strings.Split(value, ":")
		if len(parts) != 2 {
			return
		}
		hour, err1 := strconv.Atoi(parts[0])
		min, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return
		}
		at := time.Date(date.Year(), date.Month(), date.Day(), hour, min, 0, 0, loc)
		list = append(list, &prayerTime{Name: name, At: at})
	}

	add("Subuh", sched.Fajr)
	add("Dzuhur", sched.Dhuhr)
	add("Ashar", sched.Asr)
	add("Maghrib", sched.Maghrib)
	add("Isya", sched.Isha)

	return list
}

func buildPrayerMessage(p *prayerTime, cityName string) string {
	location := ""
	if cityName != "" {
		location = " (" + cityName + ")"
	}
	return fmt.Sprintf("🕌 Waktu sholat %s%s\nPukul: %s", p.Name, location, p.At.Format("15:04"))
}

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
