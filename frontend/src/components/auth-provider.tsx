"use client";
import { createContext, useContext, useEffect, useState, useSyncExternalStore } from "react";
import { AuthAPI } from "@/lib/auth/api";
import { Session } from "@/lib/auth/session";
import { BrowserCoordination } from "@/lib/auth/browser-coordination";
const Context = createContext<Session | null>(null);
export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [{ session, coordination }] = useState(() => {
    const coordination = new BrowserCoordination();
    return { coordination, session: new Session(new AuthAPI(), coordination) };
  });
  useEffect(() => {
    try { coordination.open((event) => session.receive(event)); }
    catch (error) { session.fail(error); return; }
    void session.check();
    const focus = () => { if (document.visibilityState === "visible") void session.check(); };
    window.addEventListener("focus", focus);
    document.addEventListener("visibilitychange", focus);
    return () => { coordination.close(); window.removeEventListener("focus", focus); document.removeEventListener("visibilitychange", focus); };
  }, [session, coordination]);
  return <Context value={session}>{children}</Context>;
}
export function useAuth() {
  const session = useContext(Context);
  if (!session) throw new Error("AuthProvider is required");
  const state = useSyncExternalStore(session.subscribe, session.snapshot, session.serverSnapshot);
  return { ...state, session };
}
