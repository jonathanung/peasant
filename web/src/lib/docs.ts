import { DOCS_URL } from '@/lib/village/links';

/**
 * Where each how-to lives in the docs. Instructions live only there: the app
 * links to the page or section that teaches a task instead of restating its
 * commands or steps.
 */
export const DOCS = {
  /** `peasant ingest`: scan this computer for agent sessions (set up). */
  record: `${DOCS_URL}#kickstart`,
  /** `peasant kickstart`: choose which projects peasant records. */
  selection: `${DOCS_URL}#kickstart`,
  /** `peasant prune`: the only thing that removes recorded sessions (cli reference). */
  prune: `${DOCS_URL}#cli`,
  /** Automatic publishing: rules, `/peasant auto`, the git hook (publish). */
  autoPublish: `${DOCS_URL}#publish`,
  /** Signing in to peasant online from this computer (publish). */
  signIn: `${DOCS_URL}#publish`,
  /** config.yaml and hooks.yaml: where they live, and `peasant config` (cli reference). */
  settingsFiles: `${DOCS_URL}#cli`,
  /** Reading the code map (the docs overview, until it has its own section). */
  codeMap: DOCS_URL,
  /** Reading changes: lines of work and the sessions behind them (the pr reviews guide). */
  changes: `${DOCS_URL}/guides/reviews`,
} as const;

export type DocsTopic = keyof typeof DOCS;
