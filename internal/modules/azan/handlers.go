// modules/azan/handlers.go
package azan

import (
	"fmt"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"main/internal/config"
)

// معالجة الرسائل النصية (أوامر الأذان)
func CommandHandler(m *telegram.NewMessage) error {
	if m.Sender == nil {
		return nil
	}

	text := m.Text()
	chatID := m.Chat.ID
	senderID := m.Sender.ID

	// لوحة أوامر الأذان
	if text == "اعدادات الاذان" || text == "الاذان" || text == "اوامر الاذان" {
		kb := telegram.InlineKeyboardMarkup{
			Rows: []telegram.InlineKeyboardRow{
				{telegram.InlineKeyboardButton{Text: "أوامـر الـمـالـك", CallbackData: "cmd_owner"}},
				{telegram.InlineKeyboardButton{Text: "أوامـر الـمـشـرفـيـن", CallbackData: "cmd_admin"}},
				{telegram.InlineKeyboardButton{Text: "اغـلاق", CallbackData: "cmd_close"}},
			},
		}
		m.Reply("<b>مـرحـبـاً بـك فـي قـائـمـة أوامـر الأذان</b>\n<b>اخـتـر الـقـائـمـة الـمـنـاسـبـة لـرتـبـتـك مـن الأزرار :</b>", &telegram.SendOptions{ReplyMarkup: kb})
		return nil
	}

	// تفعيل الأذان (أوامر سريعة)
	if text == "تفعيل الاذان" {
		if !IsAdminOrOwner(m) {
			return nil
		}
		settings, _ := GetChatSettings(chatID)
		if settings.AzanActive {
			m.Reply("💫 الاذان مــفــعــل بــالــفــعــل.")
			return nil
		}
		UpdateChatSetting(chatID, "azan_active", true)
		m.Reply("⭐ تــم تــفــعــيــل الاذان بــنــجــاح.")
		return nil
	}

	// قفل الأذان
	if text == "قفل الاذان" {
		if !IsAdminOrOwner(m) {
			return nil
		}
		settings, _ := GetChatSettings(chatID)
		if settings.ForcedActive && !IsOwner(senderID) {
			m.Reply("🧚 <b>عــذرا هــذا أمــر اجــبــاري مــن الــمــالــك</b>")
			return nil
		}
		if !settings.AzanActive {
			m.Reply("💫 الاذان مــعــطــل بــالــفــعــل.")
			return nil
		}
		UpdateChatSetting(chatID, "azan_active", false)
		m.Reply("⭐ تــم قــفــل الاذان بــنــجــاح.")
		return nil
	}

	// تفعيل الأذكار
	if text == "تفعيل الدعاء" {
		UpdateChatSetting(chatID, "dua_active", true)
		m.Reply("🩵 تــم تــفــعــيــل الاذكــار بــنــجــاح.")
		return nil
	}

	// اختبار الأذان (تشغيل تجريبي للصوت)
	if text == "تست الاذان" {
		if !IsAdminOrOwner(m) {
			return nil
		}
		m.Reply("⏳ <b>جــاري تــشــغــيــل الأذان الــتــجــريــبــي . . .</b>")
		go StartAzanStream(chatID, "Fajr", PrayerLinks["Fajr"], true)
		return nil
	}

	return nil
}

// معالج أزرار لوحة التحكم (Callback)
func CallbackHandler(cb *telegram.CallbackQuery) error {
	data := cb.Data
	chatID := cb.Message.Chat.ID
	userID := cb.Sender.ID

	// إغلاق اللوحة
	if data == "cmd_close" || data == "close_panel" {
		cb.Message.Delete()
		return nil
	}

	// أوامر المالك
	if data == "cmd_owner" {
		if !IsOwner(userID) {
			cb.Answer(&telegram.CallbackQueryAnswerOptions{Text: "• عـذرا هـذا الـزر لـلـمـالـك فـقـط 🤍", ShowAlert: true})
			return nil
		}
		text := "<b>أوامــر الــمــالــك (الــســورس) :</b>\n\n• تفعيل الاذان الاجباري\n• فحص الاذان\n• تغيير رابط الاذان"
		kb := telegram.InlineKeyboardMarkup{
			Rows: []telegram.InlineKeyboardRow{
				{telegram.InlineKeyboardButton{Text: "رجـوع", CallbackData: "cmd_back_main"}},
			},
		}
		cb.Message.Edit(text, &telegram.EditOptions{ReplyMarkup: kb})
		return nil
	}

	// أوامر المشرفين والعودة إلى القائمة الرئيسية
	if data == "cmd_admin" || data == "cmd_back_main" {
		if data == "cmd_back_main" {
			kb := telegram.InlineKeyboardMarkup{
				Rows: []telegram.InlineKeyboardRow{
					{telegram.InlineKeyboardButton{Text: "أوامـر الـمـالـك", CallbackData: "cmd_owner"}},
					{telegram.InlineKeyboardButton{Text: "أوامـر الـمـشـرفـيـن", CallbackData: "cmd_admin"}},
					{telegram.InlineKeyboardButton{Text: "اغـلاق", CallbackData: "cmd_close"}},
				},
			}
			cb.Message.Edit("<b>مـرحـبـاً بـك فـي قـائـمـة أوامـر الأذان</b>", &telegram.EditOptions{ReplyMarkup: kb})
			return nil
		}
		// عرض لوحة التحكم في الأذان (تفعيل/تعطيل)
		ShowSettingsPanel(cb.Message, chatID)
		return nil
	}

	// ضبط الإعدادات (تفعيل/تعطيل في اللوحة التفاعلية)
	if strings.HasPrefix(data, "set_") {
		parts := strings.Split(data, "_")
		settings, _ := GetChatSettings(chatID)
		if parts[1] == "main" {
			UpdateChatSetting(chatID, "azan_active", !settings.AzanActive)
		} else if parts[1] == "dua" {
			UpdateChatSetting(chatID, "dua_active", !settings.DuaActive)
		} else if parts[1] == "p" {
			pkey := parts[2]
			currVal := settings.Prayers[pkey]
			UpdatePrayerSetting(chatID, pkey, !currVal)
		}
		ShowSettingsPanel(cb.Message, chatID)
		return nil
	}

	return nil
}

// عرض لوحة تحكم تفاعلية بإعدادات الأذان للمجموعة
func ShowSettingsPanel(msg *telegram.Message, chatID int64) {
	settings, _ := GetChatSettings(chatID)

	stMain := "『 مــعــطــل 』"
	if settings.AzanActive {
		stMain = "『 مــفــعــل 』"
	}
	stDua := "『 مــعــطــل 』"
	if settings.DuaActive {
		stDua = "『 مــفــعــل 』"
	}

	rows := []telegram.InlineKeyboardRow{}
	rows = append(rows, telegram.InlineKeyboardRow{
		telegram.InlineKeyboardButton{Text: "الاذان الـعـام : " + stMain, CallbackData: fmt.Sprintf("set_main_%d", chatID)},
	})
	rows = append(rows, telegram.InlineKeyboardRow{
		telegram.InlineKeyboardButton{Text: "دعـاء الـصـبـاح : " + stDua, CallbackData: fmt.Sprintf("set_dua_%d", chatID)},
	})

	pRow := telegram.InlineKeyboardRow{}
	order := []string{"Fajr", "Dhuhr", "Asr", "Maghrib", "Isha"}
	for _, k := range order {
		isActive := settings.Prayers[k]
		pst := "『 مــعــطــل 』"
		if isActive {
			pst = "『 مــفــعــل 』"
		}
		name := PrayerNamesStretched[k]
		btnText := fmt.Sprintf("%s : %s", name, pst)
		pRow = append(pRow, telegram.InlineKeyboardButton{Text: btnText, CallbackData: fmt.Sprintf("set_p_%s_%d", k, chatID)})
		if len(pRow) == 2 {
			rows = append(rows, pRow)
			pRow = telegram.InlineKeyboardRow{}
		}
	}
	if len(pRow) > 0 {
		rows = append(rows, pRow)
	}
	rows = append(rows, telegram.InlineKeyboardRow{
		telegram.InlineKeyboardButton{Text: "اغـلاق", CallbackData: "close_panel"},
	})

	kb := telegram.InlineKeyboardMarkup{Rows: rows}
	text := fmt.Sprintf("<b>لـوحـة تـحـكـم الأذان ( لـلـجـروب %d ) :</b>", chatID)
	msg.Edit(text, &telegram.EditOptions{ReplyMarkup: kb})
}

// دوال مساعدة للتحقق من الصلاحيات
func IsOwner(userID int64) bool {
	return userID == config.OwnerID
}

func IsAdminOrOwner(m *telegram.NewMessage) bool {
	if IsOwner(m.Sender.ID) {
		return true
	}
	return true // في هذا المثال يتم اعتبار كل المرسلين كمسؤولين بخلاف المالك
}
