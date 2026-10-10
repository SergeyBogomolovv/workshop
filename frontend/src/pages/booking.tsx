/** Форма записи с разделами, использующая актуальный каталог и доступное время. */
import { useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { ArrowLeft, ArrowRight, CalendarDays, ShieldCheck } from "lucide-react";
import { api, errorMessage } from "../api/client";
import { useResource } from "../api/use-resource";
import { calendarDate, money } from "../api/types";
import type { Catalog } from "../api/types";
import { ErrorNotice, Loading, PageHeading } from "../components/ui";
import { formText } from "../features/contact-fields";

/** Загружает каталог до отображения формы записи с первоначальным выбором. */
export function BookingPage() {
  const { data, error, loading } = useResource<Catalog>("/catalog");

  return (
    <>
      <Link className="back-link" to="/appointments">
        <ArrowLeft size={16} /> Мои заявки
      </Link>
      <PageHeading
        eyebrow="НОВАЯ ЗАЯВКА"
        title="Что будем чинить?"
        description="Выберите услугу и удобный час приёма. Остальное возьмём на себя."
      />
      <ErrorNotice message={error} />
      {loading && <Loading />}
      {data && <BookingForm catalog={data} />}
    </>
  );
}

/** Согласует выбранные категорию, услугу и доступное время приёма. */
function BookingForm({ catalog }: { catalog: Catalog }) {
  const [params] = useSearchParams();
  const selectedService = catalog.services.find(
    (item) => item.id === params.get("service"),
  );
  const initialCategory =
    params.get("category") ?? selectedService?.categories[0] ?? "";
  const [category, setCategory] = useState(
    catalog.categories.some((item) => item.code === initialCategory)
      ? initialCategory
      : "",
  );
  const [serviceID, setServiceID] = useState(
    selectedService?.categories.includes(initialCategory)
      ? selectedService.id
      : "",
  );
  const [date, setDate] = useState(calendarDate());
  const [slot, setSlot] = useState("");
  const [revision, setRevision] = useState(0);
  const {
    data: slots,
    error: slotsError,
    loading,
  } = useResource<string[]>(`/slots?date=${date}`, revision);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const navigate = useNavigate();
  const offering = catalog.services.find((item) => item.id === serviceID);
  /** Проверяет описание и использует точное время интервала, полученное от сервера. */
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const description = formText(data, "description").trim();
    setError("");
    if (
      Array.from(description).length < 10 ||
      Array.from(description).length > 1000 ||
      /[<>]/.test(description)
    ) {
      setError("Описание: 10–1000 символов без HTML-разметки.");

      return;
    }

    if (!slots?.includes(slot)) {
      setError("Выберите свободное время.");

      return;
    }

    setBusy(true);
    try {
      const result = await api<{ id: string }>("/appointments", {
        method: "POST",
        body: {
          category,
          service_id: serviceID,
          slot_start: slot,
          description,
        },
      });
      navigate(`/appointments/${result.id}`, {
        replace: true,
        state: { created: true },
      });
    } catch (error) {
      setError(errorMessage(error));
      setSlot("");
      setRevision((value) => value + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="two-column">
      <form
        className="form-card booking-form"
        autoComplete="off"
        onSubmit={(event) => void submit(event)}
      >
        <ErrorNotice message={error} />
        <fieldset>
          <legend>
            <span>01</span> Устройство и услуга
          </legend>
          <div className="form-grid">
            <div className="field">
              <label htmlFor="category">Ваше устройство</label>
              <select
                required
                id="category"
                name="category"
                value={category}
                onChange={(event) => {
                  setCategory(event.target.value);
                  setServiceID("");
                }}
              >
                <option value="" disabled>
                  Выберите устройство
                </option>
                {catalog.categories.map((item) => (
                  <option key={item.code} value={item.code}>
                    {item.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="field">
              <label htmlFor="service">Что нужно сделать</label>
              <select
                required
                id="service"
                name="service"
                value={serviceID}
                disabled={!category}
                onChange={(event) => setServiceID(event.target.value)}
              >
                <option value="" disabled>
                  Выберите услугу
                </option>
                {catalog.services
                  .filter((item) => item.categories.includes(category))
                  .map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name}
                    </option>
                  ))}
              </select>
            </div>
          </div>
        </fieldset>
        <fieldset>
          <legend>
            <span>02</span> Удобное время
          </legend>
          <div className="form-grid">
            <div className="field">
              <label htmlFor="date">Дата приёма</label>
              <input
                required
                type="date"
                id="date"
                name="date"
                min={calendarDate()}
                max={calendarDate(29)}
                value={date}
                onChange={(event) => {
                  setDate(event.target.value || calendarDate());
                  setSlot("");
                }}
              />
              <small>Ближайшие 30 дней, включая сегодня</small>
            </div>
            <div className="field">
              <label htmlFor="slot">Время приёма</label>
              <select
                required
                id="slot"
                name="slot"
                value={slot}
                disabled={loading || !slots?.length}
                onChange={(event) => setSlot(event.target.value)}
              >
                <option value="" disabled>
                  {loading
                    ? "Загружаем…"
                    : slots?.length
                      ? "Выберите время"
                      : "Нет свободного времени"}
                </option>
                {slots?.map((value) => (
                  <option key={value} value={value}>
                    {new Intl.DateTimeFormat("ru-RU", {
                      hour: "2-digit",
                      minute: "2-digit",
                      timeZone: "Europe/Moscow",
                    }).format(new Date(value))}
                  </option>
                ))}
              </select>
              <small>Московское время · приём устройства</small>
            </div>
          </div>
          <ErrorNotice message={slotsError} />
          {!loading && slots?.length === 0 && (
            <p className="notice">
              На эту дату свободных слотов нет. Попробуйте следующий день.
            </p>
          )}
        </fieldset>
        <fieldset>
          <legend>
            <span>03</span> Немного о проблеме
          </legend>
          <div className="field">
            <label htmlFor="description">Что случилось с устройством?</label>
            <textarea
              required
              id="description"
              name="description"
              rows={4}
              minLength={10}
              maxLength={1000}
              placeholder="Например: ноутбук сильно нагревается и шумит при работе…"
              aria-describedby="description-hint"
            />
            <small id="description-hint">
              От 10 до 1000 символов. Не указывайте пароли и другие секретные
              данные.
            </small>
          </div>
        </fieldset>
        <button
          type="submit"
          className="button primary"
          disabled={busy || loading || !slot}
        >
          {busy ? "Создаём заявку…" : "Подтвердить запись"}
          <ArrowRight size={18} />
        </button>
      </form>
      <aside className="booking-aside">
        <div className="info-card summary-card">
          <CalendarDays size={28} />
          <h2>Ваша запись</h2>
          <dl>
            <div>
              <dt>Услуга</dt>
              <dd>{offering?.name ?? "Пока не выбрана"}</dd>
            </div>
            <div>
              <dt>Ориентировочно</dt>
              <dd className="summary-price">
                {offering ? `от ${money(offering.price_kopecks)}` : "—"}
              </dd>
            </div>
          </dl>
          <p>
            Мастер уточнит стоимость после диагностики и согласует её с вами.
          </p>
        </div>
        <div className="aside-tip">
          <ShieldCheck size={22} />
          <p>Передумали? Заявку можно отменить, пока ремонт ещё не начался.</p>
        </div>
      </aside>
    </div>
  );
}
