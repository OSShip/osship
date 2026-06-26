'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { api, createListing, getStoredUser } from '@/lib/api';

export default function NewListingPage() {
  const user = getStoredUser();
  const router = useRouter();
  const [form, setForm] = useState({
    oss_project_name: '', oss_repo_url: '', description: '',
    price_cents: 5000, duration_weeks: 4, total_slots: 5,
  });
  const [error, setError] = useState('');

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    try {
      await createListing(form);
      router.push('/dashboard/mentor');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create listing');
    }
  }

  if (!user) return <p>Please <a href="/login">login</a>.</p>;

  return (
    <>
      <h1>New Mentorship Listing</h1>
      <form className="form" onSubmit={handleSubmit}>
        <input placeholder="Project name" value={form.oss_project_name} onChange={(e) => setForm({ ...form, oss_project_name: e.target.value })} required />
        <input placeholder="Repo URL" value={form.oss_repo_url} onChange={(e) => setForm({ ...form, oss_repo_url: e.target.value })} required />
        <textarea placeholder="Description" value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} rows={4} required />
        <input type="number" placeholder="Price (cents)" value={form.price_cents} onChange={(e) => setForm({ ...form, price_cents: +e.target.value })} />
        <input type="number" placeholder="Duration (weeks)" value={form.duration_weeks} onChange={(e) => setForm({ ...form, duration_weeks: +e.target.value })} />
        <input type="number" placeholder="Total slots" value={form.total_slots} onChange={(e) => setForm({ ...form, total_slots: +e.target.value })} />
        {error && <p className="error">{error}</p>}
        <button type="submit" className="btn">Publish Listing</button>
      </form>
    </>
  );
}
