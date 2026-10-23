/** The peasant online address, when the server does not name one. */
export function defaultOnlineUrl(): string {
  return process.env.NEXT_PUBLIC_COMMONS_URL ?? 'https://village.peasantlabs.org';
}

/** Where peasant online lists the collectives a reader can create or join. */
const ONLINE_COLLECTIVES_PATH = '/groups';

/** The collectives page on the peasant online this computer signs in to. */
export function onlineCollectivesHref(onlineUrl?: string): string {
  return `${onlineUrl || defaultOnlineUrl()}${ONLINE_COLLECTIVES_PATH}`;
}
