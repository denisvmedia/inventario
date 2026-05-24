import { spawn, ChildProcess } from 'child_process';
import axios from 'axios';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';
import { BACKEND_URL, BASE_URL, USE_PREBUILT } from './urls.js';
import { startOAuthStub, stopOAuthStub } from './oauth-stub-server.js';

// Sleep utility
const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

// Process management
let backendProcess: ChildProcess | null = null;
let frontendProcess: ChildProcess | null = null;

// Path resolution for ES modules
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const projectRoot = resolve(__dirname, '../..');
const frontendRoot = resolve(projectRoot, 'frontend');
const backendRoot = resolve(projectRoot, 'go');

/**
 * Start the backend server
 */
/**
 * When OAUTH_STUB_ENABLED=true, start the OAuth stub server first and
 * return the env-var bundle that points the BE at it (Google provider
 * only — GitHub follow-up). The vars are merged into the BE spawn env
 * below so the bootstrap layer registers the provider with the
 * stub-server URLs.
 *
 * Returns an empty object when the stub is disabled, so the spread in
 * the backend's env block is a no-op.
 */
async function maybeStartOAuthStub(): Promise<Record<string, string>> {
  if (process.env.OAUTH_STUB_ENABLED !== 'true') {
    return {};
  }
  const stubURL = await startOAuthStub();
  return {
    INVENTARIO_RUN_OAUTH_GOOGLE_CLIENT_ID:
      process.env.INVENTARIO_RUN_OAUTH_GOOGLE_CLIENT_ID ?? 'stub-client-id',
    INVENTARIO_RUN_OAUTH_GOOGLE_CLIENT_SECRET:
      process.env.INVENTARIO_RUN_OAUTH_GOOGLE_CLIENT_SECRET ?? 'stub-client-secret',
    // Pin the redirect base at the FE-facing URL (Vite on :5173 in dev
    // mode; backend on :3333 when USE_PREBUILT=true) so the BE's final
    // 302 to "/" lands on the same origin the browser started at. This
    // matters because the FE auth cookie / localStorage are scoped per
    // origin — bouncing across :5173 → :3333 mid-flow would drop the
    // session state on the floor.
    INVENTARIO_RUN_OAUTH_REDIRECT_BASE_URL:
      process.env.INVENTARIO_RUN_OAUTH_REDIRECT_BASE_URL ?? BASE_URL,
    // 32-byte fixed key so the BE doesn't generate a random one and
    // print it; the test never needs to verify states across restarts.
    INVENTARIO_RUN_OAUTH_STATE_KEY:
      process.env.INVENTARIO_RUN_OAUTH_STATE_KEY ?? 'e2e-stub-oauth-state-key-32-bytes!',
    INVENTARIO_RUN_OAUTH_GOOGLE_AUTH_URL_OVERRIDE: `${stubURL}/authorize`,
    INVENTARIO_RUN_OAUTH_GOOGLE_TOKEN_URL_OVERRIDE: `${stubURL}/token`,
    INVENTARIO_RUN_OAUTH_GOOGLE_USERINFO_URL_OVERRIDE: `${stubURL}/userinfo`,
  };
}

export async function startBackend(): Promise<void> {
  console.log('Starting backend server...');
  console.log(`Working directory: ${projectRoot}`);

  // Check if main.go exists
  try {
    const { existsSync } = await import('fs');
    if (!existsSync(`${backendRoot}/main.go`)) {
      console.error(`Error: main.go not found in ${backendRoot}`);
      throw new Error(`main.go not found in ${backendRoot}`);
    }
    console.log(`Found main.go in ${backendRoot}`);
  } catch (error) {
    console.error('Error checking for main.go:', error);
    throw error;
  }

  // create uploads directory if it doesn't exist
  try {
    const { mkdir } = await import('fs/promises');
    await mkdir(`${projectRoot}/uploads`, { recursive: true });
    console.log(`Created uploads directory in ${projectRoot}`);
  } catch (error) {
    console.error('Error creating uploads directory:', error);
    throw error;
  }

  // Build frontend for embedding
  console.log('Building frontend for embedding...')
  try {
    const { execSync } = await import('child_process');
    execSync('npm install', { cwd: frontendRoot });
    execSync('npm run build', { cwd: frontendRoot });
    console.log('Frontend built successfully');
  } catch (error) {
    console.error('Error building frontend:', error);
    throw error;
  }

  console.log('Downloading go modules')
  try {
    const { execSync } = await import('child_process');
    execSync('go mod download', { cwd: backendRoot });
    console.log('Downloaded go modules');
  } catch (error) {
    console.error('Error downloading go modules:', error);
    throw error;
  }

  // Optionally start the OAuth stub server before the BE so the override
  // env vars are baked into the spawn env. The stub is a no-op when
  // OAUTH_STUB_ENABLED!=='true', so callers that don't exercise OAuth
  // (the default) see no behaviour change.
  const oauthStubEnv = await maybeStartOAuthStub();

  console.log('Executing: go run -tags with_frontend ./cmd/inventario/... run');
  backendProcess = spawn('go', ['run', '-tags', 'with_frontend', './cmd/inventario/...', 'run'], {
    cwd: backendRoot,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: {
      ...process.env,
      ...oauthStubEnv,
      PATH: process.env.PATH,
      // E2E runs are intentionally high-throughput and parallelized.
      // Keep auth/global rate limiting disabled unless explicitly overridden.
      INVENTARIO_RUN_AUTH_RATE_LIMIT_DISABLED: process.env.INVENTARIO_RUN_AUTH_RATE_LIMIT_DISABLED ?? 'true',
      INVENTARIO_RUN_GLOBAL_RATE_LIMIT_DISABLED: process.env.INVENTARIO_RUN_GLOBAL_RATE_LIMIT_DISABLED ?? 'true',
      // The shared admin user piles up many groups across parallel test
      // workers (#1388 caps a real user at 3); set 0 to disable the
      // per-user membership cap for the duration of the e2e run.
      INVENTARIO_RUN_MAX_GROUP_MEMBERSHIPS_PER_USER: process.env.INVENTARIO_RUN_MAX_GROUP_MEMBERSHIPS_PER_USER ?? '0',
      // Memory-mode `seedMemoryDBDefaultTenant` defaults to slug
      // "default", but the seed code only provisions the orphan test
      // user when tenant.Slug == "test-org" (the postgres CI lane gets
      // that slug from the bootstrap migration). Pin the slug to
      // test-org so the no-group-redirect specs find their fixture.
      INVENTARIO_RUN_MEMORY_TENANT_SLUG: process.env.INVENTARIO_RUN_MEMORY_TENANT_SLUG ?? 'test-org',
      INVENTARIO_RUN_MEMORY_TENANT_NAME: process.env.INVENTARIO_RUN_MEMORY_TENANT_NAME ?? 'Test Organization',
      // Feedback (#1387) needs a destination address or the handler
      // 503s. The stub email provider doesn't actually deliver anything;
      // this value just satisfies the "configured?" check so the e2e
      // flow can exercise the happy path.
      INVENTARIO_RUN_SUPPORT_EMAIL: process.env.INVENTARIO_RUN_SUPPORT_EMAIL ?? 'support@e2e.test',
      // Opt the seed into the system-admin fixture (sysadmin@test-org.com)
      // so the admin-section e2e suite (#1758) can authenticate as a
      // platform admin. OFF by default in the binary — the /api/v1/seed
      // endpoint is unauthenticated, so this must never be set in a real
      // deployment.
      INVENTARIO_SEED_SYSTEM_ADMIN_FIXTURE:
        process.env.INVENTARIO_SEED_SYSTEM_ADMIN_FIXTURE ?? 'true',
      // Currency-migration surface defaults on in the binary now
      // (Config.FeatureCurrencyMigration env-default since #1612). Pass
      // an explicit override through only if the operator wants to flip
      // the kill-switch off for a one-off run.
      ...(process.env.INVENTARIO_RUN_FEATURE_CURRENCY_MIGRATION !== undefined
        ? {
            INVENTARIO_RUN_FEATURE_CURRENCY_MIGRATION:
              process.env.INVENTARIO_RUN_FEATURE_CURRENCY_MIGRATION,
          }
        : {}),
    },
  });

  // Handle process output
  backendProcess.stdout?.on('data', (data) => {
    console.log(`Backend: ${data.toString().trim()}`);
  });

  backendProcess.stderr?.on('data', (data) => {
    console.error(`Backend error: ${data.toString().trim()}`);
  });

  backendProcess.on('error', (error) => {
    console.error(`Failed to start backend: ${error.message}`);
    throw error;
  });

  backendProcess.on('exit', (code, signal) => {
    if (code !== null) {
      console.log(`Backend process exited with code ${code}`);
    } else if (signal !== null) {
      console.log(`Backend process killed with signal ${signal}`);
    }
  });

  // Wait for backend to be ready
  await waitForBackend();
  console.log('Backend server is ready');
}

/**
 * Wait for the backend to be available
 */
async function waitForBackend(maxRetries = 60, retryInterval = 1000): Promise<void> {
  let retries = 0;

  console.log(`Waiting for backend to be available at ${BACKEND_URL}`);

  while (retries < maxRetries) {
    try {
      console.log(`Attempt ${retries + 1}/${maxRetries} to connect to backend...`);
      const response = await axios.get(BACKEND_URL, { timeout: 5000 });
      if (response.status === 200) {
        console.log('Successfully connected to backend!');
        return;
      }
      console.log(`Received status ${response.status}, waiting for 200...`);
    } catch (error) {
      if (axios.isAxiosError(error)) {
        console.log(`Connection attempt failed: ${error.message}`);
      } else {
        console.log(`Unknown error: ${error}`);
      }

      // Retry after delay
      console.log(`Waiting ${retryInterval}ms before next attempt...`);
      await sleep(retryInterval);
      retries++;

      if (retries === maxRetries) {
        throw new Error('Backend server failed to start within the expected time');
      }
    }
  }
}

/**
 * Seed the database with test data
 */
export async function seedDatabase(): Promise<void> {
  console.log('Seeding database...');

  // Give the backend a moment to fully initialize
  await sleep(500);

  try {
    // Seed the database (endpoint is public for e2e testing)
    const response = await axios.post(`${BACKEND_URL}/api/v1/seed`);

    if (response.status === 200) {
      console.log('Database seeded successfully');
    } else {
      throw new Error(`Failed to seed database: ${response.statusText}`);
    }
  } catch (error) {
    console.error('Error seeding database:', error);
    throw error;
  }
}

/**
 * Start the frontend server
 */
export async function startFrontend(): Promise<void> {
  console.log('Starting frontend server...');
  console.log(`Frontend directory: ${frontendRoot}`);

  // Check if package.json exists in frontend directory
  try {
    const { existsSync } = await import('fs');
    if (!existsSync(`${frontendRoot}/package.json`)) {
      console.error(`Error: package.json not found in ${frontendRoot}`);
      throw new Error(`package.json not found in ${frontendRoot}`);
    }
    console.log(`Found package.json in ${frontendRoot}`);
  } catch (error) {
    console.error('Error checking for package.json:', error);
    throw error;
  }

  console.log('Executing: npm run dev');
  frontendProcess = spawn('npm', ['run', 'dev'], {
    cwd: frontendRoot,
    stdio: 'pipe',
    shell: true,
  });

  // Handle process output
  frontendProcess.stdout?.on('data', (data) => {
    console.log(`Frontend: ${data.toString().trim()}`);
  });

  frontendProcess.stderr?.on('data', (data) => {
    console.error(`Frontend error: ${data.toString().trim()}`);
  });

  frontendProcess.on('error', (error) => {
    console.error(`Failed to start frontend: ${error.message}`);
    throw error;
  });

  frontendProcess.on('exit', (code, signal) => {
    if (code !== null) {
      console.log(`Frontend process exited with code ${code}`);
    } else if (signal !== null) {
      console.log(`Frontend process killed with signal ${signal}`);
    }
  });

  // Wait for frontend to be ready
  await waitForFrontend();
  console.log('Frontend server is ready');
}

/**
 * Wait for the frontend to be available
 */
async function waitForFrontend(maxRetries = 120, retryInterval = 1000): Promise<void> {
  let retries = 0;

  console.log(`Waiting for frontend to be available at ${BASE_URL}`);

  while (retries < maxRetries) {
    try {
      console.log(`Attempt ${retries + 1}/${maxRetries} to connect to frontend...`);
      const response = await axios.get(BASE_URL, { timeout: 5000 });
      if (response.status === 200) {
        console.log('Successfully connected to frontend!');
        return;
      }
      console.log(`Received status ${response.status}, waiting for 200...`);
    } catch (error) {
      if (axios.isAxiosError(error)) {
        console.log(`Connection attempt failed: ${error.message}`);
      } else {
        console.log(`Unknown error: ${error}`);
      }

      // Retry after delay
      console.log(`Waiting ${retryInterval}ms before next attempt...`);
      await sleep(retryInterval);
      retries++;

      if (retries === maxRetries) {
        throw new Error('Frontend server failed to start within the expected time');
      }
    }
  }
}

/**
 * Start the entire stack (backend + frontend)
 */
export async function startStack(): Promise<void> {
  if (USE_PREBUILT) {
    // Seeding is done inside the inventario-init-data container (SEED_DATABASE=true).
    // Re-posting /api/v1/seed here would duplicate non-idempotent fixtures.
    console.log(`USE_PREBUILT=true — waiting for pre-started stack at ${BASE_URL}`);
    await waitForBackend();
    await waitForFrontend();
    return;
  }

  try {
    await startBackend();
    await seedDatabase();
    await startFrontend();
  } catch (error) {
    await stopStack();
    throw error;
  }
}

/**
 * Stop all running processes
 */
export async function stopStack(): Promise<void> {
  if (USE_PREBUILT) {
    // Stack is externally managed (docker-compose down is the caller's job).
    return;
  }

  console.log('Stopping all services...');

  if (backendProcess) {
    backendProcess.kill('SIGTERM');
    backendProcess = null;
  }

  if (frontendProcess) {
    frontendProcess.kill('SIGTERM');
    frontendProcess = null;
  }

  // Stop the OAuth stub server if it's up. Safe to call when disabled.
  try {
    await stopOAuthStub();
  } catch (err) {
    console.warn('Failed to stop OAuth stub:', err);
  }

  console.log('All services stopped');
}

// Handle unexpected shutdowns
process.on('exit', () => {
  stopStack();
});

process.on('SIGINT', () => {
  stopStack();
  process.exit(0);
});

process.on('SIGTERM', () => {
  stopStack();
  process.exit(0);
});

process.on('uncaughtException', (error) => {
  console.error('Uncaught exception:', error);
  stopStack();
  process.exit(1);
});
