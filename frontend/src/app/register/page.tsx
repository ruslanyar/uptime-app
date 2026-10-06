import Link from 'next/link';
import { AuthForm } from '@/components/auth-form';

export default function Register() {
  return (
    <>
      <p className="eyebrow">ВАШ АККАУНТ</p>
      <h1>Создать аккаунт</h1>
      <p className="muted">Начните с личного аккаунта в Uptime.</p>
      <AuthForm
        register
        footer={
          <p className="footer-link">
            Уже есть аккаунт? <Link href="/login">Войти</Link>
          </p>
        }
      />
    </>
  );
}
