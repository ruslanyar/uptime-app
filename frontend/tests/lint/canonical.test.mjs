import assert from 'node:assert/strict';
import { test } from 'node:test';
import { ESLint } from 'eslint';

const rule = 'better-tailwindcss/enforce-canonical-classes';
const filePath = 'src/canonical-probe.tsx';
const examples = [
  {
    name: 'JSX pixel size',
    code: 'export const Example = () => <div className="h-[650px]" />;',
    expected: 'h-162.5',
  },
  {
    name: 'cn with responsive pixel size',
    code: "import { cn } from '@/lib/utils';\n\nexport const classes = cn('md:h-[650px]');",
    expected: 'md:h-162.5',
  },
  {
    name: 'cva variant with pixel size',
    code: "import { cva } from 'class-variance-authority';\n\nexport const variants = cva('flex', { variants: { size: { large: 'h-[650px]' } } });",
    expected: 'h-162.5',
  },
  {
    name: 'arbitrary background size',
    code: 'export const Example = () => <div className="bg-[size:64px_64px]" />;',
    expected: 'bg-size-[64px_64px]',
  },
];

test('project ESLint reports and fixes canonical classes, including pixel sizes', async () => {
  const eslint = new ESLint();
  const fixer = new ESLint({ fix: true });
  for (const { name, code, expected } of examples) {
    const [result] = await eslint.lintText(code, { filePath });
    const diagnostics = result.messages.filter(
      (message) => message.ruleId === rule,
    );
    assert.equal(diagnostics.length, 1, name);
    assert.equal(diagnostics[0].severity, 2, name);
    assert.ok(diagnostics[0].message.includes(expected), name);
    const [fixed] = await fixer.lintText(code, { filePath });
    assert.ok(fixed.output?.includes(expected), name);
    assert.equal(fixed.errorCount, 0, name);
    const [rechecked] = await eslint.lintText(fixed.output, { filePath });
    assert.equal(rechecked.errorCount, 0, name);
  }
});

test('canonical sizes and project theme classes pass without collapsing classes', async () => {
  const eslint = new ESLint({ fix: true });
  const code =
    'export const Example = () => <div className="h-162.5 md:h-162.5 bg-size-[64px_64px] bg-background text-primary text-sm leading-relaxed" />;';
  const [result] = await eslint.lintText(code, { filePath });
  assert.equal(result.errorCount, 0);
  assert.equal(result.output, undefined);
});
