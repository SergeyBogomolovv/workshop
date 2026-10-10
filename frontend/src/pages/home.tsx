/** Общедоступный каталог услуг и описание порядка записи на ремонт. */
import { useState } from "react";
import {
  ArrowDown,
  ArrowRight,
  BatteryCharging,
  CalendarDays,
  Check,
  CheckCheck,
  ChevronRight,
  CircuitBoard,
  Clock3,
  Laptop,
  Monitor,
  PlugZap,
  Settings2,
  ShieldCheck,
  Smartphone,
  Sparkles,
  Tablet,
} from "lucide-react";
import { Link } from "react-router-dom";
import { useResource } from "../api/use-resource";
import { money } from "../api/types";
import type { Catalog } from "../api/types";
import { useAuth } from "../app/auth";
import { ErrorNotice, Loading } from "../components/ui";

const serviceIcons = [
  CircuitBoard,
  Monitor,
  BatteryCharging,
  Sparkles,
  Settings2,
  PlugZap,
];

/** Отображает главную страницу с фильтром категорий и актуальными ценами услуг. */
export function HomePage() {
  const { data, error, loading } = useResource<Catalog>("/catalog");
  const [category, setCategory] = useState("all");
  const { user } = useAuth();
  const bookingURL =
    user?.role === "admin" ? "/admin/appointments" : "/appointments/new";

  return (
    <>
      <section className="hero">
        <div className="hero-copy">
          <div className="hero-label">
            <span /> ЗАБОТА О ВАШЕЙ ТЕХНИКЕ
          </div>
          <h1>
            Сломалось?
            <br />
            Давайте <span>починим.</span>
          </h1>
          <p>
            Смартфон, ноутбук или планшет —<br className="desktop-break" />{" "}
            вернём в строй то, что вам дорого.
          </p>
          <div className="hero-actions">
            <Link className="button primary" to={bookingURL}>
              {user?.role === "admin" ? "К заявкам" : "Записаться на ремонт"}
              <ArrowRight size={19} />
            </Link>
            <a className="text-link" href="#services">
              Выбрать услугу <ArrowDown size={17} />
            </a>
          </div>
          <div className="hero-promises">
            <span>
              <Check size={15} /> Понятные цены
            </span>
            <span>
              <Check size={15} /> Запись без звонков
            </span>
          </div>
        </div>
        <div className="hero-art" aria-hidden="true">
          <div className="orbit orbit-one" />
          <div className="orbit orbit-two" />
          <span className="art-spark art-spark-one">✳</span>
          <span className="art-spark art-spark-two">+</span>
          <div className="device-laptop">
            <div className="laptop-screen">
              <div className="screen-top">
                <i />
                <i />
                <i />
              </div>
              <div className="screen-center">
                <CheckCheck size={48} />
                <span>Всё работает</span>
                <div className="screen-bars">
                  <i />
                  <i />
                  <i />
                </div>
              </div>
            </div>
            <div className="laptop-base" />
          </div>
          <div className="device-phone">
            <div className="phone-notch" />
            <BatteryCharging size={34} />
            <span>100%</span>
            <small>Снова в деле</small>
          </div>
          <div className="art-tag tag-top">
            <ShieldCheck size={20} />
            <span>В надёжных руках</span>
          </div>
          <div className="art-tag tag-bottom">
            <span className="tag-icon">
              <Check size={18} />
            </span>
            <div>
              <strong>Хорошие новости</strong>
              <small>Ваша техника готова</small>
            </div>
          </div>
          <span className="art-caption">
            РЕМОНТИРУЕМ. ВОССТАНАВЛИВАЕМ. ЗАБОТИМСЯ.
          </span>
        </div>
      </section>
      <section className="benefit-strip" aria-label="Преимущества сервиса">
        <div>
          <CalendarDays />
          <span>
            <strong>В удобное время</strong>
            <small>Выберите свободный час приёма</small>
          </span>
        </div>
        <div>
          <ShieldCheck />
          <span>
            <strong>Всё под контролем</strong>
            <small>Статус ремонта в вашем кабинете</small>
          </span>
        </div>
        <div>
          <Clock3 />
          <span>
            <strong>Без лишних ожиданий</strong>
            <small>Оформите заявку за пару минут</small>
          </span>
        </div>
      </section>
      <section id="services" className="catalog-section">
        <div className="section-heading">
          <div>
            <p className="eyebrow">ЧЕМ МОЖЕМ ПОМОЧЬ</p>
            <h2>Вернём технику в строй</h2>
          </div>
          <p>
            Начнём с диагностики.
            <br />
            Стоимость согласуем до ремонта.
          </p>
        </div>
        <div
          className="filter-tabs"
          role="group"
          aria-label="Категория устройства"
        >
          {[
            { code: "all", name: "Вся техника", icon: CircuitBoard },
            { code: "smartphone", name: "Смартфоны", icon: Smartphone },
            { code: "laptop", name: "Ноутбуки", icon: Laptop },
            { code: "tablet", name: "Планшеты", icon: Tablet },
          ].map(({ code, name, icon: Icon }) => (
            <button
              type="button"
              key={code}
              className={category === code ? "active" : ""}
              aria-pressed={category === code}
              onClick={() => setCategory(code)}
            >
              <Icon size={17} />
              {name}
            </button>
          ))}
        </div>
        <ErrorNotice message={error} />
        {loading && <Loading />}
        <div className="service-grid">
          {data?.services
            .filter(
              (service) =>
                category === "all" || service.categories.includes(category),
            )
            .map((service) => {
              const Icon =
                serviceIcons[Number(service.id.slice(-1)) - 1] ?? CircuitBoard;

              return (
                <article className="service-card" key={service.id}>
                  <div className="service-icon">
                    <Icon size={25} strokeWidth={1.5} />
                  </div>
                  <h3>{service.name}</h3>
                  <p>{service.description}</p>
                  <div className="service-bottom">
                    <span>
                      от <strong>{money(service.price_kopecks)}</strong>
                    </span>
                    <Link
                      to={
                        user?.role === "admin"
                          ? "/admin/appointments"
                          : `/appointments/new?service=${service.id}${category !== "all" ? `&category=${category}` : ""}`
                      }
                      aria-label={`Записаться: ${service.name}`}
                    >
                      <ArrowRight size={20} />
                    </Link>
                  </div>
                </article>
              );
            })}
        </div>
        <p className="price-note">
          Цены ориентировочные. Итоговая стоимость зависит от устройства и
          результатов диагностики.
        </p>
      </section>
      <section className="how-section">
        <div>
          <p className="eyebrow">ПРОСТО И ПРОЗРАЧНО</p>
          <h2>
            Три шага.
            <br />И никаких сложностей.
          </h2>
          <Link className="text-link" to={bookingURL}>
            Начнём? <ChevronRight size={18} />
          </Link>
        </div>
        <ol className="steps">
          <li>
            <span>01</span>
            <div>
              <h3>Выберите услугу и время</h3>
              <p>
                Расскажите, что случилось. Мы будем ждать вас к назначенному
                часу.
              </p>
            </div>
          </li>
          <li>
            <span>02</span>
            <div>
              <h3>Принесите устройство</h3>
              <p>Проверим технику и согласуем с вами детали ремонта.</p>
            </div>
          </li>
          <li>
            <span>03</span>
            <div>
              <h3>Следите за результатом</h3>
              <p>
                Статус заявки и история изменений — всегда в личном кабинете.
              </p>
            </div>
          </li>
        </ol>
      </section>
    </>
  );
}
