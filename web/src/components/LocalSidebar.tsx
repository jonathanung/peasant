'use client';

import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Users } from 'lucide-react';
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
  return [
    ...APP_SIDEBAR_APPS.map((entry) => {
      const href = urls[entry.id];
      return href ? { ...entry, href } : entry;
    }),
    // Shown, not linked, until a community space exists.
    { id: 'community', label: 'community', tag: 'soon' },
  ];
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
 * in-page section bar of the settings page itself. The sidebar owns its own
 * order, the same in every app: the nav, then search (the command palette,
 * ⌘K), then the app switcher (the docs, and community marked soon), then the
 * footer with the theme toggle and the account. The collectives link opens
 * online.
 */
export function LocalSidebar() {
  const pathname = usePathname() ?? '/';
  const { theme, setTheme } = useTheme();
  const { capabilities } = useServerCapabilities();
  const auth = useSignIn();
  const nav = visibleNavSections(capabilities);
  const home = nav.find((entry) => entry.href === '/');

  const groups: AppSidebarGroup[] = [
    {
      id: 'sections',
      ariaLabel: 'sections',
      items: [
        ...nav.map((entry) => ({ id: entry.id, label: entry.label, href: entry.href, icon: entry.icon, title: entry.title })),
        { id: 'collectives', label: 'collectives', href: onlineCollectivesHref(), icon: Users, external: true, target: '_blank', rel: 'noreferrer', title: 'collectives, opens online' },
      ],
    },
  ];
  const underHome = UNDER_HOME.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
  const currentId = nav.find((entry) => isSectionActive(entry, pathname))?.id ?? (underHome ? home?.id : undefined);

  return (
    <AppSidebar
      app="local"
      apps={switcherApps()}
      groups={groups}
      currentId={currentId}
      search={{ onOpen: () => window.dispatchEvent(new Event(OPEN_COMMAND_PALETTE_EVENT)), shortcut: '⌘K', keyShortcuts: 'Meta+K Control+K' }}
      theme={theme}
      onThemeChange={setTheme}
      homeHref="/"
      account={auth?.authenticated && auth.username ? { handle: auth.username, href: settingsSectionHref('account') } : undefined}
      LinkComponent={NextLink}
      storageKey="peasant.local-sidebar"
      defaultWidth={272}
    />
  );
}
