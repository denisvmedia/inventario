import { spawn, ChildProcess, execSync } from 'child_process';
import axios from 'axios';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

// Sleep utility
const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

// Process management
let backendProcess: ChildProcess | null = null;
let frontendProcess: ChildProcess | null = null;
let postgresContainerName: string | null = null;

// Path resolution for ES modules
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const projectRoot = resolve(__dirname, '../..');
const frontendRoot = resolve(projectRoot, 'frontend');
const backendRoot = resolve(projectRoot, 'go');

// Database configuration
const POSTGRES_DB = process.env.E2E_POSTGRES_DB || 'inventario_e2e';
const POSTGRES_USER = process.env.E2E_POSTGRES_USER || 'inventario_e2e';
const POSTGRES_PASSWORD = process.env.E2E_POSTGRES_PASSWORD || 'inventario_e2e_password';
const POSTGRES_PORT = process.env.E2E_POSTGRES_PORT || '5433'; // Different port to avoid conflicts
const POSTGRES_DSN = `postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable`;

// Check if running in CI environment (GitHub Actions)
const isCI = process.env.CI === 'true' || process.env.GITHUB_ACTIONS === 'true';

/**
 * Start PostgreSQL container for e2e tests (local development only)
 */
export async function startPostgres(): Promise<void> {
  // In CI environment, PostgreSQL service is already running
  if (isCI) {
    console.log('Running in CI environment, using existing PostgreSQL service...');
    await waitForPostgres();
    return;
  }

  console.log('Starting PostgreSQL container for e2e tests...');

  // Generate unique container name
  const timestamp = Date.now();
  postgresContainerName = `inventario-e2e-postgres-${timestamp}`;

  try {
    // Check if Docker is available
    execSync('docker --version', { stdio: 'ignore' });
  } catch (error) {
    throw new Error('Docker is not available. Please install Docker to run e2e tests with PostgreSQL.');
  }

  try {
    // Start PostgreSQL container
    const dockerCommand = [
      'docker', 'run', '-d',
      '--name', postgresContainerName,
      '-e', `POSTGRES_DB=${POSTGRES_DB}`,
      '-e', `POSTGRES_USER=${POSTGRES_USER}`,
      '-e', `POSTGRES_PASSWORD=${POSTGRES_PASSWORD}`,
      '-e', 'POSTGRES_INITDB_ARGS=--encoding=UTF8 --lc-collate=C --lc-ctype=C',
      '-p', `${POSTGRES_PORT}:5432`,
      'postgres:17-alpine'
    ];

    console.log(`Executing: ${dockerCommand.join(' ')}`);
    execSync(dockerCommand.join(' '), { stdio: 'inherit' });

    console.log(`PostgreSQL container ${postgresContainerName} started`);

    // Wait for PostgreSQL to be ready
    await waitForPostgres();
    console.log('PostgreSQL is ready for connections');

  } catch (error) {
    console.error('Failed to start PostgreSQL container:', error);
    await stopPostgres(); // Cleanup on failure
    throw error;
  }
}

/**
 * Wait for PostgreSQL to be ready
 */
async function waitForPostgres(maxRetries = 60, retryInterval = 1000): Promise<void> {
  let retries = 0;

  console.log(`Waiting for PostgreSQL to be ready at localhost:${POSTGRES_PORT}`);

  while (retries < maxRetries) {
    try {
      console.log(`Attempt ${retries + 1}/${maxRetries} to connect to PostgreSQL...`);

      if (isCI || !postgresContainerName) {
        // In CI or when using external PostgreSQL, use psql directly
        const checkCommand = `psql "${POSTGRES_DSN}" -c "SELECT 1;" > /dev/null 2>&1`;
        execSync(checkCommand, { stdio: 'ignore' });
      } else {
        // Use docker exec to check if PostgreSQL is ready
        const checkCommand = [
          'docker', 'exec', postgresContainerName,
          'pg_isready', '-U', POSTGRES_USER, '-d', POSTGRES_DB
        ];
        execSync(checkCommand.join(' '), { stdio: 'ignore' });
      }

      console.log('Successfully connected to PostgreSQL!');
      return;

    } catch (error) {
      console.log(`PostgreSQL not ready yet, waiting ${retryInterval}ms before next attempt...`);
      await sleep(retryInterval);
      retries++;

      if (retries === maxRetries) {
        throw new Error('PostgreSQL failed to start within the expected time');
      }
    }
  }
}

/**
 * Stop PostgreSQL container (local development only)
 */
export async function stopPostgres(): Promise<void> {
  // In CI environment, PostgreSQL service is managed by GitHub Actions
  if (isCI) {
    console.log('Running in CI environment, PostgreSQL service will be cleaned up automatically');
    return;
  }

  if (!postgresContainerName) {
    return;
  }

  console.log(`Stopping PostgreSQL container ${postgresContainerName}...`);

  try {
    // Stop and remove the container
    execSync(`docker stop ${postgresContainerName}`, { stdio: 'ignore' });
    execSync(`docker rm ${postgresContainerName}`, { stdio: 'ignore' });
    console.log(`PostgreSQL container ${postgresContainerName} stopped and removed`);
  } catch (error) {
    console.error(`Failed to stop PostgreSQL container: ${error}`);
  } finally {
    postgresContainerName = null;
  }
}

/**
 * Reset the database for clean test state
 */
export async function resetDatabase(): Promise<void> {
  console.log('Resetting database for clean test state...');

  try {
    if (isCI || !postgresContainerName) {
      // In CI or when using external PostgreSQL, use psql directly
      const postgresMainDSN = `postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/postgres?sslmode=disable`;

      const dropCommand = `psql "${postgresMainDSN}" -c "DROP DATABASE IF EXISTS ${POSTGRES_DB};"`;
      const createCommand = `psql "${postgresMainDSN}" -c "CREATE DATABASE ${POSTGRES_DB};"`;

      execSync(dropCommand, { stdio: 'ignore' });
      execSync(createCommand, { stdio: 'ignore' });
    } else {
      // Use docker exec for local container
      const dropCommand = [
        'docker', 'exec', postgresContainerName,
        'psql', '-U', POSTGRES_USER, '-d', 'postgres',
        '-c', `DROP DATABASE IF EXISTS ${POSTGRES_DB};`
      ];

      const createCommand = [
        'docker', 'exec', postgresContainerName,
        'psql', '-U', POSTGRES_USER, '-d', 'postgres',
        '-c', `CREATE DATABASE ${POSTGRES_DB};`
      ];

      execSync(dropCommand.join(' '), { stdio: 'ignore' });
      execSync(createCommand.join(' '), { stdio: 'ignore' });
    }

    console.log('Database reset completed');
  } catch (error) {
    console.error('Failed to reset database:', error);
    throw error;
  }
}

/**
 * Start the backend server
 */
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

  console.log(`Executing: go run -tags with_frontend main.go run --db-dsn="${POSTGRES_DSN}"`);
  backendProcess = spawn('go', [
    'run', '-tags', 'with_frontend', 'main.go', 'run',
    '--db-dsn', POSTGRES_DSN
  ], {
    cwd: backendRoot,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env, PATH: process.env.PATH },
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

  console.log('Waiting for backend to be available at http://localhost:3333');

  while (retries < maxRetries) {
    try {
      console.log(`Attempt ${retries + 1}/${maxRetries} to connect to backend...`);
      const response = await axios.get('http://localhost:3333', { timeout: 5000 });
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
    const response = await axios.post('http://localhost:3333/api/v1/seed');
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

  console.log('Waiting for frontend to be available at http://localhost:5173');

  while (retries < maxRetries) {
    try {
      console.log(`Attempt ${retries + 1}/${maxRetries} to connect to frontend...`);
      const response = await axios.get('http://localhost:5173', { timeout: 5000 });
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
 * Start the entire stack (postgres + backend + frontend)
 */
export async function startStack(): Promise<void> {
  try {
    await startPostgres();
    await startBackend();
    await seedDatabase();
    await startFrontend();
  } catch (error) {
    await stopStack();
    throw error;
  }
}

/**
 * Stop all running processes and containers
 */
export async function stopStack(): Promise<void> {
  console.log('Stopping all services...');

  if (backendProcess) {
    backendProcess.kill('SIGTERM');
    backendProcess = null;
  }

  if (frontendProcess) {
    frontendProcess.kill('SIGTERM');
    frontendProcess = null;
  }

  await stopPostgres();

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
