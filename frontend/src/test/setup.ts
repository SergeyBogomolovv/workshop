/** Подключение проверок DOM и сброс состояния браузерного окружения между тестами. */
import "@testing-library/jest-dom/vitest";
import { afterEach, beforeEach, expect, vi } from "vitest";
import { cleanup } from "@testing-library/react";
import { clearCSRF } from "../api/client";

beforeEach(() => {
  localStorage.clear();
  clearCSRF();
  window.history.replaceState(null, "", "/");
  vi.stubGlobal("scrollTo", vi.fn());
  vi.spyOn(console, "error").mockImplementation(() => {});
  vi.spyOn(console, "warn").mockImplementation(() => {});
  vi.spyOn(console, "log").mockImplementation(() => {});
});
afterEach(() => {
  cleanup();
  expect(console.error).not.toHaveBeenCalled();
  expect(console.warn).not.toHaveBeenCalled();
  expect(console.log).not.toHaveBeenCalled();
  vi.unstubAllGlobals();
});
