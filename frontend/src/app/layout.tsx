import type { Metadata } from 'next';
import Link from 'next/link';
import { AuthProvider } from '@/components/auth-provider';
import './globals.css';

export const metadata: Metadata = {
  title: 'Uptime — аккаунт',
  description: 'Регистрация и вход в Uptime',
};
export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="ru">
      <body>
        <div className="shell">
          <header>
            <Link className="brand" href="/">
              <span aria-hidden="true" />
              uptime
            </Link>
          </header>
          <main>
            <section className="card">
              <AuthProvider>{children}</AuthProvider>
            </section>
          </main>
        </div>
      </body>
    </html>
  );
}
