/** Общие обязательные контактные поля для регистрации и редактирования профиля. */
import { Field } from "../components/ui";
import type { User } from "../api/types";

/** Задаёт подходящие типы полей и видимые подсказки для строгих форматов. */
export function ContactFields({ user }: { user?: User }) {
  return (
    <>
      <Field
        name="name"
        label="Ваше имя"
        minLength={2}
        maxLength={80}
        defaultValue={user?.name}
        hint="От 2 до 80 символов: буквы, пробел, дефис или апостроф"
      />
      <Field
        name="phone"
        label="Телефон"
        type="tel"
        pattern="\+7[0-9]{10}"
        defaultValue={user?.phone}
        placeholder="+79991234567"
        hint="+7 и 10 цифр, без пробелов и скобок"
      />
      <Field
        name="email"
        label="Электронная почта"
        type="email"
        maxLength={254}
        defaultValue={user?.email}
        placeholder="you@example.ru"
      />
    </>
  );
}

/** Читает обязательное текстовое значение формы по имени поля. */
export function formText(data: FormData, key: string): string {
  return String(data.get(key) ?? "");
}

/** Проверяет форматы контактов с учётом Unicode перед отправкой на сервер. */
export function validateContacts(data: FormData): string {
  const name = formText(data, "name").trim();
  if (
    Array.from(name).length < 2 ||
    Array.from(name).length > 80 ||
    !/^[\p{L} '-]+$/u.test(name) ||
    !/\p{L}/u.test(name)
  ) {
    return "Проверьте имя: допустимы буквы, пробел, дефис и апостроф.";
  }

  if (!/^\+7[0-9]{10}$/.test(formText(data, "phone"))) {
    return "Введите телефон в формате +79991234567.";
  }

  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formText(data, "email").trim())) {
    return "Введите корректный email.";
  }

  return "";
}
