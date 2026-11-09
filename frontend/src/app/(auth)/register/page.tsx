import type { Metadata } from 'next';
import { AuthShell } from '@/components/layout/AuthShell';
import { RegisterForm } from '@/components/auth/RegisterForm';

export const metadata: Metadata = { title: 'Create an account' };

export default function RegisterPage() {
  return (
    <AuthShell eyebrow="Your next plan starts here" title="Create your account" description="Join BiletFlow to discover events and make plans with your community.">
      <RegisterForm />
    </AuthShell>
  );
}
