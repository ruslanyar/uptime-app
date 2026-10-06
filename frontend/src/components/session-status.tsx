"use client";
import { useAuth } from "./auth-provider";
import { AuthError, errorMessage } from "@/lib/auth/api";
export function SessionStatus() {
  const { status, error, session } = useAuth();
  if (status === "error") return <div className="notice" role="alert"><p>{errorMessage(error)}</p>{!(error instanceof AuthError && ["unsupported", "config"].includes(error.kind)) && <button className="secondary" onClick={() => void session.check(true)}>Повторить проверку</button>}</div>;
  return <p role="status" className="muted">Проверяем сессию…</p>;
}
