package azan

import (
	"context"
	"encoding/json"
	"fmt"
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

var Scheduler *cron.Cron
var BotClient *telegram.Client

func InitAzanScheduler(client *telegram.Client) {
	BotClient = client
	loc, _ := time.LoadLocation("Africa/Cairo")
	Scheduler = cron.New(cron.WithLocation(loc))

	Scheduler.AddFunc("5 0 * * *", UpdateAzanTimes)
	
	Scheduler.AddFunc("0 7 * * *", func() { BroadcastDuas(MorningDuas, "أذكار الصباح") })
	Scheduler.AddFunc("0 20 * * *", func() { BroadcastDuas(NightDuas, "أذكار المساء") })

	go UpdateAzanTimes()
	Scheduler.Start()
}

func UpdateAzanTimes() {
	resp, err := http.Get("http://api.aladhan.com/v1/timingsByCity?city=Cairo&country=Egypt&method=5")
	if err != nil { return }
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Timings map[string]string `json:"timings"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	for name, timeStr := range result.Data.Timings {
		if link, exists := PrayerLinks[name]; exists {
			cleanTime := strings.Split(timeStr, " ")[0]
			parts := strings.Split(cleanTime, ":")
			h, _ := strconv.Atoi(parts[0])
			m, _ := strconv.Atoi(parts[1])

			pName := name
			pLink := link

			Scheduler.AddFunc(fmt.Sprintf("%d %d * * *", m, h), func() {
				BroadcastAzan(pName, pLink)
			})
		}
	}
}

func BroadcastAzan(prayerKey, link string) {
	chats, _ := GetAllActiveChats()
	for _, chat := range chats {
		if enabled, ok := chat.Prayers[prayerKey]; ok && !enabled {
			continue
		}
		go StartAzanStream(chat.ChatID, prayerKey, link, false)
	}
}

func BroadcastDuas(duas []string, title string) {
	chats, _ := GetAllActiveChats()
	rand.Seed(time.Now().UnixNano())
	selectedDua := duas[rand.Intn(len(duas))]

	for _, chat := range chats {
		settings, _ := GetChatSettings(chat.ChatID)
		if !settings.DuaActive { continue }

		go func(cid int64) {
			BotClient.SendMessage(cid, &telegram.SendMessageOptions{
				Text: fmt.Sprintf("💫 **%s**\n\n%s\n\n<b>تـقـبـل الله مـنـا ومـنـكـم صـالـح الاعـمـال 🧚</b>", title, selectedDua),
			})
		}(chat.ChatID)
	}
}

// 🧠 الدالة الذكية للتشغيل
func StartAzanStream(chatID int64, prayerKey, link string, forceTest bool) {
	cs, err := core.GetChatState(chatID)
	if err != nil { return }

	// 1️⃣ فحص الكول وفتحه إجبارياً
	activeVC, _ := cs.IsActiveVC()
	if !activeVC {
		// الكول مغلق، نحاول نفتحه
		assistant := core.Assistants.Get(chatID)
		if assistant != nil {
			assistant.PhoneCreateGroupCall(chatID, "")
			// ننتظر 3 ثواني عشان التليجرام يستوعب
			time.Sleep(3 * time.Second)
		} else {
			if forceTest { BotClient.SendMessage(chatID, &telegram.SendMessageOptions{Text: "⚠️ لا يوجد مساعد في هذا الجروب."}) }
			return
		}
	}

	// 2️⃣ انضمام المساعد
	if present, _ := cs.IsAssistantPresent(); !present {
		cs.TryJoin()
		time.Sleep(2 * time.Second)
	}

	// 3️⃣ إرسال الاستيكر
	if stickerID, ok := PrayerStickers[prayerKey]; ok {
		BotClient.SendSticker(chatID, &telegram.SendStickerOptions{
			Sticker: &telegram.InputFileID{ID: stickerID},
		})
	}

	// 4️⃣ رسالة الأذان
	caption := fmt.Sprintf("🕌 **حـان الآن مـوعـد أذان %s**\n<b>بـالـتـوقـيـت الـمـحـلـي لـمـديـنـة الـقـاهـره 🧚</b>", PrayerNamesStretched[prayerKey])
	statusMsg, _ := BotClient.SendMessage(chatID, &telegram.SendMessageOptions{Text: caption})

	// 5️⃣ تجهيز الأغنية (استخدام config.OwnerID مباشرة بدون مصفوفة)
	dummyMsg := &telegram.NewMessage{
		Client: BotClient,
		Message: &telegram.Message{
			Chat:   &telegram.Chat{ID: chatID},
			Text:   link,
			Sender: &telegram.Peer{ID: config.OwnerID}, 
		},
	}

	tracks, err := platforms.GetTracks(dummyMsg, false)
	if err != nil || len(tracks) == 0 { return }

	track := tracks[0](track.Requester) = "خـدمـة الأذان"

	ctx := context.Background()
	path, err := platforms.Download(ctx, track, statusMsg)
	if err != nil {
		statusMsg.Edit("❌ فـشـل تـحـمـيـل الأذان.")
		return
	}

	r := core.GetRoom(chatID)
	r.Play(track, path, true) 

	// 😈 6️⃣ كود إخفاء الكيبورد (المصيدة)
	go func() {
		// ننتظر ثانية واحدة عشان ندي فرصة للبوت يبعت الكيبورد
		time.Sleep(1200 * time.Millisecond)

		// نجيب آخر 5 رسائل في الشات
		history, err := BotClient.GetHistory(chatID, 0, 0, 0, 5, 0, 0, 0)
		if err == nil && history != nil {
			for _, m := range history.Messages {
				// لو الرسالة من البوت نفسه (BotClient.Self.ID) + فيها أزرار (ReplyMarkup)
				// + ليست رسالة الأذان (التي لا تحتوي على أزرار)
				// إذاً هي رسالة التشغيل، نقوم بحذفها
				if m.Sender.ID == BotClient.Self.ID && m.ReplyMarkup != nil {
					BotClient.DeleteMessages(chatID, []int{m.ID})
					// خلاص مسكناها ومسحناها، نخرج من اللوب
					return 
				}
			}
		}
	}()
}
