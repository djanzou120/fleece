/**
 * Root page — redirect to dashboard or login.
 * In a real app this would check the session and redirect accordingly.
 * For the scaffold, we redirect to the dashboard.
 */
import { redirect } from 'next/navigation';

export default function RootPage() {
  redirect('/dashboard');
}
