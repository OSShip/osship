'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';

const TAGLINE =
  'Structured mentorship on real OSS projects. Transparent payouts. Live sessions. Verifiable progress.';

export function Hero() {
  const [typed, setTyped] = useState('');
  const [showCursor, setShowCursor] = useState(true);

  useEffect(() => {
    let i = 0;
    const interval = setInterval(() => {
      i += 1;
      setTyped(TAGLINE.slice(0, i));
      if (i >= TAGLINE.length) clearInterval(interval);
    }, 28);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    const blink = setInterval(() => setShowCursor((v) => !v), 530);
    return () => clearInterval(blink);
  }, []);

  return (
    <section className="landing-hero">
      <div className="terminal-window">
        <div className="terminal-titlebar">
          <span className="terminal-dots">
            <i />
            <i />
            <i />
          </span>
          <span className="terminal-title">osship — tty1</span>
        </div>
        <div className="terminal-body">
          <pre className="ascii-logo" aria-hidden="true">{`
   ___  ____  ____  _   _ ___ ____  
  / _ \\/ ___||  _ \\| | | |_ _|  _ \\ 
 | | | \\___ \\| |_) | |_| || || |_) |
 | |_| |___) |  __/|  _  || ||  __/ 
  \\___/|____/|_|   |_| |_|___|_|    
          `}</pre>

          <p className="terminal-line">
            <span className="prompt">guest@osship</span>
            <span className="prompt-sep">:</span>
            <span className="prompt-path">~</span>
            <span className="prompt-sep">$</span>{' '}
            <span className="cmd">./osship --init</span>
          </p>

          <h1 className="hero-heading">
            <span className="hero-prefix">&gt; </span>
            Open Source Mentorship
          </h1>

          <p className="hero-tagline">
            {typed}
            <span className={`cursor ${showCursor ? 'visible' : ''}`}>█</span>
          </p>

          <div className="hero-actions">
            <Link href="#listings" className="term-btn">
              <span className="term-btn-prefix">$</span> browse --listings
            </Link>
            <Link href="/register" className="term-btn term-btn-ghost">
              <span className="term-btn-prefix">$</span> register --new-user
            </Link>
          </div>

          <p className="terminal-line terminal-muted">
            <span className="prompt">sys</span>
            <span className="prompt-sep">@</span>
            <span className="prompt-path">osship</span>
            <span className="prompt-sep">$</span>{' '}
            echo &quot;status: online&quot; &amp;&amp; uptime
          </p>
          <p className="terminal-output">
            status: online &nbsp;·&nbsp; mentors ready &nbsp;·&nbsp; ledger public
          </p>
        </div>
      </div>
    </section>
  );
}
