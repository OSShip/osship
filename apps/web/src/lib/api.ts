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
  mentor_display_name?: string;
  mentor_github_username?: string;
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

export async function fetchListings(ossProject?: string): Promise<Listing[]> {
  const q = ossProject ? `&oss_project=${encodeURIComponent(ossProject)}` : '';
  return api<Listing[]>(`/listings?status=active${q}`);
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

export interface MentorApplication {
  id: string;
  user_id: string;
  status: string;
  github_data?: {
    summary?: {
      login?: string;
      public_repos?: number;
      followers?: number;
      total_prs?: number;
      recent_repos?: string[];
    };
    profile?: { login?: string; html_url?: string; bio?: string };
    pull_requests?: { total_count?: number; items?: Array<{ title?: string; html_url?: string; state?: string }> };
  };
  created_at?: string;
}

export const PLATFORM_FEE_PERCENT = 10;

export function calculateFees(priceCents: number) {
  const fee = Math.round(priceCents * PLATFORM_FEE_PERCENT / 100);
  return { fee, payout: priceCents - fee };
}

export interface ConnectStatus {
  connected: boolean;
  account_id?: string;
}

export async function fetchConnectStatus(): Promise<ConnectStatus> {
  return api<ConnectStatus>('/payments/connect/status');
}

export async function startStripeConnect(returnUrl: string, refreshUrl: string) {
  return api<{ onboarding_url: string; account_id: string }>('/payments/connect/onboard', {
    method: 'POST',
    body: JSON.stringify({ return_url: returnUrl, refresh_url: refreshUrl }),
  });
}

export async function fetchLedger(listingId: string) {
  return api<Array<{
    id: string;
    event_type: string;
    gross_cents: number;
    platform_fee_cents: number;
    mentor_payout_cents: number;
    created_at: string;
  }>>(`/payments/ledger/${listingId}`);
}

export async function applyMentor(githubUsername?: string) {
  return api<MentorApplication>('/mentors/apply', {
    method: 'POST',
    body: JSON.stringify({ github_username: githubUsername }),
  });
}

export async function fetchMentorApplications(status = 'pending') {
  return api<MentorApplication[]>(`/mentors/admin/applications?status=${status}`);
}

export async function reviewMentorApplication(id: string, status: 'approved' | 'rejected') {
  return api(`/mentors/admin/applications/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  });
}

export async function createListing(payload: Omit<Listing, 'id' | 'mentor_id' | 'filled_slots' | 'status'> & { status?: string }) {
  return api<Listing>('/listings', { method: 'POST', body: JSON.stringify(payload) });
}

export async function serverFetchListings(ossProject?: string): Promise<Listing[]> {
  try {
    const base = process.env.INTERNAL_API_URL || 'http://gateway:8080/api/v1';
    const q = ossProject ? `&oss_project=${encodeURIComponent(ossProject)}` : '';
    const res = await fetch(`${base}/listings?status=active${q}`, { next: { revalidate: 60 } });
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
