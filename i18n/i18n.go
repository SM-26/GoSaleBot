package i18n

import (
    "fmt"
)
var Messages = map[string]map[string]string{
	"en": {
		"welcome": "Welcome! Let's create a sale post. Please enter the title:",
		"start": "Send /start to begin creating a sale post.",
		"enter_description": "Enter a description:",
		"enter_price": "Enter the price:",
		"enter_location": "Enter the location:",
		"send_photos": "Send one or more photos (type 'done' when finished):",
		"photo_received": "Photo received. Send another or type 'done'.",
		"preview": "Preview:\nTitle: %s\nDescription: %s\nPrice: %s\nLocation: %s\nPhotos: %d\nSend 'confirm' to submit or 'cancel' to abort.",
		"send_photo_or_done": "Send a photo or type 'done' when finished.",
		"failed_save": "Failed to save post. Please try again.",
		"post_saved_failed_forward": "Post saved, but failed to forward to moderation group.",
		"post_submitted": "Post submitted for moderation!",
		"post_cancelled": "Post creation cancelled.",
		"send_confirm_or_cancel": "Send 'confirm' to submit or 'cancel' to abort.",
		"session_reset": "Session reset. Send /start to begin.",
		"post_rejected": "Your post was rejected: %s",
	},
	"cz": {
		"welcome": "Vítejte! Pojďme vytvořit prodejní příspěvek. Zadejte prosím název:",
		"start": "Pošlete /start pro zahájení vytváření prodejního příspěvku.",
		"enter_description": "Zadejte popis:",
		"enter_price": "Zadejte cenu:",
		"enter_location": "Zadejte lokalitu:",
		"send_photos": "Pošlete jednu nebo více fotografií (napište 'done' až skončíte):",
		"photo_received": "Fotografie přijata. Pošlete další nebo napište 'done'.",
		"preview": "Náhled:\nNázev: %s\nPopis: %s\nCena: %s\nLokalita: %s\nFotografií: %d\nPošlete 'confirm' pro odeslání nebo 'cancel' pro zrušení.",
		"send_photo_or_done": "Pošlete fotografii nebo napište 'done' až skončíte.",
		"failed_save": "Nepodařilo se uložit příspěvek. Zkuste to prosím znovu.",
		"post_saved_failed_forward": "Příspěvek uložen, ale nepodařilo se jej předat ke schválení.",
		"post_submitted": "Příspěvek byl odeslán ke schválení!",
		"post_cancelled": "Vytváření příspěvku bylo zrušeno.",
		"send_confirm_or_cancel": "Pošlete 'confirm' pro odeslání nebo 'cancel' pro zrušení.",
		"session_reset": "Relace byla resetována. Pošlete /start pro zahájení.",
		"post_rejected": "Váš příspěvek byl zamítnut: %s",
	},
	"he": {
		"welcome": "ברוך הבא! בוא ניצור פוסט מכירה. אנא הכנס כותרת:",
		"start": "שלח /start כדי להתחיל ליצור פוסט מכירה.",
		"enter_description": "הכנס תיאור:",
		"enter_price": "הכנס מחיר:",
		"enter_location": "הכנס מיקום:",
		"send_photos": "שלח תמונה אחת או יותר (כתוב 'done' כשתסיים):",
		"photo_received": "התמונה התקבלה. שלח עוד או כתוב 'done'.",
		"preview": "תצוגה מקדימה:\nכותרת: %s\nתיאור: %s\nמחיר: %s\nמיקום: %s\nמספר תמונות: %d\nשלח 'confirm' לאישור או 'cancel' לביטול.",
		"send_photo_or_done": "שלח תמונה או כתוב 'done' כשתסיים.",
		"failed_save": "שמירת הפוסט נכשלה. נסה שוב.",
		"post_saved_failed_forward": "הפוסט נשמר, אך לא נשלח לקבוצת המנהלים.",
		"post_submitted": "הפוסט נשלח לאישור!",
		"post_cancelled": "יצירת הפוסט בוטלה.",
		"send_confirm_or_cancel": "שלח 'confirm' לאישור או 'cancel' לביטול.",
		"session_reset": "הסשן אופס. שלח /start כדי להתחיל.",
		"post_rejected": "הפוסט שלך נדחה: %s",
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
