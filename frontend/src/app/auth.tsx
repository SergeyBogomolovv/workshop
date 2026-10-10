/** Хранение текущего профиля при недоступном JavaScript сессионном cookie HttpOnly. */
import { createContext, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { api, clearCSRF, errorMessage } from "../api/client";
import type { User } from "../api/types";

interface AuthState {
  user: User | null;
  loading: boolean;
  error: string;
  setUser: (user: User | null) => void;
  logout: () => Promise<void>;
}
const AuthContext = createContext<AuthState | null>(null);

/** Однократно определяет текущую сессию без ожидаемой ошибки 401 для гостя. */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    void api<{ user: User | null }>("/auth/session", {
      signal: controller.signal,
    })
      .then((data) => setUser(data.user))
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setError(errorMessage(error));
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      });

    return () => controller.abort();
  }, []);
  /** Отзывает сессию перед обновлением отображаемого состояния учётной записи. */
  async function logout() {
    await api("/auth/logout", { method: "POST" });
    clearCSRF();
    setUser(null);
  }

  return (
    <AuthContext.Provider value={{ user, loading, error, setUser, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

/** Возвращает состояние авторизации внутри провайдера приложения. */
export function useAuth(): AuthState {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("AuthProvider is required");
  }

  return value;
}
