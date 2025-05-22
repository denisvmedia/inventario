import { startStack } from './setup-stack.js';
import { FullConfig } from '@playwright/test';
import waitOn from 'wait-on';

async function globalSetup(config: FullConfig) {
  await waitOn({
    resources: ['http://localhost:5173'],
    delay: 100, // минимальная задержка перед стартом (мс)
    interval: 250, // интервал между попытками
    timeout: 30000, // сколько максимум ждать (мс)
    tcpTimeout: 1000, // timeout на один tcp-коннект
    window: 1000 // сколько должно пройти успешных проверок подряд
  });

  // Only start the stack if it's not already running
  // This is useful for local development where you might want to start the stack manually
  if (process.env.START_STACK === 'true') {
    await startStack();
  }
}

export default globalSetup;
