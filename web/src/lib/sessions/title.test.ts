import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { parseStrictYAML, requireExactRequiredFields, requireRecord, requireUniqueNames } from '@/test/strictYaml';
import { cleanSessionTitle } from './title';

type Case = { name: string; title: string; expected: string };

const root = requireRecord(
  parseStrictYAML(readFileSync(resolve(process.cwd(), 'src/lib/sessions/testdata/session_title.yaml'), 'utf8'), 'session title fixture'),
  'session title fixture',
);
requireExactRequiredFields(root, ['requiredNames', 'cases'], 'session title fixture');
const cases = (root.cases as unknown[]).map((row, index) => requireRecord(row, `session title cases[${index}]`));
requireUniqueNames(cases, 'session title cases');
cases.forEach((row, index) => requireExactRequiredFields(row, ['name', 'title', 'expected'], `session title cases[${index}]`));

describe('cleanSessionTitle', () => {
  it('holds exactly the required cases', () => {
    expect(cases.map((row) => row.name).sort()).toEqual([...(root.requiredNames as string[])].sort());
  });
  for (const row of cases as Case[]) {
    it(row.name, () => {
      const shown = cleanSessionTitle(row.title);
      expect(shown).toBe(row.expected);
      expect(shown).not.toMatch(/<\/?user_query>/);
    });
  }
});
