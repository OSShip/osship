import './globals.css';
import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'OSShip — Open Source Mentorship',
  description: 'Paid mentorship on real OSS projects',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <header className="header">
          <Link href="/" className="logo">OSShip</Link>
          <nav>
            <Link href="/">Listings</Link>
            <Link href="/login">Login</Link>
            <Link href="/register">Register</Link>
            <Link href="/dashboard/student">Student</Link>
            <Link href="/dashboard/mentor">Mentor</Link>
            <Link href="/dashboard/admin/applications">Admin</Link>
          </nav>
        </header>
        <main className="container">{children}</main>
      </body>
    </html>
  );
}
