// Kazakh / Russian / English for the check-in app. English text is the key (same as the web app).
// Server messages arrive in English and are translated here too; a missing phrase shows in English.
// Run `npm run i18n:check` to list phrases without a translation.
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { storage } from './api/storage';

export type Locale = 'kk' | 'ru' | 'en';
type Params = Record<string, string | number>;

const KEY = 'biletflow_locale';
let active: Locale = 'ru';

/** Current language, for code outside React (the API client sends it as X-Locale). */
export const currentLocale = () => active;

export function t(en: string, params?: Params): string {
  let out = active === 'en' ? en : phrases[en]?.[active] ?? en;
  if (params) for (const [k, v] of Object.entries(params)) out = out.split(`{${k}}`).join(String(v));
  return out;
}

type I18n = { locale: Locale; setLocale: (l: Locale) => void; t: typeof t };
const I18nContext = createContext<I18n>({ locale: active, setLocale: () => undefined, t });

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setState] = useState<Locale>(active);
  useEffect(() => {
    storage.get(KEY).then((saved) => {
      if (saved === 'kk' || saved === 'ru' || saved === 'en') { active = saved; setState(saved); }
    }).catch(() => undefined);
  }, []);
  const setLocale = useCallback((l: Locale) => {
    active = l;
    setState(l);
    storage.set(KEY, l).catch(() => undefined);
  }, []);
  // A new `t` identity per language makes every screen that uses the hook re-render.
  const value = useMemo<I18n>(() => ({ locale, setLocale, t: (en, p) => t(en, p) }), [locale, setLocale]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export const useI18n = () => useContext(I18nContext);

const LABELS: Record<Locale, string> = { kk: 'Қаз', ru: 'Рус', en: 'Eng' };

export function LanguagePicker() {
  const { locale, setLocale } = useI18n();
  return (
    <View style={styles.row} accessibilityRole="radiogroup" accessibilityLabel={t('Language')}>
      {(['kk', 'ru', 'en'] as Locale[]).map((l) => (
        <Pressable key={l} accessibilityRole="radio" accessibilityState={{ selected: locale === l }} onPress={() => setLocale(l)} hitSlop={6}
          style={[styles.btn, locale === l && styles.on]}>
          <Text style={[styles.text, locale === l && styles.onText]}>{LABELS[l]}</Text>
        </Pressable>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', gap: 6, alignSelf: 'center' },
  btn: { borderWidth: 1, borderColor: '#ddd', borderRadius: 8, paddingVertical: 6, paddingHorizontal: 12, backgroundColor: '#fff' },
  on: { backgroundColor: '#1a73e8', borderColor: '#1a73e8' },
  text: { color: '#666', fontWeight: '600', fontSize: 14 },
  onText: { color: '#fff' },
});

export const phrases: Record<string, { kk: string; ru: string }> = {
  'Language': { kk: 'Тіл', ru: 'Язык' },
  'Back': { kk: 'Артқа', ru: 'Назад' },
  // Sign-in
  'BiletFlow Check-in': { kk: 'BiletFlow кіруді тексеру', ru: 'BiletFlow: контроль входа' },
  'Sign in as event staff': { kk: 'Іс-шара қызметкері ретінде кіріңіз', ru: 'Войдите как сотрудник события' },
  'Email': { kk: 'Эл. пошта', ru: 'Эл. почта' },
  'Password': { kk: 'Құпиясөз', ru: 'Пароль' },
  'Sign in': { kk: 'Кіру', ru: 'Войти' },
  'Please enter your email and password.': { kk: 'Эл. пошта мен құпиясөзді енгізіңіз.', ru: 'Введите эл. почту и пароль.' },
  'This app is only for event staff. Attendees can find their tickets on the BiletFlow website.': { kk: 'Бұл қолданба тек іс-шара қызметкерлеріне арналған. Қатысушылар билеттерін BiletFlow сайтынан таба алады.', ru: 'Это приложение только для сотрудников событий. Участники найдут свои билеты на сайте BiletFlow.' },
  'Connected to the BiletFlow backend': { kk: 'BiletFlow серверіне қосылған', ru: 'Подключено к серверу BiletFlow' },
  'Demo data · admin@biletflow.kz / admin123': { kk: 'Демо деректер · admin@biletflow.kz / admin123', ru: 'Демо-данные · admin@biletflow.kz / admin123' },
  'Email or password is incorrect.': { kk: 'Эл. пошта немесе құпиясөз қате.', ru: 'Неверная почта или пароль.' },
  'Please verify your email first (check the link we sent you).': { kk: 'Алдымен эл. поштаңызды растаңыз (біз жіберген сілтемені тексеріңіз).', ru: 'Сначала подтвердите почту (проверьте отправленную ссылку).' },

  // Screen titles and events
  'My Events': { kk: 'Менің іс-шараларым', ru: 'Мои события' },
  'Event': { kk: 'Іс-шара', ru: 'Событие' },
  'Scan ticket': { kk: 'Билетті сканерлеу', ru: 'Сканировать билет' },
  'Result': { kk: 'Нәтиже', ru: 'Результат' },
  'Search attendee': { kk: 'Қатысушыны іздеу', ru: 'Найти участника' },
  'Sign out': { kk: 'Шығу', ru: 'Выйти' },
  'Sign out?': { kk: 'Шығасыз ба?', ru: 'Выйти?' },
  'You will need to sign in again to scan tickets.': { kk: 'Билеттерді сканерлеу үшін қайта кіру қажет болады.', ru: 'Чтобы сканировать билеты, нужно будет войти снова.' },
  'Cancel': { kk: 'Бас тарту', ru: 'Отмена' },
  'Signed in as {email}': { kk: 'Кірген: {email}', ru: 'Вы вошли как {email}' },
  'You are not assigned to any events yet.': { kk: 'Сізге әлі ешбір іс-шара тағайындалмаған.', ru: 'Вам пока не назначено ни одного события.' },
  'Event not found': { kk: 'Іс-шара табылмады', ru: 'Событие не найдено' },
  'Registered': { kk: 'Тіркелген', ru: 'Зарегистрировано' },
  'Checked in': { kk: 'Кірді', ru: 'Прошли' },
  'Published': { kk: 'Жарияланған', ru: 'Опубликовано' },
  'Draft': { kk: 'Жоба', ru: 'Черновик' },
  'Cancelled': { kk: 'Жойылған', ru: 'Отменено' },
  'Not published': { kk: 'Жарияланбаған', ru: 'Не опубликовано' },

  // Scanner
  'Camera access is needed to scan tickets.': { kk: 'Билеттерді сканерлеу үшін камераға рұқсат қажет.', ru: 'Чтобы сканировать билеты, нужен доступ к камере.' },
  'Allow camera': { kk: 'Камераға рұқсат беру', ru: 'Разрешить камеру' },
  'Point the camera at the ticket QR code': { kk: 'Камераны билеттің QR-кодына бағыттаңыз', ru: 'Наведите камеру на QR-код билета' },
  'Test without a QR (demo data):': { kk: 'QR-сыз тексеру (демо деректер):', ru: 'Проверка без QR (демо-данные):' },
  'Valid': { kk: 'Жарамды', ru: 'Действителен' },
  'Used': { kk: 'Пайдаланылған', ru: 'Использован' },
  'Refunded': { kk: 'Қайтарылған', ru: 'Возвращён' },
  'Promo QR': { kk: 'Промо QR', ru: 'Промо-QR' },
  'Fake': { kk: 'Жалған', ru: 'Поддельный' },

  // Result
  'Valid ticket': { kk: 'Билет жарамды', ru: 'Билет действителен' },
  'Already checked in': { kk: 'Бұрын кірген', ru: 'Уже прошёл' },
  'Ticket cancelled': { kk: 'Билет жойылған', ru: 'Билет отменён' },
  'Ticket refunded': { kk: 'Билет ақшасы қайтарылған', ru: 'За билет вернули деньги' },
  'Ticket is for a different event': { kk: 'Билет басқа іс-шараға арналған', ru: 'Билет на другое событие' },
  'Promo QR code, not a ticket': { kk: 'Бұл билет емес, промо QR-код', ru: 'Это промо-QR-код, а не билет' },
  'Invalid ticket': { kk: 'Билет жарамсыз', ru: 'Недействительный билет' },
  'Check-in closed: the event is not published': { kk: 'Кіру жабық: іс-шара жарияланбаған', ru: 'Вход закрыт: событие не опубликовано' },
  'Checking ticket…': { kk: 'Билет тексерілуде…', ru: 'Проверяем билет…' },
  'Could not complete': { kk: 'Орындау мүмкін болмады', ru: 'Не удалось выполнить' },
  'Checked in at {time}': { kk: 'Кірген уақыты: {time}', ru: 'Прошёл в {time}' },
  'Check in': { kk: 'Кіргізу', ru: 'Пропустить' },
  'Checking in…': { kk: 'Кіргізілуде…', ru: 'Пропускаем…' },
  'Undo check-in': { kk: 'Кіруді болдырмау', ru: 'Отменить проход' },
  'Scan next': { kk: 'Келесісін сканерлеу', ru: 'Сканировать следующий' },

  // Search
  'Name or ticket code': { kk: 'Аты немесе билет коды', ru: 'Имя или код билета' },
  'No attendees found': { kk: 'Қатысушылар табылмады', ru: 'Участники не найдены' },
  'Not checked in': { kk: 'Кірмеген', ru: 'Не прошёл' },
  'Try again': { kk: 'Қайталау', ru: 'Повторить' },

  // Errors (app and server)
  'Session expired': { kk: 'Сеанс мерзімі өтті', ru: 'Сеанс истёк' },
  'Please sign in again.': { kk: 'Қайта кіріңіз.', ru: 'Войдите снова.' },
  'Something went wrong. Please try again.': { kk: 'Бірдеңе дұрыс болмады. Қайталап көріңіз.', ru: 'Что-то пошло не так. Попробуйте ещё раз.' },
  'Could not reach the BiletFlow server. Check your internet connection (and that the backend is running) and try again.': { kk: 'BiletFlow серверіне қосылу мүмкін болмады. Интернетті (және сервердің қосулы екенін) тексеріп, қайталаңыз.', ru: 'Не удалось связаться с сервером BiletFlow. Проверьте интернет (и что сервер запущен) и повторите.' },
  'No internet connection. Check-in needs internet. Check Wi-Fi or mobile data and try again.': { kk: 'Интернет жоқ. Кіруді тексеруге интернет қажет. Wi-Fi немесе мобильді интернетті тексеріп, қайталаңыз.', ru: 'Нет интернета. Для контроля входа нужен интернет. Проверьте Wi-Fi или мобильные данные и повторите.' },
  'This ticket can no longer be checked in. Scan it again to see why.': { kk: 'Бұл билетпен енді кіргізу мүмкін емес. Себебін көру үшін қайта сканерлеңіз.', ru: 'По этому билету уже нельзя пропустить. Отсканируйте снова, чтобы узнать причину.' },
  'Ticket not found': { kk: 'Билет табылмады', ru: 'Билет не найден' },
  'This ticket is not checked in': { kk: 'Бұл билетпен әлі кірмеген', ru: 'По этому билету ещё не проходили' },
  'You are not allowed to undo check-ins for this event': { kk: 'Бұл іс-шарада кіруді болдырмауға рұқсатыңыз жоқ', ru: 'Вам нельзя отменять проходы на этом событии' },
  'Too many attempts. Please wait a moment and try again.': { kk: 'Әрекет тым көп. Сәл күтіп, қайталаңыз.', ru: 'Слишком много попыток. Подождите немного и повторите.' },
  'This event is not published': { kk: 'Бұл іс-шара жарияланбаған', ru: 'Это событие не опубликовано' },
};
