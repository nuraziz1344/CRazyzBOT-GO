package prayer

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"crazyzbot-go/internal/helper"
	"crazyzbot-go/internal/services"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type prayerTime struct {
	Name string
	At   time.Time
}

func StartScheduler(ctx context.Context, client *whatsmeow.Client, service *Service) {
	go func() {
		cityID, cityName := resolveCity(ctx, service)
		if cityID == "" {
			log.Println("Prayer scheduler: PRAYER_CITY_ID or PRAYER_CITY not set; scheduler disabled")
			return
		}

		for {
			nextPrayer, scheduleDate, err := getNextPrayer(ctx, service, cityID)
			if err != nil {
				log.Println("Prayer scheduler error:", err)
				if !sleepOrDone(ctx, 5*time.Minute) {
					return
				}
				continue
			}

			wait := time.Until(nextPrayer.At)
			if wait < time.Second {
				wait = time.Second
			}

			log.Printf("Prayer scheduler: next %s at %s (%s)", nextPrayer.Name, nextPrayer.At.Format(time.RFC3339), scheduleDate.Format("2006-01-02"))

			if !sleepOrDone(ctx, wait) {
				return
			}

			targets := collectPrayerTargets(ctx, client)
			if len(targets) == 0 {
				log.Println("Prayer scheduler: no targets found")
				continue
			}

			message := buildPrayerMessage(nextPrayer, cityName)
			for _, jid := range targets {
				helper.SendTextMessage(client, jid, message, nil)
			}
		}
	}()
}

func resolveCity(ctx context.Context, service *Service) (string, string) {
	cityID := strings.TrimSpace(os.Getenv("PRAYER_CITY_ID"))
	if cityID != "" {
		return cityID, strings.TrimSpace(os.Getenv("PRAYER_CITY_NAME"))
	}

	cityName := strings.TrimSpace(os.Getenv("PRAYER_CITY"))
	if cityName == "" {
		return "", ""
	}

	id, err := service.GetCityID(ctx, cityName)
	if err != nil {
		log.Println("Prayer scheduler: failed to resolve city:", err)
		return "", ""
	}
	return id, cityName
}

func getNextPrayer(ctx context.Context, service *Service, cityID string) (*prayerTime, time.Time, error) {
	now := time.Now()

	pt, err := getNextPrayerForDate(ctx, service, cityID, now)
	if err == nil && pt != nil {
		return pt, now, nil
	}

	tomorrow := now.AddDate(0, 0, 1)
	pt, err = getNextPrayerForDate(ctx, service, cityID, tomorrow)
	if err != nil {
		return nil, time.Time{}, err
	}
	if pt == nil {
		return nil, time.Time{}, fmt.Errorf("no prayer time found")
	}
	return pt, tomorrow, nil
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

func collectPrayerTargets(ctx context.Context, client *whatsmeow.Client) []types.JID {
	targets := make(map[string]types.JID)

	ownerNumber := strings.TrimSpace(os.Getenv("OWNER_NUMBER"))
	if ownerNumber != "" {
		jid := types.NewJID(ownerNumber, "s.whatsapp.net")
		targets[jid.String()] = jid
	}

	keywords := parseKeywords(os.Getenv("PRAYER_GROUP_KEYWORDS"))
	if len(keywords) == 0 {
		return mapToSlice(targets)
	}

	groups, err := client.GetJoinedGroups(ctx)
	if err != nil {
		log.Println("Prayer scheduler: error getting joined groups:", err)
		return mapToSlice(targets)
	}

	for _, group := range groups {
		if matchesAnyKeyword(group.Name, keywords) {
			jid := group.JID
			targets[jid.String()] = jid
		}
	}

	return mapToSlice(targets)
}

func parseKeywords(raw string) []string {
	var out []string
	for _, k := range strings.Split(raw, ",") {
		k = strings.ToLower(strings.TrimSpace(k))
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}

func matchesAnyKeyword(name string, keywords []string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for _, key := range keywords {
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
