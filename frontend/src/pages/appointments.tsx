/** Списки заявок клиента и администратора с фильтрами и ограниченной пагинацией. */
import { useState } from "react";
import { Link } from "react-router-dom";
import {
  ArrowRight,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  Plus,
} from "lucide-react";
import { useResource } from "../api/use-resource";
import { dateTime, statusLabels } from "../api/types";
import type { Page } from "../api/types";
import {
  Badge,
  Empty,
  ErrorNotice,
  Loading,
  PageHeading,
} from "../components/ui";

/** Подбирает заголовок и пояснение для пустого списка заявок. */
function emptyState(status: string, admin: boolean) {
  if (status) {
    return {
      title: "Заявок с таким статусом нет",
      description: "Выберите другой статус или посмотрите все заявки.",
    };
  }

  if (admin) {
    return {
      title: "Пока нет заявок",
      description: "Новые обращения появятся здесь после записи клиента.",
    };
  }

  return {
    title: "Здесь начинается забота о технике",
    description: "Создайте первую заявку — выберите услугу и удобное время.",
  };
}

/** Показывает актуальные заявки с постоянными ссылками и явными состояниями пустого списка. */
export function AppointmentsPage({ admin = false }: { admin?: boolean }) {
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [revision, setRevision] = useState(0);

  const base = admin ? "/admin/appointments" : "/appointments";
  const empty = emptyState(status, admin);
  const { data, error, loading } = useResource<Page>(
    `${base}?page=${page}${status ? `&status=${status}` : ""}`,
    revision,
  );

  return (
    <>
      <PageHeading
        eyebrow={admin ? "КАБИНЕТ МАСТЕРА" : "ЛИЧНЫЙ КАБИНЕТ"}
        title={admin ? "Заявки на ремонт" : "Мои заявки"}
        description={
          admin
            ? "Все обращения клиентов. От приёма устройства до выдачи."
            : "Всё о вашей технике — от первой записи до готового ремонта."
        }
      >
        {!admin && (
          <Link className="button primary" to="/appointments/new">
            <Plus size={18} />
            Новая заявка
          </Link>
        )}
      </PageHeading>
      <div className="list-toolbar">
        <div className="field inline-field">
          <label htmlFor="status-filter">Статус</label>
          <select
            id="status-filter"
            value={status}
            onChange={(event) => {
              setStatus(event.target.value);
              setPage(1);
            }}
          >
            <option value="">Все заявки</option>
            {Object.entries(statusLabels).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </div>
        <button
          type="button"
          className="text-button"
          disabled={loading}
          onClick={() => setRevision((value) => value + 1)}
        >
          Обновить
        </button>
      </div>
      <ErrorNotice message={error} />
      {loading && <Loading />}
      {data && data.items.length === 0 && (
        <Empty title={empty.title}>
          <p>{empty.description}</p>
          {!admin && !status && (
            <Link className="button primary" to="/appointments/new">
              Записаться на ремонт <ArrowRight size={18} />
            </Link>
          )}
        </Empty>
      )}
      <div className="appointment-list">
        {data?.items.map((item) => (
          <Link
            key={item.id}
            to={`${base}/${item.id}`}
            className="appointment-card"
          >
            <div className="appointment-icon">
              <CalendarDays size={24} />
            </div>
            <div className="appointment-main">
              <span className="request-number">
                ЗАЯВКА № {item.id.slice(0, 8).toUpperCase()}
              </span>
              <h2>{item.service_name}</h2>
              <p>
                {item.category_name} <span>·</span> {dateTime(item.slot_start)}{" "}
                МСК
              </p>
            </div>
            <Badge status={item.status} />
            <ChevronRight className="row-arrow" size={21} />
          </Link>
        ))}
      </div>
      {data && (page > 1 || data.has_more) && (
        <nav className="pagination" aria-label="Страницы заявок">
          <button
            type="button"
            className="button secondary small"
            disabled={page === 1}
            onClick={() => setPage((value) => value - 1)}
          >
            <ChevronLeft size={16} /> Назад
          </button>
          <span>Страница {page}</span>
          <button
            type="button"
            className="button secondary small"
            disabled={!data.has_more}
            onClick={() => setPage((value) => value + 1)}
          >
            Далее <ChevronRight size={16} />
          </button>
        </nav>
      )}
    </>
  );
}
