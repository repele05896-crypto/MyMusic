package azan

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/robfig/cron/v3"

	"main/internal/config"
	"main/internal/core"
	"main/internal/platforms"
)

var (
	Scheduler *cron.Cron
	BotClient *telegram.Client
)

func InitAzanScheduler(client *telegram.Client) {
	BotClient = client

	loc, err := time.LoadLocation("Africa/Cairo")
	if err != nil {
		log.Println("Loc error, using local time:", err)
		loc = time.Local
	}

	Scheduler = cron.New(cron.WithLocation(loc))

	if _, err := Scheduler.AddFunc("5 0 * * *", UpdateAzanTimes); err != nil {
		log.Println("AddFunc UpdateAzanTimes failed:", err)
	}
	if _, err := Scheduler.AddFunc("0 7 * * *", func() {
		BroadcastDuas(MorningDuas, "أذكار الصباح")
	}); err != nil {
		log.Println("AddFunc MorningDuas failed:", err)
	}
	if _, err := Scheduler.AddFunc("0 20 * * *", func() {
		BroadcastDuas(NightDuas, "أذكار المساء")
	}); err != nil {
		log.Println("AddFunc NightDuas failed:", err)
	}

	go UpdateAzanTimes()
	Scheduler.Start()
}

func UpdateAzanTimes() {
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("http://api.aladhan.com/v1/timingsByCity?city=Cairo&country=Egypt&method=5")
	if err != nil {
		log.Println("HTTP request failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp == nil {
		log.Println("HTTP response is nil")
		return
	}

	var result struct {
		Data struct {
			Timings map[string]string `json:"timings"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Println("JSON decode failed:", err)
		return
	}

	if Scheduler != nil {
		loc := Scheduler.Location()
		Scheduler.Stop()
		Scheduler = cron.New(cron.WithLocation(loc))

		if _, err := Scheduler.AddFunc("5 0 * * *", UpdateAzanTimes); err != nil {
			log.Println("AddFunc UpdateAzanTimes failed:", err)
		}
		if _, err := Scheduler.AddFunc("0 7 * * *", func() {
			BroadcastDuas(MorningDuas, "أذكار الصباح")
		}); err != nil {
			log.Println("AddFunc MorningDuas failed:", err)
		}
		if _, err := Scheduler.AddFunc("0 20 * * *", func() {
			BroadcastDuas(NightDuas, "أذكار المساء")
		}); err != nil {
			log.Println("AddFunc NightDuas failed:", err)
		}
	}

	for prayerKey, link := range PrayerLinks {
		timeStr, ok := result.Data.Timings[prayerKey]
		if !ok {
			continue
		}

		clean := strings.Split(timeStr, " ")[0]
		parts := strings.Split(clean, ":")
		if len(parts) != 2 {
			continue
		}
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])

		pk := prayerKey
		pl := link
		spec := fmt.Sprintf("%d %d * * *", m, h)
		
		Scheduler.AddFunc(spec, func() {
			BroadcastAzan(pk, pl)
		})
	}

	Scheduler.Start()
	log.Println("Azan times updated")
}

func BroadcastAzan(prayerKey, link string) {
	chats, err := GetAllActiveChats()
	if err != nil {
		log.Println("GetAllActiveChats failed:", err)
		return
	}

	for _, chat := range chats {
		if enabled, ok := chat.Prayers[prayerKey]; ok && !enabled {
			continue
		}
		go StartAzanStream(chat.ChatID, prayerKey, link, false)
	}
}

func BroadcastDuas(duas []string, title string) {
	chats, err := GetAllActiveChats()
	if err != nil {
		log.Println("GetAllActiveChats failed:", err)
		return
	}
	if len(duas) == 0 {
		return
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	dua := duas[r.Intn(len(duas))]

	for _, chat := range chats {
		settings, err := GetChatSettings(chat.ChatID)
		if err != nil {
			continue
		}
		if !settings.DuaActive {
			continue
		}

		BotClient.SendMessage(chat.ChatID, &telegram.SendMessageOptions{
			Text: fmt.Sprintf(
				"💫 **%s**\n\n%s\n\n<b>تـقـبـل الله مـنـا ومـنـكـم صـالـح الأعـمـال 🧚</b>",
				title,
				dua,
			),
			ReplyMarkup: nil,
		})
	}
}

func StartAzanStream(chatID int64, prayerKey, link string, forceTest bool) {
	cs, err := core.GetChatState(chatID)
	if err != nil {
		log.Println("GetChatState failed:", err)
		return
	}

	active, _ := cs.IsActiveVC()
	if !active {
		assistant := core.Assistants.Get(chatID)
		if assistant == nil {
			if forceTest {
				BotClient.SendMessage(chatID, &telegram.SendMessageOptions{
					Text:        "No Assistant",
					ReplyMarkup: nil,
				})
			}
			return
		}
		if err2 := assistant.PhoneCreateGroupCall(chatID, ""); err2 != nil {
			log.Println("PhoneCreateGroupCall error:", err2)
		}
		time.Sleep(3 * time.Second)
	}

	if present, _ := cs.IsAssistantPresent(); !present {
		if err2 := cs.TryJoin(); err2 != nil {
			log.Println("Join VC failed:", err2)
		}
		time.Sleep(2 * time.Second)
	}

	if stickerID, ok := PrayerStickers[prayerKey]; ok {
		BotClient.SendSticker(chatID, &telegram.SendStickerOptions{
			Sticker: &telegram.InputFileID{ID: stickerID},
		})
	}

	caption := fmt.Sprintf(
		"🕌 **حـان الآن مـوعـد أذان %s**\n<b>بـالـتـوقـيـت الـمـحـلـي لـمـديـنـة الـقـاهـره 🧚</b>",
		PrayerNamesStretched[prayerKey],
	)

	statusMsg, err := BotClient.SendMessage(chatID, &telegram.SendMessageOptions{
		Text:        caption,
		ReplyMarkup: nil,
	})
	if err != nil {
		log.Println("SendMessage caption failed:", err)
		return
	}

	dummyMsg := &telegram.NewMessage{
		Client: BotClient,
		Message: &telegram.Message{
			Chat:        &telegram.Chat{ID: chatID},
			Text:        link,
			Sender:      &telegram.Peer{ID: config.OwnerID},
			ReplyMarkup: nil,
		},
	}

	tracks, err := platforms.GetTracks(dummyMsg, false)
	if err != nil {
		log.Println("GetTracks failed:", err)
		BotClient.DeleteMessages(chatID, []int{statusMsg.ID})
		return
	}
	if len(tracks) == 0 {
		log.Println("No tracks found for link")
		BotClient.DeleteMessages(chatID, []int{statusMsg.ID})
		return
	}

	track := tracks[0](track.Requester) = "خـدمـة الأذان"

	ctx := context.Background()
	path, err := platforms.Download(ctx, track, statusMsg)
	if err != nil {
		statusMsg.Edit("Download Fail")
		log.Println("Download error:", err)
		return
	}

	if room := core.GetRoom(chatID); room != nil {
		room.Play(track, path, true)
	}

	// === SNIPER MODE: HIDE KEYBOARD ===
	go func() {
		// Quick loop to catch the playing message
		for i := 0; i < 5; i++ {
			time.Sleep(800 * time.Millisecond)
			history, err := BotClient.GetHistory(chatID, 0, 0, 0, 3, 0, 0, 0)
			if err == nil && history != nil {
				for _, m := range history.Messages {
					// Check if message is from bot AND has buttons (Keyboard)
					if m.Sender.ID == BotClient.Self.ID && m.ReplyMarkup != nil {
						BotClient.DeleteMessages(chatID, []int{m.ID})
						return
					}
				}
			}
		}
	}()
}
