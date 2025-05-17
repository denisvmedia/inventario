import { stopStack } from './setup-stack.js';
import { FullConfig } from '@playwright/test';

async function globalTeardown(config: FullConfig) {
  // Only stop the stack if we started it
  if (process.env.START_STACK === 'true') {
    await stopStack();
  }
}

export default globalTeardown;
