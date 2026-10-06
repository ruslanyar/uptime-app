import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuthForm } from "@/components/auth-form";
import { AuthError } from "@/lib/auth/api";
const { mutate, auth } = vi.hoisted(() => {
  const mutate = vi.fn();
  return { mutate, auth: { status: "anonymous", error: undefined as unknown, session: { mutate } } };
});
vi.mock("@/components/auth-provider", () => ({ useAuth: () => auth }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }) }));
describe("auth forms", () => {
  beforeEach(() => { auth.status = "anonymous"; auth.error = undefined; mutate.mockReset(); });
  it("shows field errors and password toggle", () => {
    render(<AuthForm register />);
    fireEvent.click(screen.getByRole("button", { name: "Создать аккаунт" }));
    expect(screen.getByText(/Введите имя длиной/)).toBeVisible();
    expect(screen.getByLabelText("Пароль")).toHaveAttribute("type", "password");
    fireEvent.click(screen.getByRole("button", { name: "Показать пароль" }));
    expect(screen.getByLabelText("Пароль")).toHaveAttribute("type", "text");
  });
  it.each([400, 401, 403, 409, 500])("shows server error %s and enables retry", async (status) => {
    mutate.mockRejectedValueOnce(new AuthError("http", status));
    render(<AuthForm />);
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@b.com" } });
    fireEvent.change(screen.getByLabelText("Пароль"), { target: { value: "my test password 123" } });
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    await waitFor(() => expect(screen.getByRole("alert")).toBeVisible());
    expect(screen.getByRole("button", { name: "Войти" })).toBeEnabled();
  });
  it("allows registration after a rejected login in another form", () => {
    auth.status = "error"; auth.error = new AuthError("http", 401);
    render(<AuthForm register />);
    expect(screen.getByLabelText("Имя")).toBeVisible();
  });
  it("disables duplicate submission", async () => {
    let done!: () => void;
    mutate.mockReturnValueOnce(new Promise<void>((resolve) => { done = resolve; }));
    render(<AuthForm />);
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@b.com" } });
    fireEvent.change(screen.getByLabelText("Пароль"), { target: { value: "my test password 123" } });
    const previous = mutate.mock.calls.length;
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect(screen.getByRole("button", { name: "Подождите…" })).toBeDisabled();
    done(); await waitFor(() => expect(screen.getByRole("button", { name: "Войти" })).toBeEnabled());
    expect(mutate.mock.calls.length).toBe(previous + 1);
  });
});
