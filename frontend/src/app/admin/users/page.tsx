import type { Metadata } from 'next';
import { AdminUsers } from '@/components/admin-users';

export const metadata: Metadata = { title: 'Пользователи — Uptime' };

export default function AdminUsersPage() {
  return <AdminUsers />;
}
