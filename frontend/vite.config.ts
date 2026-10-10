/** Настройка React, Tailwind и перенаправления API-запросов с того же источника при разработке. */
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: { proxy: { "/api": "http://localhost:8088" } },
  build: { sourcemap: false },
});
