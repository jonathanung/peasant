'use client';

import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Moon, Search, Sun, Users } from 'lucide-react';
import type { SyncAuthResponse } from '@peasant-labs/schema';
import { AppSidebar, APP_SIDEBAR_APPS, type AppSidebarGroup } from '@/lib/ft-ui';
import { useTheme } from '@/hooks/useTheme';
import { useServerCapabilities } from '@/contexts/ServerCapabilitiesContext';
import { OPEN_COMMAND_PALETTE_EVENT } from '@/components/command/CommandPalette';
import { isSectionActive, visibleNavSections } from '@/lib/nav/sections';
import { settingsSectionHref } from '@/lib/settings/catalog';
import { fetchVillageAuth } from '@/lib/api/settings';
import { DOCS_URL, onlineCollectivesHref } from '@/lib/village/links';

/**
 * Where the switcher's other app lives: the docs, opened in a new tab. The
 * switcher leaves this app out, and online gets no address, so it is left out
 * too (an app with no href is omitted); the collectives link is the local
 * app's way to online.
 */
function switcherApps() {
  const urls: Partial<Record<string, string>> = { docs: DOCS_URL };
  return APP_SIDEBAR_APPS.map((entry) => {
    const href = urls[entry.id];
    return href ? { ...entry, href, target: '_blank', rel: 'noopener noreferrer' } : entry;
  });
}

/** The pages under Home: a project's sessions and a transcript. */
const UNDER_HOME = ['/sessions', '/projects'];

/**
 * The sign-in this computer publishes with, for the account row. A read that
 * fails leaves the row out: the sidebar never guesses who is signed in.
 */
function useSignIn(): SyncAuthResponse | null {
  const [auth, setAuth] = useState<SyncAuthResponse | null>(null);
  const read = useCallback(() => {
    let live = true;
    fetchVillageAuth().then((next) => { if (live) setAuth(next); }).catch(() => { if (live) setAuth(null); });
    return () => { live = false; };
  }, []);
  useEffect(() => {
    let cancel = read();
    // A sign-in or log-out happens in a terminal or another tab; read again on return.
    const onFocus = () => { cancel(); cancel = read(); };
    window.addEventListener('focus', onFocus);
    return () => { cancel(); window.removeEventListener('focus', onFocus); };
  }, [read]);
  return auth;
}

function NextLink({ href, ...rest }: { href: string } & React.AnchorHTMLAttributes<HTMLAnchorElement>) {
  return <Link href={href} {...rest} />;
}

/**
 * The local app's sidebar: fairtrade's AppSidebar, with the nav derived from
 * the section registry (lib/nav/sections.ts). Route-only sections (online,
 * analytics, changes, the code map) and `/share` are not linked from it. A
 * settings page keeps this same nav, settings current: its sections are the
 * in-page section bar of the settings page itself. Search (the command
 * palette) and the theme toggle sit in their own group, so they stay reachable
 * in the collapsed rail and in the phone drawer. The collectives link opens
 * online; the app switcher opens the docs.
 */
export function LocalSidebar() {
  const pathname = usePathname() ?? '/';
  const { theme, toggle } = useTheme();
  const { capabilities } = useServerCapabilities();
  const auth = useSignIn();
  const nav = visibleNavSections(capabilities);
  const home = nav.find((entry) => entry.href === '/');
  const nextTheme = theme === 'light' ? 'dark' : 'light';

  const tools: AppSidebarGroup = {
    id: 'tools',
    ariaLabel: 'search and theme',
    items: [
      {
        id: 'search',
        label: 'search',
        icon: Search,
        title: 'search and jump (⌘K)',
        onSelect: () => window.dispatchEvent(new Event(OPEN_COMMAND_PALETTE_EVENT)),
      },
      {
        id: 'theme',
        label: `${nextTheme} mode`,
        icon: theme === 'light' ? Moon : Sun,
        title: `switch to ${nextTheme} mode`,
        onSelect: toggle,
      },
    ],
  };

  const groups: AppSidebarGroup[] = [
    {
      id: 'sections',
      ariaLabel: 'sections',
      items: [
        ...nav.map((entry) => ({ id: entry.id, label: entry.label, href: entry.href, icon: entry.icon, title: entry.title })),
        { id: 'collectives', label: 'collectives', href: onlineCollectivesHref(), icon: Users, external: true, target: '_blank', rel: 'noreferrer', title: 'collectives, opens online' },
      ],
    },
    tools,
  ];
  const underHome = UNDER_HOME.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
  const currentId = nav.find((entry) => isSectionActive(entry, pathname))?.id ?? (underHome ? home?.id : undefined);

  return (
    <AppSidebar
      app="local"
      apps={switcherApps()}
      groups={groups}
      currentId={currentId}
      homeHref="/"
      account={auth?.authenticated && auth.username ? { handle: auth.username, href: settingsSectionHref('account') } : undefined}
      LinkComponent={NextLink}
      storageKey="peasant.local-sidebar"
      defaultWidth={272}
    />
  );
}
