'use client';

import { useEffect, useState } from 'react';
import { api, getStoredUser, Session } from '@/lib/api';
import { JitsiEmbed } from '@/components';

export default function StudentDashboard() {
  const user = getStoredUser();
  const [enrollments, setEnrollments] = useState<{ id: string; listing_id: string; status: string }[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [prUrl, setPrUrl] = useState('');
  const [joinUrl, setJoinUrl] = useState('');

  useEffect(() => {
    if (!user) return;
    api<typeof enrollments>(`/users/${user.id}/enrollments`).then(setEnrollments).catch(() => {});
  }, [user]);

  async function loadSessions(listingId: string) {
    const data = await api<Session[]>(`/sessions/listings/${listingId}`);
    setSessions(data);
  }

  async function joinSession(sessionId: string) {
    const data = await api<{ jitsi_url: string }>(`/sessions/${sessionId}/join`, { method: 'POST' });
    setJoinUrl(data.jitsi_url);
  }

  async function linkPR() {
    if (!user) return;
    await api('/users/me/contributions', { method: 'POST', body: JSON.stringify({ pr_url: prUrl }) });
    setPrUrl('');
    alert('PR linked!');
  }

  if (!user) return <p>Please <a href="/login">login</a>.</p>;

  return (
    <>
      <h1>Student Dashboard</h1>
      <p className="muted">Welcome, {user.display_name || user.email}</p>

      <section className="section">
        <h2>My Enrollments</h2>
        {enrollments.length === 0 ? <p className="muted">No enrollments yet.</p> : (
          <ul className="stats">
            {enrollments.map((e) => (
              <li key={e.id}>
                Listing {e.listing_id.slice(0, 8)}... — {e.status}
                <button className="btn secondary" style={{ marginLeft: '1rem' }} onClick={() => loadSessions(e.listing_id)}>View sessions</button>
              </li>
            ))}
          </ul>
        )}
      </section>

      {sessions.length > 0 && (
        <section className="section">
          <h2>Sessions</h2>
          {sessions.map((s) => (
            <div key={s.id} className="card" style={{ marginBottom: '1rem' }}>
              <p>{new Date(s.scheduled_at).toLocaleString()} — {s.status}</p>
              <button className="btn" onClick={() => joinSession(s.id)}>Join Session</button>
            </div>
          ))}
        </section>
      )}

      {joinUrl && <JitsiEmbed url={joinUrl} />}

      <section className="section">
        <h2>Link Contribution (PR)</h2>
        <div className="form">
          <input placeholder="https://github.com/org/repo/pull/123" value={prUrl} onChange={(e) => setPrUrl(e.target.value)} />
          <button className="btn" onClick={linkPR}>Link PR</button>
        </div>
      </section>
    </>
  );
}
