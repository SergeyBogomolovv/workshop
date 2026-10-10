/** Редактирование контактов учётной записи с отображением ошибок серверной проверки. */
import { useState } from "react";
import type { FormEvent } from "react";
import { ShieldCheck } from "lucide-react";
import { useAuth } from "../app/auth";
import { api, errorMessage } from "../api/client";
import type { User } from "../api/types";
import { ErrorNotice, PageHeading } from "../components/ui";
import {
  ContactFields,
  formText,
  validateContacts,
} from "../features/contact-fields";

/** Сохраняет все обязательные контактные поля без изменения логина. */
export function ProfilePage() {
  const { user, setUser } = useAuth();
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);
  /** Нормализует контакты и обновляет текущий профиль после успешного сохранения. */
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaved(false);
    const data = new FormData(event.currentTarget);
    const validation = validateContacts(data);
    setError(validation);
    if (validation) {
      return;
    }

    setBusy(true);
    try {
      const updated = await api<User>("/profile", {
        method: "PUT",
        body: {
          name: formText(data, "name").trim(),
          phone: formText(data, "phone"),
          email: formText(data, "email").trim(),
        },
      });
      setUser(updated);
      setSaved(true);
    } catch (error) {
      setError(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  if (!user) {
    return null;
  }

  return (
    <>
      <PageHeading
        eyebrow="ЛИЧНЫЙ КАБИНЕТ"
        title="Ваш профиль"
        description="Контакты, по которым мастер сможет связаться с вами."
      />
      <div className="two-column">
        <section className="form-card">
          <div className="profile-summary">
            <span className="avatar">{user.name.slice(0, 1)}</span>
            <div>
              <h2>{user.name}</h2>
              <p className="muted">
                @{user.login} ·{" "}
                {user.role === "admin" ? "Администратор" : "Клиент"}
              </p>
            </div>
          </div>
          <ErrorNotice message={error} />
          {saved && (
            <div className="notice success" role="status">
              Изменения сохранены.
            </div>
          )}
          <form autoComplete="off" onSubmit={(event) => void submit(event)}>
            <ContactFields user={user} />
            <button type="submit" className="button primary" disabled={busy}>
              {busy ? "Сохраняем…" : "Сохранить изменения"}
            </button>
          </form>
        </section>
        <aside className="info-card">
          <ShieldCheck size={30} />
          <h2>Ваши данные защищены</h2>
          <p>
            Контакты доступны вам и администратору, который обрабатывает вашу
            заявку.
          </p>
          <p>
            Изменения применятся ко всем вашим заявкам. Логин остаётся прежним.
          </p>
        </aside>
      </div>
    </>
  );
}
