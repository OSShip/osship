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
          <Link href="/" className="logo">osship</Link>
          <nav>
            <Link href="/#listings">listings</Link>
            <Link href="/login">login</Link>
            <Link href="/register">register</Link>
            <Link href="/dashboard/student">student</Link>
            <Link href="/dashboard/mentor">mentor</Link>
            <Link href="/dashboard/admin/applications">admin</Link>
            <Link href="/dashboard/admin/ledger">ledger</Link>
          </nav>
        </header>
        <main className="container">{children}</main>
      </body>
    </html>
  );
}
