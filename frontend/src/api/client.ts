/** Загрузка JSON с того же источника без сохранения токенов авторизации в браузерном хранилище. */
export class ApiError extends Error {
  status: number;
  code: string;

  /** Сохраняет открытые поля ошибки сервера, не раскрывая внутренние детали передачи данных. */
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

let csrf: string | null = null;

/** Параметры JSON-запроса и способ проверки его сессии. */
interface RequestOptions {
  method?: string;
  body?: unknown;
  public?: boolean;
  signal?: AbortSignal;
}

/** Очищает CSRF-токен из памяти при изменении сессии пользователя. */
export function clearCSRF(): void {
  csrf = null;
}

/** Выполняет JSON-запрос, добавляя привязанный к сессии CSRF-токен для защищённых изменений. */
export async function api<T>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const method = options.method ?? "GET";
  const headers: Record<string, string> = {};

  if (options.body !== undefined) {
    headers["Content-Type"] = "application/json";
  }

  if (method !== "GET" && !options.public) {
    if (!csrf) {
      csrf = (await api<{ token: string }>("/auth/csrf")).token;
    }

    headers["X-CSRF-Token"] = csrf;
  }

  let response: Response;

  try {
    response = await fetch(`/api/v1${path}`, {
      method,
      headers,
      body:
        options.body === undefined ? undefined : JSON.stringify(options.body),
      credentials: "same-origin",
      signal: options.signal,
    });
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") {
      throw error;
    }

    throw new ApiError(
      0,
      "network",
      "Сервер недоступен. Проверьте подключение и попробуйте снова.",
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const data = await response.json().catch(() => null);

  if (!response.ok) {
    if (response.status === 401 || response.status === 403) {
      clearCSRF();
    }

    throw new ApiError(
      response.status,
      data?.code ?? "request",
      data?.message ?? "Не удалось выполнить запрос. Попробуйте позже.",
    );
  }

  if (data === null && path !== "/auth/session") {
    throw new ApiError(502, "response", "Сервер вернул некорректный ответ.");
  }

  return data as T;
}

/** Преобразует перехваченные ошибки в понятное пользователю сообщение. */
export function errorMessage(error: unknown): string {
  return error instanceof Error
    ? error.message
    : "Не удалось выполнить действие.";
}
