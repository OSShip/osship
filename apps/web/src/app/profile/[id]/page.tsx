'use client';

import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import { api } from '@/lib/api';

export default function ProfilePage() {
  const { id } = useParams<{ id: string }>();
  const [profile, setProfile] = useState<{ display_name?: string; bio?: string; github_username?: string; role: string } | null>(null);
  const [contributions, setContributions] = useState<{ pr_url: string; github_verified: boolean }[]>([]);

  useEffect(() => {
    api<typeof profile>(`/users/${id}/profile`).then(setProfile).catch(() => {});
  }, [id]);

  return (
    <>
      {profile ? (
        <>
          <h1>{profile.display_name || 'Profile'}</h1>
          <p className="muted">Role: {profile.role}</p>
          {profile.github_username && <p>GitHub: @{profile.github_username}</p>}
          {profile.bio && <p>{profile.bio}</p>}
        </>
      ) : (
        <p>Loading...</p>
      )}
    </>
  );
}
