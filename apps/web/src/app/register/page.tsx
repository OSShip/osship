'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { register } from '@/lib/api';

export default function RegisterPage() {
  const router = useRouter();
  const [form, setForm] = useState({
    email: '', password: '', role: 'student', display_name: '', github_username: '',
  });
  const [error, setError] = useState('');

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    try {
      await register(form);
      router.push('/dashboard/student');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed');
    }
  }

  return (
    <>
      <h1>Register</h1>
      <form className="form" onSubmit={handleSubmit}>
        <input type="email" placeholder="Email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required />
        <input type="password" placeholder="Password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required />
        <select value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })}>
          <option value="student">Student</option>
          <option value="mentor">Mentor</option>
        </select>
        <input placeholder="Display name" value={form.display_name} onChange={(e) => setForm({ ...form, display_name: e.target.value })} />
        <input placeholder="GitHub username" value={form.github_username} onChange={(e) => setForm({ ...form, github_username: e.target.value })} />
        {error && <p className="error">{error}</p>}
        <button type="submit" className="btn">Register</button>
      </form>
    </>
  );
}
