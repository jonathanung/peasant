'use client';

import { type ReactNode } from 'react';
import { WebSocketProvider } from '@/contexts/WebSocketContext';
import { ServerCapabilitiesProvider } from '@/contexts/ServerCapabilitiesContext';
import { PublishProvider } from '@/contexts/PublishContext';
import { AppSidebarLayout } from '@/lib/ft-ui';
import { LocalSidebar } from '@/components/LocalSidebar';
import { LocalOfflineNotice } from '@/components/LocalOfflineNotice';
import { CommandPalette } from '@/components/command/CommandPalette';
import { DevAnnotateOverlay } from '@/components/dev/DevAnnotateOverlay';
/**
 * LayoutShell wraps the app in the WebSocket provider and renders the
 * persistent chrome: fairtrade's sidebar beside the page (a top bar with a
 * drawer on a phone) and, at the top of the page column while the local app is
 * unreachable, the offline notice (pinned where the screen has room, scrolling
 * with the page elsewhere — see LocalOfflineNotice). The WebSocket connection
 * lives here — pages subscribe/unsubscribe to channels without tearing down the
 * socket.
 *
 * The first-run tour (components/tour) is not mounted. Its provider, steps and
 * the `data-tour` anchors stay in the tree so it can come back.
 * The publish scan cache and in-flight publishes also live here,
 * so they outlive the pages that use them.
 */
export function LayoutShell({ children }: { children: ReactNode }) {
  return (
    <WebSocketProvider>
      <ServerCapabilitiesProvider>
        <PublishProvider>
          <AppSidebarLayout sidebar={<LocalSidebar />}>
            <LocalOfflineNotice />
            {children}
          </AppSidebarLayout>
          <CommandPalette />
          {process.env.NODE_ENV === 'development' && <DevAnnotateOverlay />}
        </PublishProvider>
      </ServerCapabilitiesProvider>
    </WebSocketProvider>
  );
}
