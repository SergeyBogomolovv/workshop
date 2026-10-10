/** Загрузка ресурсов API с возможностью отмены и явными состояниями ожидания и ошибки. */
import { useEffect, useState } from "react";
import { api, errorMessage } from "./client";

/** Повторяет загрузку при смене маршрута или счётчика обновлений и игнорирует устаревшие запросы. */
export function useResource<T>(path: string, revision = 0) {
  const [state, setState] = useState<{
    data: T | null;
    error: string;
    loading: boolean;
  }>({ data: null, error: "", loading: true });
  useEffect(() => {
    const controller = new AbortController();
    setState({ data: null, error: "", loading: true });
    void api<T>(path, { signal: controller.signal })
      .then((data) => setState({ data, error: "", loading: false }))
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setState({ data: null, error: errorMessage(error), loading: false });
        }
      });

    return () => controller.abort();
  }, [path, revision]);

  return state;
}
