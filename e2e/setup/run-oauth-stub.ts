/**
 * Run the OAuth stub on its own, without the rest of the stack.
 *
 * The CI OAuth lane needs the stub listening before the backend starts,
 * because the backend reads the endpoint overrides at boot. Playwright's
 * global setup runs after the stack is up, so it cannot be the thing that
 * starts it there — this entry point is.
 *
 *   OAUTH_STUB_HOST=0.0.0.0 npm run oauth-stub
 *
 * The spec configures the active profile over HTTP
 * (POST /__control__/profile), so nothing needs to share a process with it.
 */
import { startOAuthStub, stopOAuthStub } from "./oauth-stub-server.js";

async function main() {
  const url = await startOAuthStub();
  console.log(`OAuth stub is running at ${url}. Press Ctrl+C to stop.`);

  process.stdin.resume();

  for (const signal of ["SIGINT", "SIGTERM"] as const) {
    process.on(signal, async () => {
      await stopOAuthStub();
      process.exit(0);
    });
  }
}

main().catch(async (error) => {
  console.error("Failed to start the OAuth stub:", error);
  await stopOAuthStub();
  process.exit(1);
});
