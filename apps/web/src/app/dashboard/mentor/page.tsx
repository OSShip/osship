'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { api, getStoredUser } from '@/lib/api';

export default function MentorDashboard() {
  const user = getStoredUser();
  const router = useRouter();
  const [sessionForm, setSessionForm] = useState({ listing_id: '', scheduled_at: '' });

  async function applyMentor() {
    await api('/mentors/apply', { method: 'POST', body: JSON.stringify({ github_username: user?.github_username }) });
    alert('Application submitted!');
  }

  async function createSession(e: React.FormEvent) {
    e.preventDefault();
    await api('/sessions', {
      method: 'POST',
      body: JSON.stringify({ listing_id: sessionForm.listing_id, scheduled_at: new Date(sessionForm.scheduled_at).toISOString() }),
    });
    alert('Session scheduled!');
  }

  if (!user) return <p>Please <a href="/login">login</a>.</p>;

  return (
    <>
      <h1>Mentor Dashboard</h1>
      <p className="muted">Welcome, {user.display_name || user.email}</p>

      <section className="section">
        <button className="btn" onClick={() => router.push('/dashboard/mentor/listings/new')}>Create Listing</button>
        <button className="btn secondary" style={{ marginLeft: '1rem' }} onClick={applyMentor}>Apply as Mentor</button>
      </section>

      <section className="section">
        <h2>Schedule Session</h2>
        <form className="form" onSubmit={createSession}>
          <input placeholder="Listing ID" value={sessionForm.listing_id} onChange={(e) => setSessionForm({ ...sessionForm, listing_id: e.target.value })} required />
          <input type="datetime-local" value={sessionForm.scheduled_at} onChange={(e) => setSessionForm({ ...sessionForm, scheduled_at: e.target.value })} required />
          <button type="submit" className="btn">Schedule</button>
        </form>
      </section>
    </>
  );
}
