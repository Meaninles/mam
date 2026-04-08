// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { getRuntimeConfig } from './runtimeConfig';

declare global {
  interface Window {
    __MARE_RUNTIME_CONFIG__?: {
      centerBaseUrl?: string;
      runtimeStatusEnabled?: boolean;
      runtimeStatusPollMs?: number;
    };
  }
}

describe('runtimeConfig', () => {
  afterEach(() => {
    delete window.__MARE_RUNTIME_CONFIG__;
  });

  it('uses window overrides when provided', () => {
    window.__MARE_RUNTIME_CONFIG__ = {
      centerBaseUrl: 'http://127.0.0.1:18080/',
      runtimeStatusEnabled: true,
      runtimeStatusPollMs: 3000,
    };

    expect(getRuntimeConfig()).toEqual({
      centerBaseUrl: 'http://127.0.0.1:18080',
      runtimeStatusEnabled: true,
      runtimeStatusPollMs: 3000,
    });
  });

  it('disables runtime polling by default in test environment', () => {
    expect(getRuntimeConfig()).toEqual({
      centerBaseUrl: 'http://127.0.0.1:8080',
      runtimeStatusEnabled: false,
      runtimeStatusPollMs: 15000,
    });
  });
});
