package notify

import (
	"fmt"
	"strings"
)

// template is one notification type's title and body in Kazakh, Russian and English.
// Placeholders such as {event} are filled from the notification's params.
type template struct{ title, body map[string]string }

func t(en, ru, kk string) map[string]string { return map[string]string{"en": en, "ru": ru, "kk": kk} }

var catalog = map[string]template{
	"order_confirmed": {
		t("Your tickets for {event}", "Ваши билеты на {event}", "{event}: билеттеріңіз"),
		t("Order {number} is confirmed. Tickets: {n}. Total: {total} KZT (DEMO payment, no real money moved). Your tickets are attached as a PDF and in your account.",
			"Заказ {number} подтверждён. Билетов: {n}. Итого: {total} ₸ (ДЕМО-оплата, реальные деньги не списаны). Билеты во вложении (PDF) и в вашем аккаунте.",
			"{number} тапсырысы расталды. Билеттер: {n}. Барлығы: {total} ₸ (ДЕМО-төлем, нақты ақша алынбады). Билеттер PDF түрінде тіркелген және аккаунтыңызда бар."),
	},
	"registration_confirmed": {
		t("Your tickets for {event}", "Ваши билеты на {event}", "{event}: билеттеріңіз"),
		t("Registration {number} is confirmed. Tickets: {n}. They are attached as a PDF and in your account.",
			"Регистрация {number} подтверждена. Билетов: {n}. Они во вложении (PDF) и в вашем аккаунте.",
			"{number} тіркеуі расталды. Билеттер: {n}. Олар PDF түрінде тіркелген және аккаунтыңызда бар."),
	},
	"payment_failed": {
		t("Payment failed for order {number}", "Оплата заказа {number} не прошла", "{number} тапсырысы бойынша төлем өтпеді"),
		t("No tickets were issued and no money was taken. You can try again.",
			"Билеты не выпущены, деньги не списаны. Можно попробовать снова.",
			"Билеттер берілмеді, ақша алынбады. Қайталап көруге болады."),
	},
	"refund_completed": {
		t("Refund completed for order {number}", "Возврат по заказу {number} выполнен", "{number} тапсырысы бойынша ақша қайтарылды"),
		t("{total} KZT for {event} was refunded (demo simulation, no real money moved). Your tickets are no longer valid.",
			"{total} ₸ за {event} возвращены (демо-симуляция, реальные деньги не двигались). Ваши билеты больше не действуют.",
			"{event} үшін {total} ₸ қайтарылды (демо-имитация, нақты ақша қозғалмады). Билеттеріңіз енді жарамсыз."),
	},
	"registration_cancelled": {
		t("Registration cancelled: {event}", "Регистрация отменена: {event}", "Тіркеу тоқтатылды: {event}"),
		t("The organizer cancelled your registration {number}.", "Организатор отменил вашу регистрацию {number}.", "Ұйымдастырушы {number} тіркеуіңізді тоқтатты."),
	},
	"event_cancelled": {
		t("Cancelled: {event}", "Отменено: {event}", "Тоқтатылды: {event}"),
		t("The organizer cancelled this event. Reason: {reason}. Your tickets are no longer valid; paid orders are refunded automatically.",
			"Организатор отменил событие. Причина: {reason}. Ваши билеты больше не действуют; оплаченные заказы возвращаются автоматически.",
			"Ұйымдастырушы бұл іс-шараны тоқтатты. Себебі: {reason}. Билеттеріңіз енді жарамсыз; төленген тапсырыстар автоматты түрде қайтарылады."),
	},
	"event_updated": {
		t("Event updated: {event}", "Событие изменено: {event}", "Іс-шара өзгерді: {event}"),
		t("The date, time or venue of an event you have tickets for has changed. Please check the event page.",
			"У события, на которое у вас есть билеты, изменились дата, время или место. Проверьте страницу события.",
			"Билетіңіз бар іс-шараның күні, уақыты немесе орны өзгерді. Іс-шара бетін тексеріңіз."),
	},
	"event_suspended": {
		t("Your event \"{event}\" was suspended", "Ваше событие «{event}» заблокировано", "«{event}» іс-шараңыз бұғатталды"),
		t("Reason: {reason}. Sales are stopped. Contact BiletFlow support to appeal.",
			"Причина: {reason}. Продажи остановлены. Чтобы обжаловать, напишите в поддержку BiletFlow.",
			"Себебі: {reason}. Сатылым тоқтатылды. Шағымдану үшін BiletFlow қолдауына жазыңыз."),
	},
	"paid_sales_active": {
		t("Paid sales active: {event}", "Платные продажи активны: {event}", "Ақылы сатылым белсенді: {event}"),
		t("Attendees can now buy paid tickets. Payouts are simulated in this demo.",
			"Теперь участники могут покупать платные билеты. Выплаты в демо симулированы.",
			"Енді қатысушылар ақылы билет сатып ала алады. Бұл демода төлемдер имитацияланған."),
	},
	"paid_sales_suspended": {
		t("Paid sales suspended: {event}", "Платные продажи приостановлены: {event}", "Ақылы сатылым тоқтатылды: {event}"),
		t("BiletFlow paused paid ticket sales for this event. Free registration stays open. Contact support for details.",
			"BiletFlow приостановил продажу платных билетов. Бесплатная регистрация открыта. Подробности — в поддержке.",
			"BiletFlow бұл іс-шараның ақылы билет сатылымын тоқтатты. Тегін тіркеу ашық. Толығырақ — қолдау қызметінде."),
	},
	"paid_sales_resumed": {
		t("Paid sales resumed: {event}", "Платные продажи возобновлены: {event}", "Ақылы сатылым қайта басталды: {event}"),
		t("Attendees can buy paid tickets again.", "Участники снова могут покупать платные билеты.", "Қатысушылар қайтадан ақылы билет сата алады."),
	},
	"staff_assigned": {
		t("You are door staff for {event}", "Вы на входе: {event}", "Сіз кіре берістесіз: {event}"),
		t("{organizer} added you as an Event Admin. Use the BiletFlow check-in app to scan tickets.",
			"{organizer} добавил(а) вас администратором события. Сканируйте билеты в приложении BiletFlow.",
			"{organizer} сізді іс-шара әкімшісі етіп қосты. Билеттерді BiletFlow қосымшасымен сканерлеңіз."),
	},
	"support_new_case": {
		t("New support case #{number}: {subject}", "Новое обращение №{number}: {subject}", "Жаңа өтініш №{number}: {subject}"),
		t("{preview}", "{preview}", "{preview}"),
	},
	"support_reply": {
		t("New reply on case #{number}", "Новый ответ в обращении №{number}", "№{number} өтінішке жаңа жауап"),
		t("{preview}", "{preview}", "{preview}"),
	},
	"support_status": {
		t("Case #{number} is now: {status}", "Статус обращения №{number}: {status}", "№{number} өтініштің күйі: {status}"),
		t("", "", ""),
	},
	"support_escalated": {
		t("Escalated case #{number}: {subject}", "Переданное обращение №{number}: {subject}", "Жіберілген өтініш №{number}: {subject}"),
		t("An organizer case needs a BiletFlow administrator.", "Обращение организатора требует администратора BiletFlow.", "Ұйымдастырушы өтінішіне BiletFlow әкімшісі қажет."),
	},
}

var supportStatusNames = map[string]map[string]string{
	"open":                 t("Open", "Открыто", "Ашық"),
	"in_progress":          t("In progress", "В работе", "Жұмыста"),
	"waiting_for_customer": t("Waiting for customer", "Ждём ответа клиента", "Клиенттің жауабы күтілуде"),
	"resolved":             t("Resolved", "Решено", "Шешілді"),
}

// Known reports whether a notification type has a template.
func Known(kind string) bool { _, ok := catalog[kind]; return ok }

// Render returns the title and body of a notification in the given locale.
func Render(kind, locale string, params map[string]any) (string, string) {
	tpl, ok := catalog[kind]
	if !ok {
		return kind, ""
	}
	if _, ok := tpl.title[locale]; !ok {
		locale = "en"
	}
	values := map[string]string{}
	for k, v := range params {
		values[k] = fmt.Sprint(v)
	}
	if status, ok := values["status"]; ok {
		if names, ok := supportStatusNames[status]; ok {
			values["status"] = names[locale]
		}
	}
	fill := func(s string) string {
		for k, v := range values {
			s = strings.ReplaceAll(s, "{"+k+"}", v)
		}
		return s
	}
	return fill(tpl.title[locale]), fill(tpl.body[locale])
}
