import { stopStack } from './setup-stack.js';
import { FullConfig } from '@playwright/test';

async function globalTeardown(config: FullConfig) {
  console.log('Cleaning up e2e test environment...');

  // Always stop the stack since we always start it in global setup
  await stopStack();

  console.log('E2E test environment cleanup completed');
}

export default globalTeardown;
