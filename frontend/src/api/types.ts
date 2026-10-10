/** Общие явно заданные контракты ответов API мастерской. */
export interface User {
  id: string;
  login: string;
  name: string;
  phone: string;
  email: string;
  role: "client" | "admin";
}
export interface Category {
  code: string;
  name: string;
}
export interface Offering {
  id: string;
  name: string;
  description: string;
  price_kopecks: number;
  categories: string[];
}
export interface Catalog {
  categories: Category[];
  services: Offering[];
}
export type Status =
  "new" | "accepted" | "in_progress" | "ready" | "completed" | "cancelled";
export interface Appointment {
  id: string;
  service_id: string;
  category: string;
  slot_start: string;
  description: string;
  status: Status;
  created_at: string;
  updated_at: string;
  service_name: string;
  category_name: string;
}
export interface Detail {
  appointment: Appointment;
  history: { previous_status: string; status: Status; created_at: string }[];
  client?: User;
}
export interface Page {
  items: Appointment[];
  page: number;
  has_more: boolean;
}
export const POLICY_VERSION = "2026-10-10";
export const statusLabels: Record<Status, string> = {
  new: "Новая",
  accepted: "Принята",
  in_progress: "В работе",
  ready: "Готова",
  completed: "Выдана",
  cancelled: "Отменена",
};

/** Форматирует цены, хранящиеся в целых копейках, без погрешностей отображения дробных чисел. */
export function money(kopecks: number): string {
  return new Intl.NumberFormat("ru-RU", {
    style: "currency",
    currency: "RUB",
    maximumFractionDigits: 0,
  }).format(kopecks / 100);
}

/** Отображает все даты заявок в московском часовом поясе мастерской. */
export function dateTime(value: string): string {
  return new Intl.DateTimeFormat("ru-RU", {
    day: "numeric",
    month: "long",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "Europe/Moscow",
  }).format(new Date(value));
}

/** Возвращает календарную дату в формате ISO со смещением относительно сегодняшней даты по Москве. */
export function calendarDate(offset = 0): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    timeZone: "Europe/Moscow",
  }).formatToParts(new Date());
  const value = (type: string) =>
    parts.find((part) => part.type === type)?.value ?? "";
  const date = new Date(
    `${value("year")}-${value("month")}-${value("day")}T12:00:00Z`,
  );
  date.setUTCDate(date.getUTCDate() + offset);

  return date.toISOString().slice(0, 10);
}
