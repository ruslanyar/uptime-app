import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';
import { createElement, type ComponentProps } from 'react';

afterEach(cleanup);

// Component tests run outside Next's configured image runtime. E2E exercises
// the real Image component and optimizer against the Go API.
vi.mock('next/image', () => ({
  default: (props: ComponentProps<'img'>) => createElement('img', props),
}));
