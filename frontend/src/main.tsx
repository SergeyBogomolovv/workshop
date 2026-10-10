/** Запуск типизированного приложения React и подключение общей таблицы стилей. */
import { createRoot } from "react-dom/client";
import { App } from "./app/app";
import "./styles/main.css";

const root = document.getElementById("root");
if (root) {
  createRoot(root).render(<App />);
}
