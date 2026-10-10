/** Общая навигация, подвал и уведомление о cookie при первом посещении. */
import { useEffect, useRef, useState } from "react";
import {
  Link,
  NavLink,
  Outlet,
  useLocation,
  useNavigate,
} from "react-router-dom";
import { ArrowUpRight, Cookie, LogOut, UserRound } from "lucide-react";
import { useAuth } from "./auth";
import { errorMessage } from "../api/client";
import { ErrorNotice } from "../components/ui";

/** Показывает ненавязчивое уведомление о необходимых cookie, доступное с клавиатуры. */
function CookieNotice() {
  const [visible, setVisible] = useState(() => {
    try {
      return localStorage.getItem("workshop-cookie-notice") !== "1";
    } catch {
      return true;
    }
  });
  /** Сохраняет отметку о прочтении уведомления без записи данных учётной записи. */
  function accept() {
    try {
      localStorage.setItem("workshop-cookie-notice", "1");
    } catch {
      /* Если хранилище недоступно, выбор сохраняется в памяти на время текущего посещения. */
    }

    setVisible(false);
  }

  if (!visible) {
    return null;
  }

  return (
    <section
      className="cookie-notice"
      role="dialog"
      aria-modal="false"
      aria-labelledby="cookie-title"
    >
      <Cookie size={24} />
      <div>
        <h2 id="cookie-title">Только необходимые cookie</h2>
        <p>
          Используем cookie для безопасного входа. Без рекламы и аналитики.{" "}
          <Link to="/cookies">Подробнее</Link>
        </p>
      </div>
      <button type="button" className="button primary" onClick={accept}>
        Понятно
      </button>
    </section>
  );
}

/** Отображает адаптивную оболочку и переводит фокус на основное содержимое после перехода. */
export function Layout() {
  const { user, logout } = useAuth();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const main = useRef<HTMLElement>(null);
  useEffect(() => {
    window.scrollTo(0, 0);
    main.current?.focus({ preventScroll: true });
  }, [pathname]);
  /** Завершает выход на сервере перед переходом на общедоступную главную страницу. */
  async function handleLogout() {
    setBusy(true);
    setError("");
    try {
      await logout();
      navigate("/");
    } catch (error) {
      setError(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <a className="skip-link" href="#main">
        Перейти к содержимому
      </a>
      <header className="site-header">
        <div className="container header-inner">
          <Link className="brand" to="/" aria-label="Мастерская — главная">
            <span className="brand-mark">м</span>
            <span>
              мастерская<span className="brand-dot">.</span>
            </span>
          </Link>
          <nav aria-label="Основная навигация">
            <NavLink to="/" end>
              Услуги
            </NavLink>
            {user && (
              <NavLink
                to={
                  user.role === "admin"
                    ? "/admin/appointments"
                    : "/appointments"
                }
              >
                {user.role === "admin" ? "Панель мастера" : "Мои заявки"}
              </NavLink>
            )}
          </nav>
          <div className="account-nav">
            {user ? (
              <>
                <NavLink className="profile-link" to="/profile">
                  <UserRound size={18} />
                  <span>{user.name.split(" ")[0]}</span>
                </NavLink>
                <button
                  type="button"
                  className="icon-button"
                  aria-label="Выйти"
                  title="Выйти"
                  disabled={busy}
                  onClick={() => void handleLogout()}
                >
                  <LogOut size={19} />
                </button>
              </>
            ) : (
              <Link className="button secondary small" to="/login">
                Личный кабинет <ArrowUpRight size={17} />
              </Link>
            )}
          </div>
        </div>
      </header>
      <main id="main" ref={main} tabIndex={-1} className="container main">
        <ErrorNotice message={error} />
        <Outlet />
      </main>
      <footer className="site-footer">
        <div className="container footer-inner">
          <div>
            <Link className="footer-brand" to="/">
              мастерская.
            </Link>
            <p>Технике — вторую жизнь. Вам — спокойствие.</p>
            <small>Учебный проект · Онлайн-запись на ремонт</small>
          </div>
          <div className="footer-links">
            <Link to="/privacy">Обработка данных</Link>
            <Link to="/cookies">Политика cookie</Link>
            <span>Ежедневно, 10:00–18:00 · МСК</span>
          </div>
        </div>
      </footer>
      <CookieNotice />
    </>
  );
}
