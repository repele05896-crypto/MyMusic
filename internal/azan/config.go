package azan

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"main/internal/database" 
)

// --- [ 1. الثوابت والأسماء المطولة ] ---
var PrayerNamesStretched = map[string]string{
	"Fajr":    "الـفـجـر",
	"Dhuhr":   "الـظـهـر",
	"Asr":     "الـعـصـر",
	"Maghrib": "الـمـغـرب",
	"Isha":    "الـعـشـاء",
}

var PrayerLinks = map[string]string{
	"Fajr":    "https://youtu.be/r9AWBlpantg",
	"Dhuhr":   "https://youtu.be/21MuvFr7CK8",
	"Asr":     "https://youtu.be/bb6cNncMdiM",
	"Maghrib": "https://youtu.be/hKPcNh7WHoM",
	"Isha":    "https://youtu.be/hKPcNh7WHoM",
}

// قائمة الاستيكرات
var PrayerStickers = map[string]string{
    "Fajr":    "CAACAgQAAyEFAATHCHTJAAIJD2lOq8aLkRR49evBKiITWWhwtgEoAALoGgACp_FYUQuzqVH-JHS5HgQ",
    "Dhuhr":   "CAACAgQAAyEFAATHCHTJAAIJEWlOrFKzjSDZeWfl6U3F-lrKldRXAAJMGwACMVlYUa15CORC0p0xHgQ",
    "Asr":     "CAACAgQAAyEFAATHCHTJAAIJE2lOrFRQIbcdLfnpdl5PtbdqNyR6AALFGQAC3ZZRUcK5YivXbwUAAR4E",
    "Maghrib": "CAACAgQAAyEFAATHCHTJAAIJFWlOrFT4eOnPJDsSuU6Ya-V0WPQdAALfFwACcIVQUX6NcNNCxvdRHgQ",
    "Isha":    "CAACAgQAAyEFAATHCHTJAAIJF2lOrFVxhRGefHki3d4s-hLC9cKHAALqHAAC3oZQUWqQdvdwXnGLHgQ",
}

// --- [ 2. الأدعية (تم استبدال الايموجي بـ 🤍 🤎 🩵) ] ---
var MorningDuas = []string{
	"اللهم بك أصبحنا، وبك أمسينا، وبك نحيا، وبك نموت، وإليك النشور 🤍",
	"أصبحنا وأصبح الملك لله، والحمد لله، لا إله إلا الله وحده لا شريك له 🩵",
	"اللهم إني أسألك خير هذا اليوم، فتحه، ونصره، ونوره، وبركته، وهداه 🤎",
	"رضيت بالله رباً، وبالإسلام ديناً، وبمحمد صلى الله عليه وسلم نبياً 🤍",
	"يا حي يا قيوم برحمتك أستغيث، أصلح لي شأني كله ولا تكلني إلى نفسي طرفة عين 🩵",
	"اللهم أنت ربي لا إله إلا أنت، خلقتني وأنا عبدك، وأنا على عهدك ووعدك ما استطعت 🤎",
	"اللهم إني أسألك علماً نافعاً، ورزقاً طيباً، وعملاً متقبلاً 🤍",
	"بسم الله الذي لا يضر مع اسمه شيء في الأرض ولا في السماء وهو السميع العليم 🩵",
	"اللهم عافني في بدني، اللهم عافني في سمعي، اللهم عافني في بصري 🤎",
	"اللهم إني أسألك العفو والعافية في ديني ودنياي وأهلي ومالي 🤍",
	"أصبحنا على فطرة الإسلام، وعلى كلمة الإخلاص، وعلى دين نبينا محمد 🩵",
	"اللهم اجعل صباحنا هذا صباحاً مباركاً، تفتح لنا فيه أبواب رحمتك 🤎",
	"ربي أسألك في هذا الصباح أن تريح قلبي وفكري 🤍",
	"حسبي الله لا إله إلا هو، عليه توكلت وهو رب العرش العظيم (7 مرات) 🩵",
}

var NightDuas = []string{
	"اللهم بك أمسينا، وبك أصبحنا، وبك نحيا، وبك نموت، وإليك المصير 🤎",
	"أمسينا وأمسى الملك لله، والحمد لله، لا إله إلا الله وحده لا شريك له 🤍",
	"اللهم أنت ربي لا إله إلا أنت، خلقتني وأنا عبدك، وأنا على عهدك ووعدك ما استطعت 🩵",
	"اللهم إني أسألك العفو والعافية في الدنيا والآخرة 🤎",
	"اللهم استر عوراتي وآمن روعاتي، اللهم احفظني من بين يدي ومن خلفي 🤍",
	"اللهم عافني في بدني، اللهم عافني في سمعي، اللهم عافني في بصري 🩵",
	"اللهم إني أعوذ بك من الكفر والفقر، وأعوذ بك من عذاب القبر 🤎",
	"حسبي الله لا إله إلا هو عليه توكلت وهو رب العرش العظيم 🤍",
	"بسم الله الذي لا يضر مع اسمه شيء في الأرض ولا في السماء 🩵",
	"يا حي يا قيوم برحمتك أستغيث، أصلح لي شأني كله ولا تكلني إلى نفسي طرفة عين 🤎",
	"أمسينا على فطرة الإسلام، وعلى كلمة الإخلاص، وعلى دين نبينا محمد 🤍",
}

// --- [ 3. إعدادات قاعدة البيانات ] ---
type ChatAzanSettings struct {
	ChatID         int64           `bson:"chat_id"`
	AzanActive     bool            `bson:"azan_active"`
	ForcedActive   bool            `bson:"forced_active"`
	DuaActive      bool            `bson:"dua_active"`
	NightDuaActive bool            `bson:"night_dua_active"`
	Prayers        map[string]bool `bson:"prayers"`
}

func GetChatSettings(chatID int64) (*ChatAzanSettings, error) {
	var settings ChatAzanSettings
	collection := database.MongoDB.Collection("azan_settings")

	filter := bson.M{"chat_id": chatID}
	err := collection.FindOne(context.TODO(), filter).Decode(&settings)

	if err != nil {
		newDoc := ChatAzanSettings{
			ChatID:         chatID,
			AzanActive:     true,
			DuaActive:      true,
			NightDuaActive: true,
			Prayers:        map[string]bool{"Fajr": true, "Dhuhr": true, "Asr": true, "Maghrib": true, "Isha": true},
		}
		collection.InsertOne(context.TODO(), newDoc)
		return &newDoc, nil
	}
	if settings.Prayers == nil {
		settings.Prayers = map[string]bool{"Fajr": true, "Dhuhr": true, "Asr": true, "Maghrib": true, "Isha": true}
	}
	return &settings, nil
}

func UpdateChatSetting(chatID int64, key string, value interface{}) {
	collection := database.MongoDB.Collection("azan_settings")
	opts := options.Update().SetUpsert(true)
	update := bson.M{"$set": bson.M{key: value}}
	collection.UpdateOne(context.TODO(), bson.M{"chat_id": chatID}, update, opts)
}

func UpdatePrayerSetting(chatID int64, prayerKey string, value bool) {
	collection := database.MongoDB.Collection("azan_settings")
	opts := options.Update().SetUpsert(true)
	update := bson.M{"$set": bson.M{fmt.Sprintf("prayers.%s", prayerKey): value}}
	collection.UpdateOne(context.TODO(), bson.M{"chat_id": chatID}, update, opts)
}

func GetAllActiveChats() ([]ChatAzanSettings, error) {
	var results []ChatAzanSettings
	cursor, err := database.MongoDB.Collection("azan_settings").Find(context.TODO(), bson.M{"azan_active": true})
	if err != nil { return nil, err }
	cursor.All(context.TODO(), &results)
	return results, nil
}
