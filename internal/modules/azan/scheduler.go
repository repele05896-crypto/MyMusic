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

//━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// تـهـيـئـة جـدول الأذان والأذكار
//━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
func InitAzanScheduler(client *telegram.Client) {
	BotClient = client

	loc, err := time.LoadLocation("Africa/Cairo")
	if err != nil {
		log.Println("⚠️ فشل تحميل التوقيت – سيتم استخدام التوقيت المحلي")
		loc = time.Local
	}

	Scheduler = cron.New(cron.WithLocation(loc))

	Scheduler.AddFunc("5 0 * * *", UpdateAzanTimes)

	Scheduler.AddFunc("0 7 * * *", func() {
		BroadcastDuas(MorningDuas, "أذكار الصباح")
	})

	Scheduler.AddFunc("0 20 * * *", func() {
		BroadcastDuas(NightDuas, "أذكار المساء")
	})

	go UpdateAzanTimes()
	Scheduler.Start()
}

//━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// جـلـب مـواقيت الأذان
//━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
func UpdateAzanTimes() {
	client := http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(
		"http://api.aladhan.com/v1/timingsByCity?city=Cairo&country=Egypt&method=5",
	)
	if err != nil || resp == nil {
		log.Println("❌ فشل جلب مواقيت الأذان")
		return
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Timings map[string]string `json:"timings"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Println("❌ فشل قراءة بيانات الأذان")
		return
	}

	Scheduler.Stop()
	Scheduler = cron.New(cron.WithLocation(Scheduler.Location()))

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

		Scheduler.AddFunc(
			fmt.Sprintf("%d %d * * *", m, h),
			func() {
				BroadcastAzan(pk, pl)
			},
		)
	}

	Scheduler.Start()
	log.Println("✅ تم تحديث مواقيت الأذان")
}

//━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// بث الأذان
//━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
func BroadcastAzan(prayerKey, link string) {
	chats, err := GetAllActiveChats()
	if err != nil {
		return
	}

	for _, chat := range chats {
		if enabled, ok := chat.Prayers[prayerKey]; ok && !enabled {
			continue
		}
		go StartAzanStream(chat.ChatID, prayerKey, link, false)
	}
}

//━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// بث الأذكار
//━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
func BroadcastDuas(duas []string, title string) {
	chats, _ := GetAllActiveChats()
	if len(duas) == 0 {
		return
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	dua := duas[r.Intn(len(duas))]

	for _, chat := range chats {
		settings, _ := GetChatSettings(chat.ChatID)
		if !settings.DuaActive {
			continue
		}

		go BotClient.SendMessage(chat.ChatID, &telegram.SendMessageOptions{
			Text: fmt.Sprintf(
				"💫 **%s**\n\n%s\n\n<b>تـقـبـل الله مـنـا ومـنـكـم صـالـح الأعـمـال 🧚</b>",
				title,
				dua,
			),
			ReplyMarkup: nil, // ⛔ منع أي كيبورد
		})
	}
}

//━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// تشغيل الأذان بدون زر ▶️
//━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
func StartAzanStream(chatID int64, prayerKey, link string, forceTest bool) {
	cs, err := core.GetChatState(chatID)
	if err != nil {
		return
	}

	active, _ := cs.IsActiveVC()
	if !active {
		assistant := core.Assistants.Get(chatID)
		if assistant == nil {
			if forceTest {
				BotClient.SendMessage(chatID, &telegram.SendMessageOptions{
					Text:        "❌ لا يوجد مساعد صوتي",
					ReplyMarkup: nil,
				})
			}
			return
		}
		assistant.PhoneCreateGroupCall(chatID, "")
		time.Sleep(3 * time.Second)
	}

	if present, _ := cs.IsAssistantPresent(); !present {
		cs.TryJoin()
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

	statusMsg, _ := BotClient.SendMessage(chatID, &telegram.SendMessageOptions{
		Text:        caption,
		ReplyMarkup: nil, // ⛔ منع زر التشغيل
	})

	dummyMsg := &telegram.NewMessage{
		Client: BotClient,
		Message: &telegram.Message{
			Chat:        &telegram.Chat{ID: chatID},
			Text:        link,
			Sender:      &telegram.Peer{ID: config.OwnerID},
			ReplyMarkup: nil, // ⛔ منع Inline Keyboard من المنبع
		},
	}

	tracks, err := platforms.GetTracks(dummyMsg, false)
	if err != nil || len(tracks) == 0 {
		BotClient.DeleteMessages(chatID, []int{statusMsg.ID})
		return
	}

	track := tracks[0](dummyMsg.Sender)
	track.Requester = "خـدمـة الأذان"

	ctx := context.Background()
	path, err := platforms.Download(ctx, track, statusMsg)
	if err != nil {
		statusMsg.Edit("❌ فشل تحميل الأذان")
		return
	}

	if room := core.GetRoom(chatID); room != nil {
		room.Play(track, path, true)
	}
}
