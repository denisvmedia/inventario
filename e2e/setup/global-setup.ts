import { startStack } from './setup-stack.js';
import { FullConfig } from '@playwright/test';

async function globalSetup(config: FullConfig) {
  // Only start the stack if it's not already running
  // This is useful for local development where you might want to start the stack manually
  if (process.env.START_STACK === 'true') {
    await startStack();
  }
}

export default globalSetup;
