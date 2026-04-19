import { startStack } from './setup-stack.js';
import { FullConfig } from '@playwright/test';
import waitOn from 'wait-on';
import { BASE_URL } from './urls.js';

async function globalSetup(config: FullConfig) {
  if (process.env.START_STACK === 'true') {
    await startStack();
    return;
  }

  // If stack startup is explicitly disabled, wait for an externally-managed server.
  await waitOn({
    resources: [BASE_URL],
    delay: 100, // minimum delay before starting (ms)
    interval: 250, // interval between attempts
    timeout: 30000, // maximum wait time (ms)
    tcpTimeout: 1000, // timeout for a single TCP connection
    window: 1000 // how many successful checks in a row are required
  });
}

export default globalSetup;
