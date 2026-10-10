/** Регистрация и вход с явным согласием на обработку персональных данных. */
import { useState } from "react";
import type { FormEvent } from "react";
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom";
import { ArrowRight, Check, ShieldCheck } from "lucide-react";
import { api, clearCSRF, errorMessage } from "../api/client";
import { POLICY_VERSION } from "../api/types";
import type { User } from "../api/types";
import { useAuth } from "../app/auth";
import { ErrorNotice, Field, Loading } from "../components/ui";
import {
  ContactFields,
  formText,
  validateContacts,
} from "../features/contact-fields";

/** Собирает только обязательные поля и показывает безопасные ошибки сервера рядом с формой. */
export function AuthPage({ register = false }: { register?: boolean }) {
  const { user, loading, setUser } = useAuth();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  if (loading) {
    return <Loading />;
  }

  if (user) {
    return (
      <Navigate
        to={user.role === "admin" ? "/admin/appointments" : "/appointments"}
        replace
      />
    );
  }

  /** Проверяет согласие и повтор пароля до отправки запроса регистрации. */
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = new FormData(event.currentTarget);
    const password = formText(form, "password");
    const login = formText(form, "login");
    if (register) {
      const contactError = validateContacts(form);
      if (contactError) {
        setError(contactError);

        return;
      }

      if (
        Array.from(password).length < 15 ||
        Array.from(password).length > 128
      ) {
        setError("Пароль должен содержать от 15 до 128 символов.");

        return;
      }

      if (password !== formText(form, "confirm")) {
        setError("Пароли не совпадают.");

        return;
      }

      if (form.get("consent") !== "on") {
        setError("Необходимо согласие на обработку данных.");

        return;
      }
    }

    setBusy(true);
    try {
      if (register) {
        await api("/auth/register", {
          method: "POST",
          public: true,
          body: {
            login,
            password,
            name: formText(form, "name").trim(),
            phone: formText(form, "phone"),
            email: formText(form, "email").trim(),
            consent: true,
            policy_version: POLICY_VERSION,
          },
        });
        navigate("/login", {
          replace: true,
          state: { registered: true, from: location.state?.from },
        });
      } else {
        const account = await api<User>("/auth/login", {
          method: "POST",
          public: true,
          body: { login, password },
        });
        clearCSRF();
        setUser(account);
        const from: unknown = location.state?.from;
        navigate(
          account.role === "admin"
            ? "/admin/appointments"
            : typeof from === "string" && from.startsWith("/appointments")
              ? from
              : "/appointments",
          { replace: true },
        );
      }
    } catch (error) {
      setError(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="auth-layout">
      <aside className="auth-aside">
        <p className="eyebrow">ВАША ТЕХНИКА В ХОРОШИХ РУКАХ</p>
        <h1>
          Меньше хлопот.
          <br />
          <span>Больше заботы.</span>
        </h1>
        <p>Один кабинет для всего, что связано с ремонтом.</p>
        <ul>
          <li>
            <Check size={18} /> Запись в удобное время
          </li>
          <li>
            <Check size={18} /> Статус и история каждой заявки
          </li>
          <li>
            <Check size={18} /> Все обращения в одном месте
          </li>
        </ul>
        <div className="privacy-note">
          <ShieldCheck size={28} />
          <p>Ваши контактные данные хранятся в зашифрованном виде.</p>
        </div>
      </aside>
      <section className="form-card auth-card">
        <p className="eyebrow">ЛИЧНЫЙ КАБИНЕТ</p>
        <h2>{register ? "Давайте познакомимся" : "С возвращением"}</h2>
        <p className="muted">
          {register
            ? "Создайте аккаунт, чтобы записаться на ремонт."
            : "Войдите, чтобы посмотреть заявки или создать новую."}
        </p>
        {!register && location.state?.registered && (
          <div className="notice success" role="status">
            Аккаунт создан. Войдите с вашим логином и паролем.
          </div>
        )}
        <ErrorNotice message={error} />
        <form autoComplete="off" onSubmit={(event) => void submit(event)}>
          <Field
            name="login"
            label="Логин"
            pattern="[a-zA-Z][a-zA-Z0-9_]{3,31}"
            minLength={4}
            maxLength={32}
            hint="4–32 латинские буквы, цифры или _. Начните с буквы."
          />
          <Field
            name="password"
            label="Пароль"
            type="password"
            maxLength={256}
            hint={
              register
                ? "От 15 до 128 символов. Подойдёт длинная запоминающаяся фраза."
                : undefined
            }
          />
          {register && (
            <>
              <Field
                name="confirm"
                label="Повторите пароль"
                type="password"
                maxLength={256}
              />
              <ContactFields />
              <label className="checkbox-label">
                <input type="checkbox" name="consent" required />
                <span>
                  Согласие на обработку данных. Принимаю{" "}
                  <Link to="/privacy" target="_blank" rel="noreferrer">
                    политику обработки данных
                  </Link>
                  .
                </span>
              </label>
            </>
          )}
          <button type="submit" className="button primary full" disabled={busy}>
            {busy ? "Подождите…" : register ? "Создать аккаунт" : "Войти"}
            <ArrowRight size={18} />
          </button>
        </form>
        <p className="form-switch">
          {register ? "Уже есть аккаунт?" : "Ещё нет аккаунта?"}{" "}
          <Link to={register ? "/login" : "/register"} state={location.state}>
            {register ? "Войти" : "Зарегистрироваться"}
          </Link>
        </p>
      </section>
    </div>
  );
}
