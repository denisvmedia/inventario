import { spawn, ChildProcess } from 'child_process';
import axios from 'axios';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

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

/**
 * Start the backend server
 */
export async function startBackend(): Promise<void> {
  console.log('Starting backend server...');
  console.log(`Working directory: ${projectRoot}`);

  // Check if main.go exists
  try {
    const { existsSync } = await import('fs');
    if (!existsSync(`${projectRoot}/main.go`)) {
      console.error(`Error: main.go not found in ${projectRoot}`);
      throw new Error(`main.go not found in ${projectRoot}`);
    }
    console.log(`Found main.go in ${projectRoot}`);
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

  console.log('Executing: go run main.go run');
  backendProcess = spawn('go', ['run', 'main.go', 'run'], {
    cwd: projectRoot,
    stdio: 'pipe',
    shell: true,
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
async function waitForFrontend(maxRetries = 60, retryInterval = 1000): Promise<void> {
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
 * Start the entire stack (backend + frontend)
 */
export async function startStack(): Promise<void> {
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
  console.log('Stopping all services...');

  if (backendProcess) {
    backendProcess.kill('SIGTERM');
    backendProcess = null;
  }

  if (frontendProcess) {
    frontendProcess.kill('SIGTERM');
    frontendProcess = null;
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
