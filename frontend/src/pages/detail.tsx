/** Подробности заявки с проверкой владельца и действия администратора по обработке ремонта. */
import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import {
  ArrowLeft,
  CalendarDays,
  Check,
  Clock3,
  Smartphone,
} from "lucide-react";
import { api, errorMessage } from "../api/client";
import { useResource } from "../api/use-resource";
import { dateTime, statusLabels } from "../api/types";
import type { Detail, Status } from "../api/types";
import { Badge, ErrorNotice, Loading, PageHeading } from "../components/ui";

const nextStatuses: Partial<Record<Status, Status>> = {
  new: "accepted",
  accepted: "in_progress",
  in_progress: "ready",
  ready: "completed",
};

/** Обновляет историю после каждого успешно проверенного сервером перехода статуса. */
export function DetailPage({ admin = false }: { admin?: boolean }) {
  const { id } = useParams();
  const base = admin ? "/admin/appointments" : "/appointments";
  const [revision, setRevision] = useState(0);
  const { data, error, loading } = useResource<Detail>(
    `${base}/${id}`,
    revision,
  );
  const location = useLocation();
  const [actionError, setActionError] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirmCancel, setConfirmCancel] = useState(false);
  /** Выполняет разрешённое действие и отображает актуальное состояние с сервера. */
  async function transition(status: Status) {
    setBusy(true);
    setActionError("");
    try {
      await api(admin ? `${base}/${id}/status` : `${base}/${id}/cancel`, {
        method: admin ? "PATCH" : "POST",
        ...(admin ? { body: { status } } : {}),
      });
      setConfirmCancel(false);
      setRevision((value) => value + 1);
    } catch (error) {
      setActionError(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  /** Преобразует выбранный администратором новый статус в запрос изменения. */
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const status = new FormData(event.currentTarget).get("status") as Status;
    void transition(status);
  }

  const appointment = data?.appointment;
  const canCancel =
    appointment?.status === "new" || appointment?.status === "accepted";
  const next = appointment ? nextStatuses[appointment.status] : undefined;

  return (
    <>
      <Link className="back-link" to={base}>
        <ArrowLeft size={16} /> {admin ? "Все заявки" : "Мои заявки"}
      </Link>
      <PageHeading
        eyebrow={`ЗАЯВКА № ${(id ?? "").slice(0, 8).toUpperCase()}`}
        title={appointment?.service_name ?? "Ваша заявка"}
        description="Детали обращения и история ремонта."
      >
        {appointment && <Badge status={appointment.status} />}
      </PageHeading>
      {location.state?.created && (
        <div className="notice success" role="status">
          <Check size={20} /> Вы записаны! Приносите устройство в выбранное
          время.
        </div>
      )}
      <ErrorNotice message={error || actionError} />
      {loading && <Loading />}
      {data && appointment && (
        <div className="two-column">
          <div className="detail-stack">
            <section className="form-card">
              <h2>Детали записи</h2>
              <div className="detail-facts">
                <div>
                  <Smartphone size={22} />
                  <span>
                    <small>Устройство</small>
                    <strong>{appointment.category_name}</strong>
                  </span>
                </div>
                <div>
                  <CalendarDays size={22} />
                  <span>
                    <small>Дата и время приёма</small>
                    <strong>{dateTime(appointment.slot_start)} МСК</strong>
                  </span>
                </div>
              </div>
              <div className="description-block">
                <h3>Что случилось</h3>
                <p>{appointment.description}</p>
              </div>
            </section>
            {data.client && (
              <section className="form-card">
                <h2>Контакты клиента</h2>
                <dl className="contact-list">
                  <div>
                    <dt>Имя</dt>
                    <dd>{data.client.name}</dd>
                  </div>
                  <div>
                    <dt>Телефон</dt>
                    <dd>
                      <a href={`tel:${data.client.phone}`}>
                        {data.client.phone}
                      </a>
                    </dd>
                  </div>
                  <div>
                    <dt>Email</dt>
                    <dd>
                      <a href={`mailto:${data.client.email}`}>
                        {data.client.email}
                      </a>
                    </dd>
                  </div>
                </dl>
              </section>
            )}
            <section className="form-card">
              <h2>
                <Clock3 size={21} /> История заявки
              </h2>
              <ol className="timeline">
                {data.history.map((item, index) => (
                  <li key={`${item.created_at}-${index}`}>
                    <span className="timeline-dot" />
                    <div>
                      <strong>{statusLabels[item.status]}</strong>
                      <time dateTime={item.created_at}>
                        {dateTime(item.created_at)} МСК
                      </time>
                    </div>
                  </li>
                ))}
              </ol>
            </section>
          </div>
          <aside>
            <div className="info-card">
              <h2>{admin ? "Управление заявкой" : "Что дальше?"}</h2>
              {admin && (next || canCancel) ? (
                <form autoComplete="off" onSubmit={submit}>
                  <div className="field">
                    <label htmlFor="next-status">Новый статус</label>
                    <select
                      required
                      name="status"
                      id="next-status"
                      key={appointment.status}
                      defaultValue=""
                    >
                      <option value="" disabled>
                        Выберите действие
                      </option>
                      {next && (
                        <option value={next}>{statusLabels[next]}</option>
                      )}
                      {canCancel && <option value="cancelled">Отменена</option>}
                    </select>
                  </div>
                  <button
                    type="submit"
                    className="button primary full"
                    disabled={busy}
                  >
                    {busy ? "Сохраняем…" : "Изменить статус"}
                  </button>
                </form>
              ) : (
                <p>
                  {appointment.status === "cancelled"
                    ? "Запись отменена. Вы можете выбрать новое время в любой момент."
                    : appointment.status === "completed"
                      ? "Ремонт завершён, устройство выдано. Спасибо, что обратились к нам."
                      : appointment.status === "ready"
                        ? "Устройство готово. Можно забрать его в часы работы мастерской."
                        : "Принесите устройство в назначенное время. Все изменения статуса появятся в истории заявки."}
                </p>
              )}
              {!admin && canCancel && (
                <div className="cancel-block">
                  {confirmCancel ? (
                    <>
                      <p>
                        Отменить запись? Выбранное время станет доступно другим
                        клиентам.
                      </p>
                      <div className="button-row">
                        <button
                          type="button"
                          className="button danger small"
                          disabled={busy}
                          onClick={() => void transition("cancelled")}
                        >
                          Да, отменить
                        </button>
                        <button
                          type="button"
                          className="button secondary small"
                          disabled={busy}
                          onClick={() => setConfirmCancel(false)}
                        >
                          Оставить
                        </button>
                      </div>
                    </>
                  ) : (
                    <button
                      type="button"
                      className="text-button danger-text"
                      onClick={() => setConfirmCancel(true)}
                    >
                      Отменить запись
                    </button>
                  )}
                </div>
              )}
            </div>
            <p className="aside-caption">
              Мастерская работает ежедневно
              <br />с 10:00 до 18:00 по Москве.
            </p>
          </aside>
        </div>
      )}
    </>
  );
}
