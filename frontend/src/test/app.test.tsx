/** Проверка пользовательских форм и HTML всех групп маршрутов после отображения. */
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HtmlValidate } from "html-validate";
import { App } from "../app/app";
import type { User, Catalog, Appointment } from "../api/types";
import { calendarDate } from "../api/types";

const account: User = {
  id: "20000000-0000-4000-8000-000000000001",
  login: "sample",
  name: "Анна",
  phone: "+79991234567",
  email: "anna@example.ru",
  role: "client",
};
const catalog: Catalog = {
  categories: [{ code: "laptop", name: "Ноутбук" }],
  services: [
    {
      id: "10000000-0000-4000-8000-000000000001",
      name: "Диагностика",
      description: "Проверка устройства",
      price_kopecks: 50000,
      categories: ["laptop"],
    },
  ],
};
const appointment: Appointment = {
  id: "30000000-0000-4000-8000-000000000001",
  service_id: catalog.services[0].id,
  category: "laptop",
  slot_start: `${calendarDate(1)}T10:00:00+03:00`,
  description: "Ноутбук не включается",
  status: "new",
  created_at: "2026-10-10T08:00:00Z",
  updated_at: "2026-10-10T08:00:00Z",
  service_name: "Диагностика",
  category_name: "Ноутбук",
};

/** Задаёт предсказуемые ответы запросов, сохраняя настоящую логику маршрутов и форм. */
function mockAPI(user: User | null = null) {
  const fetchMock = vi.fn(
    async (input: string | URL | Request, init?: RequestInit) => {
      const path = String(input).replace("/api/v1", "");
      let data: unknown = {};
      if (path === "/auth/session") {
        data = { user };
      } else if (path === "/catalog") {
        data = catalog;
      } else if (path === "/auth/csrf") {
        data = { token: "csrf-test-value" };
      } else if (path === "/auth/login") {
        data = account;
      } else if (path.startsWith("/slots")) {
        data = [appointment.slot_start];
      } else if (path === "/profile") {
        data = account;
      } else if (path.includes(`/appointments/${appointment.id}`)) {
        data = {
          appointment,
          history: [
            {
              previous_status: "",
              status: "new",
              created_at: appointment.created_at,
            },
          ],
          ...(user?.role === "admin" ? { client: account } : {}),
        };
      } else if (
        path.startsWith("/appointments") ||
        path.startsWith("/admin/appointments")
      ) {
        data =
          init?.method === "POST"
            ? { id: appointment.id }
            : { items: [appointment], page: 1, has_more: false };
      }

      return new Response(JSON.stringify(data), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    },
  );
  vi.stubGlobal("fetch", fetchMock);

  return fetchMock;
}

/** Проверяет нормализованную браузером разметку, включая подписи, вложенность и уникальность ID. */
async function validateHTML() {
  // HTML допускает пустые значения булевых атрибутов и любой регистр DOCTYPE; autocomplete задан по условиям задания.
  const validator = new HtmlValidate({
    extends: ["html-validate:recommended"],
    rules: {
      "void-style": "off",
      "attribute-boolean-style": "off",
      "doctype-style": "off",
      "autocomplete-password": "off",
      "prefer-native-element": "off",
    },
  });
  const markup = `<!doctype html><html lang="ru"><head><meta charset="UTF-8"><title>Мастерская</title></head><body>${document.body.innerHTML}</body></html>`;
  const report = await validator.validateString(markup);
  expect(
    report.results.flatMap((item) =>
      item.messages.map((message) => `${message.ruleId}: ${message.message}`),
    ),
  ).toEqual([]);
}

describe("public experience", () => {
  it("persists the necessary-cookie notice and loads the catalog", async () => {
    mockAPI();
    render(<App />);
    await screen.findByRole("heading", { name: "Диагностика" });
    await validateHTML();
    await userEvent.click(screen.getByRole("button", { name: "Понятно" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(localStorage.getItem("workshop-cookie-notice")).toBe("1");
  });
  it("redirects a private route to login and validates required fields", async () => {
    mockAPI();
    window.history.replaceState(null, "", "/appointments/new");
    render(<App />);
    await screen.findByRole("heading", { name: "С возвращением" });
    expect(window.location.pathname).toBe("/login");
    const input = screen.getByLabelText("Логин") as HTMLInputElement;
    expect(input.required).toBe(true);
    expect(input.checkValidity()).toBe(false);
    await validateHTML();
  });
  it("does not send registration without consent and rejects mismatched passwords", async () => {
    const fetchMock = mockAPI();
    window.history.replaceState(null, "", "/register");
    render(<App />);
    await screen.findByRole("heading", { name: "Давайте познакомимся" });
    const form = screen.getByLabelText("Логин").closest("form")!;
    for (const [label, value] of [
      ["Логин", "sample"],
      ["Пароль", "long secure phrase 2026"],
      ["Повторите пароль", "long secure phrase 2026"],
      ["Ваше имя", "Анна"],
      ["Телефон", "+79991234567"],
      ["Электронная почта", "anna@example.ru"],
    ]) {
      fireEvent.change(screen.getByLabelText(label, { exact: true }), {
        target: { value },
      });
    }

    await validateHTML();
    fireEvent.submit(form);
    await screen.findByText("Необходимо согласие на обработку данных.");
    expect(
      fetchMock.mock.calls.some(([path]) =>
        String(path).includes("/auth/register"),
      ),
    ).toBe(false);
    await userEvent.click(screen.getByRole("checkbox"));
    fireEvent.change(screen.getByLabelText("Повторите пароль"), {
      target: { value: "another long phrase" },
    });
    fireEvent.submit(form);
    await screen.findByText("Пароли не совпадают.");
    fireEvent.change(screen.getByLabelText("Повторите пароль"), {
      target: { value: "long secure phrase 2026" },
    });
    fireEvent.submit(form);
    await screen.findByText(
      "Аккаунт создан. Войдите с вашим логином и паролем.",
    );
    const sent = fetchMock.mock.calls.find(([path]) =>
      String(path).includes("/auth/register"),
    )?.[1];
    expect(JSON.parse(String(sent?.body))).toMatchObject({
      consent: true,
      policy_version: "2026-10-10",
    });
    expect(JSON.parse(String(sent?.body))).not.toHaveProperty("confirm");
  });
  it.each([
    ["/privacy", "Обработка персональных данных"],
    ["/cookies", "Политика cookie"],
    ["/missing", "Страница не найдена"],
  ])("renders valid markup for %s", async (route, title) => {
    mockAPI();
    window.history.replaceState(null, "", route);
    render(<App />);
    await screen.findByRole("heading", { name: title });
    await validateHTML();
  });
});

describe("private experience", () => {
  it.each([
    ["/profile", "Ваш профиль", false],
    ["/appointments", "Мои заявки", false],
    [`/appointments/${appointment.id}`, "Диагностика", false],
    ["/admin/appointments", "Заявки на ремонт", true],
    [`/admin/appointments/${appointment.id}`, "Диагностика", true],
  ])(
    "renders valid authenticated markup for %s",
    async (route, title, admin) => {
      mockAPI({ ...account, role: admin ? "admin" : "client" });
      window.history.replaceState(null, "", route);
      render(<App />);
      await screen.findByRole("heading", { name: title, level: 1 });
      await waitFor(() =>
        expect(screen.queryByText("Загружаем…")).not.toBeInTheDocument(),
      );
      await validateHTML();
    },
  );
  it("books a server-provided slot and attaches CSRF", async () => {
    const fetchMock = mockAPI(account);
    window.history.replaceState(null, "", "/appointments/new");
    render(<App />);
    await screen.findByLabelText("Ваше устройство");
    await userEvent.selectOptions(
      screen.getByLabelText("Ваше устройство"),
      "laptop",
    );
    await userEvent.selectOptions(
      screen.getByLabelText("Что нужно сделать"),
      catalog.services[0].id,
    );
    await waitFor(() =>
      expect(screen.getByLabelText("Время приёма")).not.toBeDisabled(),
    );
    await userEvent.selectOptions(
      screen.getByLabelText("Время приёма"),
      appointment.slot_start,
    );
    fireEvent.change(screen.getByLabelText("Что случилось с устройством?"), {
      target: { value: appointment.description },
    });
    await validateHTML();
    await userEvent.click(
      screen.getByRole("button", { name: "Подтвердить запись" }),
    );
    await screen.findByText(
      "Вы записаны! Приносите устройство в выбранное время.",
    );
    const sent = fetchMock.mock.calls.find(
      ([path, init]) =>
        String(path) === "/api/v1/appointments" && init?.method === "POST",
    )?.[1];
    expect(sent?.headers).toMatchObject({ "X-CSRF-Token": "csrf-test-value" });
    expect(JSON.parse(String(sent?.body))).toMatchObject({
      slot_start: appointment.slot_start,
    });
  });
});
