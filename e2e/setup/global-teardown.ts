import { stopStack } from './setup-stack.js';
import { FullConfig } from '@playwright/test';

async function globalTeardown(config: FullConfig) {
  // Stop the stack unless explicitly told not to
  if (process.env.START_STACK !== 'false') {
    await stopStack();
  }
}

export default globalTeardown;
