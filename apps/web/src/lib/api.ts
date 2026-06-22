const API_URL = process.env.NEXT_PUBLIC_API_URL || '/api/v1';

export interface User {
  id: string;
  email: string;
  role: string;
  display_name?: string;
  github_username?: string;
}

export interface Listing {
  id: string;
  mentor_id: string;
  oss_project_name: string;
  oss_repo_url: string;
  description: string;
  price_cents: number;
  duration_weeks: number;
  total_slots: number;
  filled_slots: number;
  status: string;
}

export interface PayoutSummary {
  total_gross_cents: number;
  total_mentor_payout_cents: number;
  total_platform_fee_cents: number;
  transaction_count: number;
}

export interface Session {
  id: string;
  listing_id: string;
  scheduled_at: string;
  jitsi_url: string;
  status: string;
}

function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem('token');
}

export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(`${API_URL}${path}`, { ...options, headers });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  return res.json();
}

export async function fetchListings(): Promise<Listing[]> {
  return api<Listing[]>('/listings?status=active');
}

export async function fetchListing(id: string): Promise<Listing> {
  return api<Listing>(`/listings/${id}`);
}

export async function fetchPayoutSummary(): Promise<PayoutSummary> {
  return api<PayoutSummary>('/public/payout-summary');
}

export async function login(email: string, password: string) {
  const data = await api<{ token: string; user: User }>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
  localStorage.setItem('token', data.token);
  localStorage.setItem('user', JSON.stringify(data.user));
  return data;
}

export async function register(payload: {
  email: string;
  password: string;
  role?: string;
  display_name?: string;
  github_username?: string;
}) {
  const data = await api<{ token: string; user: User }>('/auth/register', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
  localStorage.setItem('token', data.token);
  localStorage.setItem('user', JSON.stringify(data.user));
  return data;
}

export function getStoredUser(): User | null {
  if (typeof window === 'undefined') return null;
  const raw = localStorage.getItem('user');
  return raw ? JSON.parse(raw) : null;
}

export function logout() {
  localStorage.removeItem('token');
  localStorage.removeItem('user');
}

export function formatPrice(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

export async function serverFetchListings(): Promise<Listing[]> {
  try {
    const base = process.env.INTERNAL_API_URL || 'http://gateway:8080/api/v1';
    const res = await fetch(`${base}/listings?status=active`, { next: { revalidate: 60 } });
    if (!res.ok) return [];
    return res.json();
  } catch {
    return [];
  }
}

export async function serverFetchListing(id: string): Promise<Listing | null> {
  try {
    const base = process.env.INTERNAL_API_URL || 'http://gateway:8080/api/v1';
    const res = await fetch(`${base}/listings/${id}`, { next: { revalidate: 60 } });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}

export async function serverFetchPayoutSummary(): Promise<PayoutSummary | null> {
  try {
    const base = process.env.INTERNAL_API_URL || 'http://gateway:8080/api/v1';
    const res = await fetch(`${base}/public/payout-summary`, { next: { revalidate: 300 } });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}
