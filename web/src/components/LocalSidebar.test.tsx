import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { render, screen, cleanup, fireEvent, within, act } from '@testing-library/react';
import {
  chromeFailures,
  loadShellChromeManifest,
  routePageFile,
  shippedItems,
} from '../../scripts/visual/shell-chrome-manifest.mjs';
import { GRAPH_APP_SECTIONS, GraphSectionNav, LOCAL_APP_SECTIONS } from '@peasant-labs/fairtrade/graph';
import { ROUTE_ONLY_SECTIONS } from '@/lib/nav/sections';
import { SETTINGS_SECTIONS } from '@/lib/settings/catalog';
import { SHELL_CHROME_CASES } from '@/test/fixtures/shellChromeCases';
import { OPEN_COMMAND_PALETTE_EVENT } from '@/components/command/CommandPalette';
import { LocalSidebar } from './LocalSidebar';

let currentPathname = '/';
let currentSearch = '';
vi.mock('next/navigation', () => ({
  usePathname: () => currentPathname,
  useSearchParams: () => new URLSearchParams(currentSearch),
}));

let theme: 'light' | 'dark' = 'light';
const toggle = vi.fn();
vi.mock('@/hooks/useTheme', () => ({ useTheme: () => ({ theme, toggle }) }));

// The sidebar reads the advertised capability set. The code-map token is
// advertised here on purpose: an experimental server must not bring a
// route-only section back into the sidebar.
vi.mock('@/contexts/ServerCapabilitiesContext', () => ({
  useServerCapabilities: () => ({ status: 'ready', capabilities: new Set(['code_map_navigation_v1']) }),
}));

const manifest = loadShellChromeManifest();
const shipped = shippedItems(manifest);
let signIn: unknown = { authenticated: false };

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async (input: string) => {
    const url = new URL(String(input), 'http://localhost');
    if (url.pathname === '/api/v1/sync/auth') return Response.json(signIn);
    throw new Error(`unexpected sidebar request ${url.pathname}`);
  }));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  currentPathname = '/';
  currentSearch = '';
  theme = 'light';
  signIn = { authenticated: false };
  toggle.mockClear();
});

async function mount(ui = <LocalSidebar />) {
  const view = render(ui);
  // Let the sign-in read settle.
  await act(async () => { await new Promise((done) => setTimeout(done, 0)); });
  return view;
}

describe('LocalSidebar — the local shell chrome manifest', () => {
  it.each(['light', 'dark'] as const)('holds every show, hide and route rule in the %s theme', async (mode) => {
    theme = mode;
    await mount();
    expect(chromeFailures(manifest, { theme: mode, shipped })).toEqual([]);
  });

  it.each(SHELL_CHROME_CASES.pages.map((page) => [page.name, page.pathname] as const))(
    'holds on the %s page (%s)',
    async (_name, pathname) => {
      currentPathname = pathname;
      await mount();
      expect(chromeFailures(manifest, { theme: 'light', shipped })).toEqual([]);
    },
  );

  it("reports fairtrade's own section sub-nav if it comes back, wherever it mounts", async () => {
    await mount(
      <>
        <LocalSidebar />
        <main>
          <GraphSectionNav sections={GRAPH_APP_SECTIONS} hrefFor={(section: { id: string }) => `/${section.id}`} />
        </main>
      </>,
    );
    expect(chromeFailures(manifest, { theme: 'light', shipped })).toContain(`section-nav: present (${manifest.hide['section-nav'].selector})`);
  });

  it('reports any link to a route-only section outside the page body, whatever its markup', async () => {
    await mount(
      <>
        <LocalSidebar />
        <nav aria-label="sections">
          <a href="/analytics">analytics</a>
          <a href="/review">changes</a>
        </nav>
        <main>
          {/* The page body may link into a route-only section (a code-map breadcrumb does). */}
          <a href="/map">map</a>
        </main>
      </>,
    );
    const failures = chromeFailures(manifest, { theme: 'light', shipped });
    expect(failures).toContain('route /analytics: the persistent chrome links to it');
    expect(failures).toContain('route /review: the persistent chrome links to it');
    expect(failures).not.toContain('route /map: the persistent chrome links to it');
  });

  it('lists the registry nav sections in its order, labelled as the registry labels them', async () => {
    await mount();
    const nav = screen.getByRole('navigation', { name: 'sections' });
    const labels = within(nav).getAllByRole('link').map((link) => link.querySelector('.asb-label')?.textContent);
    const registry = LOCAL_APP_SECTIONS.filter((section) => section.inNav !== false).map((section) => section.label);
    expect(labels.slice(0, registry.length)).toEqual(registry);
  });

  it('marks home current on Home, a project and a transcript, and nothing on a route-only page', async () => {
    for (const pathname of ['/', '/sessions/abc/', '/projects/abc/sess-1/']) {
      currentPathname = pathname;
      await mount();
      expect(document.querySelector('.asb-panel [aria-current="page"]')?.getAttribute('href')).toBe('/');
      cleanup();
    }
    currentPathname = '/analytics/';
    await mount();
    expect(document.querySelector('.asb-panel [aria-current="page"]')).toBeNull();
  });

  it('switches the theme from its item, named for the mode it switches to', async () => {
    await mount();
    const button = screen.getByRole('button', { name: 'dark mode' });
    expect(button).toHaveAttribute('title', 'switch to dark mode');
    fireEvent.click(button);
    expect(toggle).toHaveBeenCalledTimes(1);
  });

  it('opens the command palette from the search item', async () => {
    const opened = vi.fn();
    window.addEventListener(OPEN_COMMAND_PALETTE_EVENT, opened);
    await mount();
    fireEvent.click(screen.getByRole('button', { name: 'search' }));
    window.removeEventListener(OPEN_COMMAND_PALETTE_EVENT, opened);
    expect(opened).toHaveBeenCalledTimes(1);
  });

  it('links the collectives page online as leaving the app', async () => {
    await mount();
    const link = screen.getByRole('link', { name: /^collectives/ });
    expect(link.getAttribute('href')).toMatch(/^https:\/\/.+\/collectives$/);
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAccessibleName(/^collectives\s*, opens outside this app$/);
  });

  it('marks this app current in the switcher and opens online and the docs outside it', async () => {
    await mount();
    const apps = screen.getByRole('navigation', { name: 'apps' });
    expect(within(apps).getByText('local').closest('[aria-current]')).toHaveAttribute('aria-current', 'true');
    const online = within(apps).getByRole('link', { name: /^online/ });
    expect(online.getAttribute('href')).toMatch(/^https:\/\//);
    expect(online).toHaveAttribute('target', '_blank');
    expect(within(apps).getByRole('link', { name: /^docs/ })).toHaveAttribute('href', 'https://peasantlabs.org/docs');
  });

  it('shows the account row only for a signed-in computer', async () => {
    await mount();
    expect(document.querySelector('.asb-account')).toBeNull();
    cleanup();
    signIn = { authenticated: true, username: 'alice-dev' };
    await mount();
    expect(document.querySelector('.asb-account')).toHaveTextContent('@alice-dev');
    expect(document.querySelector('.asb-account')).toHaveAttribute('href', '/settings?section=account');
  });
});

describe('LocalSidebar — on a settings page', () => {
  it('lists the settings sections under a way back home, the one the address names current', async () => {
    currentPathname = '/settings/';
    currentSearch = 'section=redaction';
    await mount();
    expect(screen.getByRole('link', { name: 'home' })).toHaveAttribute('href', '/');
    const nav = screen.getByRole('navigation', { name: 'settings' });
    const links = within(nav).getAllByRole('link');
    expect(links.map((link) => link.textContent)).toEqual(SETTINGS_SECTIONS.map((section) => section.label));
    expect(links.map((link) => link.getAttribute('href'))).toEqual(SETTINGS_SECTIONS.map((section) => `/settings?section=${section.id}`));
    expect(within(nav).getByRole('link', { name: 'redaction' })).toHaveAttribute('aria-current', 'page');
    // Search and the theme stay reachable here too.
    expect(screen.getByRole('button', { name: 'search' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'dark mode' })).toBeInTheDocument();
  });

  it('marks the default section current when the address names none', async () => {
    currentPathname = '/settings/';
    await mount();
    expect(within(screen.getByRole('navigation', { name: 'settings' })).getByRole('link', { name: 'account' })).toHaveAttribute('aria-current', 'page');
  });
});

describe('route-only sections', () => {
  it('are exactly the manifest routes, and each still has its page', () => {
    expect(ROUTE_ONLY_SECTIONS.map((section) => section.href).sort()).toEqual(Object.keys(manifest.routes).sort());
    for (const path of Object.keys(manifest.routes)) {
      expect(routePageFile(path), `${path} must keep its page`).not.toBeNull();
    }
  });
});

describe('shell chrome manifest loader', () => {
  it('rejects a manifest that drops a required name', async () => {
    const { readFileSync } = await import('node:fs');
    const { SHELL_CHROME_FIXTURE } = await import('../../scripts/visual/shell-chrome-manifest.mjs');
    const source = readFileSync(SHELL_CHROME_FIXTURE, 'utf8');
    const withoutShare = source.replace(/  share-button:\n    selector: [^\n]+\n/, '');
    expect(withoutShare).not.toBe(source);
    expect(() => loadShellChromeManifest(withoutShare)).toThrow(/hide is missing required names: share-button/);
  });
});
