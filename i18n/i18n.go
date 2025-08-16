package i18n

import (
	"fmt"
)

var Messages = map[string]map[string]string{
	"en": {
		"welcome":                   "Welcome! Let's create a sale post. Please enter the title:",
		"unknown_admin_command":     "Unknown admin command.",
		"start":                     "Send /start to begin creating a sale post.",
		"session_reset":             "Session reset. Send /start to begin.",
		"send_photos":               "Send one or more photos (type 'done' when finished):",
		"send_photo_or_done":        "Send a photo or type 'done' when finished.",
		"send_confirm_or_cancel":    "Send 'confirm' to submit or 'cancel' to abort.",
		"preview":                   "Preview:\nTitle: %s\nDescription: %s\nPrice: %s\nLocation: %s\nPhotos: %d",
		"post_submitted":            "Post submitted for moderation!",
		"post_sent_for_approval":    "Post sent for approval!",
		"post_saved_failed_forward": "Post saved, but failed to forward to moderation group.",
		"post_rejected":             "Your post was rejected: %s",
		"post_cancelled":            "Post creation cancelled.",
		"post_approved":             "Your post has been approved and published!",
		"photo_received":            "Photo received. Send another or type 'done'.",
		"not_authorized":            "You are not authorized to use this command.",
		"moderation_preview":        "New Sale Post:\nTitle: %s\nDescription: %s\nPrice: %s\nLocation: %s\nStatus: pending %s",
		"for_sale":                  "FOR SALE! \nTitle: %s\nDescription: %s\nPrice: %s\nLocation: %s\nPosted by: %s",
		"failed_save":               "Failed to save post. Please try again.",
		"failed_clear_users":        "Failed to clear users: %s",
		"failed_clear_posts":        "Failed to clear posts: %s",
		"failed_clear_photos":       "Failed to clear photos: %s",
		"enter_price":               "Enter the price:",
		"enter_location":            "Enter the location:",
		"enter_description":         "Enter a description:",
		"db_cleared":                "Database cleared (photos, posts).",
		"config_updated":            "Config updated: %s = %s",
		"config_usage":              "Usage: /config KEY VALUE",
		"failed_update_config":      "Failed to update config: %s",
	},
	"cz": {
		"post_approved":             "Váš příspěvek byl schválen a zveřejněn!",
		"welcome":                   "Vítejte! Pojďme vytvořit prodejní příspěvek. Zadejte prosím název:",
		"start":                     "Pošlete /start pro zahájení vytváření prodejního příspěvku.",
		"enter_description":         "Zadejte popis:",
		"enter_price":               "Zadejte cenu:",
		"enter_location":            "Zadejte lokalitu:",
		"send_photos":               "Pošlete jednu nebo více fotografií (napište 'done' až skončíte):",
		"photo_received":            "Fotografie přijata. Pošlete další nebo napište 'done'.",
		"preview":                   "Náhled:\nNázev: %s\nPopis: %s\nCena: %s\nLokalita: %s\nFotografií: %d\nPošlete 'confirm' pro odeslání nebo 'cancel' pro zrušení.",
		"send_photo_or_done":        "Pošlete fotografii nebo napište 'done' až skončíte.",
		"failed_save":               "Nepodařilo se uložit příspěvek. Zkuste to prosím znovu.",
		"post_saved_failed_forward": "Příspěvek uložen, ale nepodařilo se jej předat ke schválení.",
		"post_submitted":            "Příspěvek byl odeslán ke schválení!",
		"post_cancelled":            "Vytváření příspěvku bylo zrušeno.",
		"send_confirm_or_cancel":    "Pošlete 'confirm' pro odeslání nebo 'cancel' pro zrušení.",
		"session_reset":             "Relace byla resetována. Pošlete /start pro zahájení.",
		"post_rejected":             "Váš příspěvek byl zamítnut: %s",
		"not_authorized":            "Nemáte oprávnění používat tento příkaz.",
		"failed_clear_photos":       "Nepodařilo se vymazat fotografie: %s",
		"failed_clear_posts":        "Nepodařilo se vymazat příspěvky: %s",
		"failed_clear_users":        "Nepodařilo se vymazat uživatele: %s",
		"db_cleared":                "Databáze byla vymazána (fotky, příspěvky, uživatelé).",
		"config_updated":            "Konfigurace aktualizována: %s = %s",
		"config_usage":              "Použití: /config KLÍČ HODNOTA",
		"unknown_admin_command":     "Neznámý administrátorský příkaz.",
		"moderation_preview":        "Nový prodejní příspěvek:\nNázev: %s\nPopis: %s\nCena: %s\nLokalita: %s\nStav: čeká na schválení %s",
		"post_sent_for_approval":    "Příspěvek odeslán ke schválení!",
		"for_sale":                  "NA PRODEJ! \nNázev: %s\nPopis: %s\nCena: %s\nLokalita: %s\nZveřejnil/a: %s",
		"failed_update_config":      "Failed to update config: %s",
	},
	"he": {
		"welcome":                   "ברוך הבא! בוא ניצור פוסט מכירה. אנא הכנס כותרת:",
		"unknown_admin_command":     "פקודת אדמין לא מוכרת.",
		"start":                     "שלח /start כדי להתחיל ליצור פוסט מכירה.",
		"session_reset":             "הסשן אופס. שלח /start כדי להתחיל.",
		"send_photos":               "שלח תמונה אחת או יותר (כתוב 'done' כשתסיים):",
		"send_photo_or_done":        "שלח תמונה או כתוב 'done' כשתסיים.",
		"send_confirm_or_cancel":    "שלח 'confirm' לאישור או 'cancel' לביטול.",
		"preview":                   "תצוגה מקדימה:\nכותרת: %s\nתיאור: %s\nמחיר: %s\nמיקום: %s\nמספר תמונות: %d",
		"post_submitted":            "הפוסט נשלח לאישור!",
		"post_sent_for_approval":    "הפוסט נשלח לאישור!",
		"post_saved_failed_forward": "הפוסט נשמר, אך לא נשלח לקבוצת המנהלים.",
		"post_rejected":             "הפוסט שלך נדחה: %s",
		"post_cancelled":            "יצירת הפוסט בוטלה.",
		"post_approved":             "הפוסט שלך אושר ופורסם!",
		"photo_received":            "התמונה התקבלה. שלח עוד או כתוב 'done'.",
		"not_authorized":            "אין לך הרשאה להשתמש בפקודה זו.",
		"moderation_preview":        "פוסט מכירה חדש:\nכותרת: %s\nתיאור: %s\nמחיר: %s\nמיקום: %s\nסטטוס: ממתין לאישור %s",
		"for_sale":                  "למכירה! \nכותרת: %s\nתיאור: %s\nמחיר: %s\nמיקום: %s\nפורסם על ידי: %s",
		"failed_save":               "שמירת הפוסט נכשלה. נסה שוב.",
		"failed_clear_users":        "נכשל בניקוי משתמשים: %s",
		"failed_clear_posts":        "נכשל בניקוי פוסטים: %s",
		"failed_clear_photos":       "נכשל בניקוי תמונות: %s",
		"enter_price":               "הכנס מחיר:",
		"enter_location":            "הכנס מיקום:",
		"enter_description":         "הכנס תיאור:",
		"db_cleared":                "המסד נתונים נוקה (תמונות, פוסטים).",
		"config_updated":            "הגדרה עודכנה: %s = %s",
		"config_usage":              "שימוש: /config מפתח ערך",
		"failed_update_config":      "Failed to update config: %s",
	},
	// Add more languages here
}

func T(lang, key string, args ...interface{}) string {
	if m, ok := Messages[lang]; ok {
		if msg, ok := m[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(msg, args...)
			}
			return msg
		}
	}
	// fallback to English
	if msg, ok := Messages["en"][key]; ok {
		if len(args) > 0 {
			return fmt.Sprintf(msg, args...)
		}
		return msg
	}
	return key
}
