'use client';

import { useEffect, useState } from 'react';
import {
  fetchMentorApplications,
  reviewMentorApplication,
  type MentorApplication,
} from '@/lib/api';

function GitHubSummary({ data }: { data?: MentorApplication['github_data'] }) {
  const summary = data?.summary;
  const profile = data?.profile;
  const prs = data?.pull_requests;

  if (!summary && !profile) {
    return <p className="muted">No GitHub data available.</p>;
  }

  return (
    <div className="github-summary">
      {profile?.login && (
        <p>
          <a href={profile.html_url || `https://github.com/${profile.login}`} target="_blank" rel="noopener noreferrer">
            @{profile.login}
          </a>
          {profile.bio && <span className="muted"> — {profile.bio}</span>}
        </p>
      )}
      <ul className="stats">
        {summary?.public_repos != null && <li>Public repos: <strong>{summary.public_repos}</strong></li>}
        {summary?.followers != null && <li>Followers: <strong>{summary.followers}</strong></li>}
        {(summary?.total_prs ?? prs?.total_count) != null && (
          <li>Pull requests: <strong>{summary?.total_prs ?? prs?.total_count}</strong></li>
        )}
        {summary?.recent_repos && summary.recent_repos.length > 0 && (
          <li>Recent repos: {summary.recent_repos.join(', ')}</li>
        )}
      </ul>
      {prs?.items && prs.items.length > 0 && (
        <details>
          <summary>Recent PRs</summary>
          <ul className="stats">
            {prs.items.slice(0, 5).map((pr) => (
              <li key={pr.html_url}>
                <a href={pr.html_url} target="_blank" rel="noopener noreferrer">{pr.title}</a>
                <span className="muted"> ({pr.state})</span>
              </li>
            ))}
          </ul>
        </details>
      )}
    </div>
  );
}

export default function AdminApplicationsPage() {
  const [apps, setApps] = useState<MentorApplication[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchMentorApplications('pending')
      .then(setApps)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load applications'))
      .finally(() => setLoading(false));
  }, []);

  async function review(id: string, status: 'approved' | 'rejected') {
    try {
      await reviewMentorApplication(id, status);
      setApps((prev) => prev.filter((a) => a.id !== id));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Review failed');
    }
  }

  return (
    <>
      <h1>Mentor Applications</h1>
      <p className="muted">Review GitHub contribution history before approving mentors.</p>
      {error && <p className="error">{error}</p>}
      {loading && <p className="muted">Loading applications...</p>}
      {!loading && apps.length === 0 ? (
        <p className="muted">No pending applications.</p>
      ) : (
        apps.map((a) => (
          <div key={a.id} className="card application-card">
            <p>User ID: <code>{a.user_id}</code></p>
            <GitHubSummary data={a.github_data} />
            <div className="actions">
              <button type="button" className="btn" onClick={() => review(a.id, 'approved')}>Approve</button>
              <button type="button" className="btn secondary" onClick={() => review(a.id, 'rejected')}>Reject</button>
            </div>
          </div>
        ))
      )}
    </>
  );
}
