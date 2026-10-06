import type { Metadata } from 'next';
import { JetBrains_Mono, Manrope } from 'next/font/google';
import { AuthProvider } from '@/components/auth-provider';
import './globals.css';

const manrope = Manrope({
  subsets: ['latin', 'cyrillic'],
  display: 'swap',
  style: 'normal',
  variable: '--font-manrope',
});
const jetbrainsMono = JetBrains_Mono({
  subsets: ['latin', 'cyrillic'],
  display: 'swap',
  style: 'normal',
  variable: '--font-jetbrains-mono',
});

export const metadata: Metadata = {
  title: 'Uptime — аккаунт',
  description: 'Регистрация и вход в Uptime',
};
export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html
      lang="ru"
      className={`dark ${manrope.variable} ${jetbrainsMono.variable}`}
    >
      <body>
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
