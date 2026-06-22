'use client';

import { useEffect, useState } from 'react';
import { api } from '@/lib/api';

interface Application {
  id: string;
  user_id: string;
  status: string;
  github_data?: unknown;
}

export default function AdminApplicationsPage() {
  const [apps, setApps] = useState<Application[]>([]);

  useEffect(() => {
    api<Application[]>('/mentors/admin/applications?status=pending').then(setApps).catch(() => {});
  }, []);

  async function review(id: string, status: 'approved' | 'rejected') {
    await api(`/mentors/admin/applications/${id}`, { method: 'PATCH', body: JSON.stringify({ status }) });
    setApps((prev) => prev.filter((a) => a.id !== id));
  }

  return (
    <>
      <h1>Mentor Applications</h1>
      {apps.length === 0 ? <p className="muted">No pending applications.</p> : (
        apps.map((a) => (
          <div key={a.id} className="card" style={{ marginBottom: '1rem' }}>
            <p>User: {a.user_id}</p>
            <pre style={{ fontSize: '0.75rem', overflow: 'auto', maxHeight: 200 }}>{JSON.stringify(a.github_data, null, 2)}</pre>
            <button className="btn" onClick={() => review(a.id, 'approved')}>Approve</button>
            <button className="btn secondary" style={{ marginLeft: '0.5rem' }} onClick={() => review(a.id, 'rejected')}>Reject</button>
          </div>
        ))
      )}
    </>
  );
}
