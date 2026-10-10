/** Доступные переиспользуемые поля форм, индикаторы статуса и состояния загрузки. */
import type { ReactNode, InputHTMLAttributes } from "react";
import { AlertCircle, ArrowRight, LoaderCircle } from "lucide-react";
import { Link } from "react-router-dom";
import { statusLabels } from "../api/types";
import type { Status } from "../api/types";

/** Связывает каждое поле с видимой подписью и подсказкой о формате. */
export function Field({
  label,
  hint,
  name,
  ...props
}: InputHTMLAttributes<HTMLInputElement> & {
  label: string;
  hint?: string;
  name: string;
}) {
  return (
    <div className="field">
      <label htmlFor={name}>{label}</label>
      <input
        type="text"
        id={name}
        name={name}
        required
        autoComplete="off"
        aria-describedby={hint ? `${name}-hint` : undefined}
        {...props}
      />
      {hint && <small id={`${name}-hint`}>{hint}</small>}
    </div>
  );
}

/** Отображает доступное сообщение об ошибке без внутренних данных сервера и стека вызовов. */
export function ErrorNotice({ message }: { message?: string }) {
  return message ? (
    <div className="notice error" role="alert">
      <AlertCircle size={19} />
      <span>{message}</span>
    </div>
  ) : null;
}

/** Показывает состояние асинхронной загрузки пользователям и вспомогательным технологиям. */
export function Loading() {
  return (
    <div className="loading" role="status">
      <LoaderCircle className="spin" size={24} /> Загружаем…
    </div>
  );
}

/** Показывает статус текстом и цветом, не полагаясь только на цветовое обозначение. */
export function Badge({ status }: { status: Status }) {
  return (
    <span className={`badge status-${status}`}>
      <span className="badge-dot" />
      {statusLabels[status]}
    </span>
  );
}

/** Создаёт единый заголовок страницы с необязательными действиями. */
export function PageHeading({
  eyebrow,
  title,
  description,
  children,
}: {
  eyebrow: string;
  title: string;
  description?: string;
  children?: ReactNode;
}) {
  return (
    <div className="page-heading">
      <div>
        <p className="eyebrow">{eyebrow}</p>
        <h1>{title}</h1>
        {description && <p className="muted">{description}</p>}
      </div>
      {children}
    </div>
  );
}

/** Оформляет переход к следующему действию как основную кнопку. */
export function ActionLink({
  to,
  children,
}: {
  to: string;
  children: ReactNode;
}) {
  return (
    <Link className="button primary" to={to}>
      {children}
      <ArrowRight size={18} />
    </Link>
  );
}

/** Показывает понятное следующее действие для пустого списка. */
export function Empty({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <div className="empty">
      <div className="empty-mark">М</div>
      <h2>{title}</h2>
      {children}
    </div>
  );
}
