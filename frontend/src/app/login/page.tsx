import Link from 'next/link';
import { AuthForm } from '@/components/auth-form';

export default function Login() {
  return (
    <>
      <p className="eyebrow">ВАШ АККАУНТ</p>
      <h1>Войти</h1>
      <p className="muted">Рады видеть вас снова в Uptime.</p>
      <AuthForm
        footer={
          <p className="footer-link">
            Нет аккаунта? <Link href="/register">Зарегистрироваться</Link>
          </p>
        }
      />
    </>
  );
}
