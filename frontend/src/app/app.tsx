/** Маршруты приложения, проверка ролей и экран восстановления после ошибки. */
import { Component } from "react";
import type { ReactNode } from "react";
import {
  BrowserRouter,
  Link,
  Navigate,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";
import { AuthProvider, useAuth } from "./auth";
import { Layout } from "./layout";
import { HomePage } from "../pages/home";
import { AuthPage } from "../pages/auth";
import { ProfilePage } from "../pages/profile";
import { BookingPage } from "../pages/booking";
import { AppointmentsPage } from "../pages/appointments";
import { DetailPage } from "../pages/detail";
import { LegalPage } from "../pages/legal";
import { Empty, ErrorNotice, Loading } from "../components/ui";

/** Ограничивает доступ к закрытым страницам; окончательную проверку прав выполняет сервер. */
function Guard({
  children,
  role,
}: {
  children: ReactNode;
  role?: "client" | "admin";
}) {
  const { user, loading, error } = useAuth();
  const location = useLocation();
  if (loading) {
    return <Loading />;
  }

  if (error && !user) {
    return <ErrorNotice message={error} />;
  }

  if (!user) {
    return (
      <Navigate
        to="/login"
        state={{ from: location.pathname + location.search }}
        replace
      />
    );
  }

  if (role && user.role !== role) {
    return (
      <Navigate
        to={user.role === "admin" ? "/admin/appointments" : "/appointments"}
        replace
      />
    );
  }

  return children;
}

/** Обрабатывает неизвестные адреса внутри общей навигационной оболочки приложения. */
function NotFound() {
  return (
    <Empty title="Страница не найдена">
      <p>Возможно, адрес изменился. Начнём с главной?</p>
      <Link className="button primary" to="/">
        На главную
      </Link>
    </Empty>
  );
}

/** Заменяет неожиданную ошибку отображения действием для восстановления страницы. */
class ErrorBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };
  /** Переключает интерфейс на экран восстановления после неперехваченной ошибки отображения. */
  static getDerivedStateFromError() {
    return { failed: true };
  }
  /** Отображает приложение или безопасное предложение перезагрузить страницу. */
  render() {
    return this.state.failed ? (
      <div className="container empty">
        <h1>Не удалось открыть страницу</h1>
        <p>Обновите страницу и попробуйте снова.</p>
        <button
          type="button"
          className="button primary"
          onClick={() => window.location.reload()}
        >
          Обновить
        </button>
      </div>
    ) : (
      this.props.children
    );
  }
}

/** Объединяет общедоступные, клиентские и административные маршруты. */
export function App() {
  return (
    <ErrorBoundary>
      <BrowserRouter>
        <AuthProvider>
          <Routes>
            <Route element={<Layout />}>
              <Route index element={<HomePage />} />
              <Route path="login" element={<AuthPage key="login" />} />
              <Route
                path="register"
                element={<AuthPage key="register" register />}
              />
              <Route
                path="profile"
                element={
                  <Guard>
                    <ProfilePage />
                  </Guard>
                }
              />
              <Route
                path="appointments"
                element={
                  <Guard role="client">
                    <AppointmentsPage />
                  </Guard>
                }
              />
              <Route
                path="appointments/new"
                element={
                  <Guard role="client">
                    <BookingPage />
                  </Guard>
                }
              />
              <Route
                path="appointments/:id"
                element={
                  <Guard role="client">
                    <DetailPage />
                  </Guard>
                }
              />
              <Route
                path="admin/appointments"
                element={
                  <Guard role="admin">
                    <AppointmentsPage admin />
                  </Guard>
                }
              />
              <Route
                path="admin/appointments/:id"
                element={
                  <Guard role="admin">
                    <DetailPage admin />
                  </Guard>
                }
              />
              <Route path="privacy" element={<LegalPage />} />
              <Route path="cookies" element={<LegalPage cookies />} />
              <Route path="*" element={<NotFound />} />
            </Route>
          </Routes>
        </AuthProvider>
      </BrowserRouter>
    </ErrorBoundary>
  );
}
