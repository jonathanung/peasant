'use client';

import { useState } from 'react';
import { X, Zap } from 'lucide-react';
import { ProviderName } from '@/lib/ft-ui';

/** The Home reminder; dismissing it changes no publishing preference. */
export function AutoPublishTip() {
  const [dismissed, setDismissed] = useState(false);
  if (dismissed) return null;
  return (
    <aside aria-label="auto-publish tip" className="home-tip">
      <Zap className="home-tip-icon" aria-hidden="true" />
      <p className="home-tip-copy">
        tick auto-publish when you publish, and later pushes publish on their own. or run{' '}
        <code className="home-tip-command">/peasant auto</code> in <ProviderName harness="claude-code" />.
      </p>
      <button type="button" className="btn btn-ghost btn-sm btn-icon home-tip-dismiss" aria-label="dismiss auto-publish tip" onClick={() => setDismissed(true)}>
        <X aria-hidden="true" />
      </button>
    </aside>
  );
}
