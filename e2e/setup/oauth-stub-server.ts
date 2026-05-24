/**
 * OAuth provider stub server for the #1394 e2e flow.
 *
 * Stands up a single Node http.Server that pretends to be Google's three
 * OAuth endpoints (/authorize, /token, /userinfo). The Inventario backend
 * is launched with the INVENTARIO_RUN_OAUTH_GOOGLE_AUTH_URL_OVERRIDE,
 * _TOKEN_URL_OVERRIDE and _USERINFO_URL_OVERRIDE env vars pointing at
 * this stub, so the BE's OAuth flow performs the full token exchange +
 * userinfo fetch against a local server with no outbound network calls
 * to Google.
 *
 * Flow:
 *
 *   1. /authorize — the BE 302s the browser here on /auth/oauth/google/start.
 *      The stub immediately 302s back to the BE's /callback path with a
 *      ?code=<stub-code>&state=<echo> pair so the test never has to render
 *      a real consent screen.
 *
 *   2. /token — the BE POSTs the code + code_verifier here on callback;
 *      the stub validates code_verifier is present and returns a JSON
 *      access token.
 *
 *   3. /userinfo — the BE fetches the OAuth profile with the access token;
 *      the stub returns a deterministic sub + email + name + verified
 *      flag controlled by setProfile().
 *
 * The default port is 4444 (fixed so tests can hard-code overrides in
 * setup-stack.ts). Override via OAUTH_STUB_PORT.
 *
 * NEVER ship this module in a production binary — it intentionally
 * forges authorization codes and verifies nothing. The Go bootstrap
 * layer emits a loud slog.Warn whenever the override env vars are set
 * so a misconfigured deployment is caught early.
 */
import { createServer, Server, IncomingMessage, ServerResponse } from 'node:http';
import { URL } from 'node:url';

export interface OAuthStubProfile {
  /** Stable subject identifier returned at /userinfo. */
  sub: string;
  /** Verified email — written verbatim into the BE user row. */
  email: string;
  /** Whether the BE should auto-link the email to an existing user. */
  emailVerified: boolean;
  /** Display name seeded into users.name on first sign-up. */
  name: string;
}

/**
 * The active profile served at /userinfo. setProfile() rewrites this in
 * place so a single long-lived stub can flip between fixtures across
 * sequential test cases without restarting.
 */
let activeProfile: OAuthStubProfile = {
  sub: 'stub-user-default',
  email: 'oauth-default@example.test',
  emailVerified: true,
  name: 'Default Stub User',
};

/**
 * Update the profile the next /userinfo call will return. Call this from
 * a test before clicking the OAuth button so the BE callback sees the
 * fixture the test wants to exercise.
 */
export function setProfile(profile: Partial<OAuthStubProfile>): void {
  activeProfile = { ...activeProfile, ...profile };
}

/** Reset the profile back to the module default. */
export function resetProfile(): void {
  activeProfile = {
    sub: 'stub-user-default',
    email: 'oauth-default@example.test',
    emailVerified: true,
    name: 'Default Stub User',
  };
}

let server: Server | null = null;

/**
 * Start the stub server on the configured port. Idempotent — calling
 * start() twice returns the same listening URL.
 *
 * @returns the public base URL the BE should be pointed at, e.g.
 *          http://localhost:4444. The /authorize path is the value to
 *          plug into OAUTH_GOOGLE_AUTH_URL_OVERRIDE; the /token path
 *          goes into OAUTH_GOOGLE_TOKEN_URL_OVERRIDE; /userinfo into
 *          OAUTH_GOOGLE_USERINFO_URL_OVERRIDE.
 */
export async function startOAuthStub(port = Number(process.env.OAUTH_STUB_PORT) || 4444): Promise<string> {
  if (server) {
    const address = server.address();
    if (address && typeof address === 'object') {
      return `http://127.0.0.1:${address.port}`;
    }
  }

  await new Promise<void>((resolve, reject) => {
    const srv = createServer(handleRequest);
    srv.once('error', reject);
    srv.listen(port, '127.0.0.1', () => {
      srv.removeListener('error', reject);
      server = srv;
      resolve();
    });
  });

  const address = server!.address();
  if (!address || typeof address !== 'object') {
    throw new Error('OAuth stub: listen() returned without an address');
  }
  const url = `http://127.0.0.1:${address.port}`;
  console.log(`[oauth-stub] listening at ${url}`);
  return url;
}

/** Stop the stub server. Safe to call when not running. */
export async function stopOAuthStub(): Promise<void> {
  if (!server) return;
  await new Promise<void>((resolve) => {
    server!.close(() => resolve());
  });
  server = null;
  console.log('[oauth-stub] stopped');
}

/**
 * The single request handler. Routes /authorize, /token, /userinfo and
 * returns 404 for anything else.
 */
function handleRequest(req: IncomingMessage, res: ServerResponse): void {
  // Best-effort error logging — the BE will surface a 400/500 if anything
  // here misbehaves, but logging the URL makes diagnostics painless.
  try {
    const url = new URL(req.url || '/', `http://${req.headers.host || '127.0.0.1'}`);
    if (url.pathname === '/authorize' && req.method === 'GET') {
      handleAuthorize(url, res);
      return;
    }
    if (url.pathname === '/token' && req.method === 'POST') {
      handleToken(req, res);
      return;
    }
    if (url.pathname === '/userinfo' && req.method === 'GET') {
      handleUserInfo(req, res);
      return;
    }
    // Control plane: tests POST a JSON Profile to /__control__/profile
    // before clicking the OAuth button so the next /userinfo call
    // returns the desired fixture. Playwright workers run in separate
    // node processes from this stub, so an in-memory setProfile() call
    // from a test wouldn't reach this module — the control endpoint
    // bridges that gap.
    if (url.pathname === '/__control__/profile' && req.method === 'POST') {
      handleSetProfile(req, res);
      return;
    }
    res.statusCode = 404;
    res.setHeader('content-type', 'text/plain');
    res.end(`stub-not-found: ${req.method} ${url.pathname}`);
  } catch (err) {
    console.error('[oauth-stub] handler error:', err);
    res.statusCode = 500;
    res.end('stub-error');
  }
}

/**
 * /authorize echoes ?state back through ?state on a 302 to the BE's
 * /callback path. The redirect_uri the BE sent in the auth-code URL is
 * the canonical callback URL — we just bounce there with a synthetic
 * ?code=stub-code so the BE's exchange step runs.
 */
function handleAuthorize(url: URL, res: ServerResponse): void {
  const state = url.searchParams.get('state') || '';
  const redirectURI = url.searchParams.get('redirect_uri') || '';
  if (!redirectURI) {
    res.statusCode = 400;
    res.end('stub: missing redirect_uri');
    return;
  }
  const callback = new URL(redirectURI);
  callback.searchParams.set('code', 'stub-authorization-code');
  callback.searchParams.set('state', state);
  res.statusCode = 302;
  res.setHeader('location', callback.toString());
  res.end();
}

/**
 * /token reads the form-encoded body the BE posts, asserts code_verifier
 * is present (PKCE), and returns a fixed access token. The BE doesn't
 * care about token expiry here; expires_in is included for completeness.
 */
function handleToken(req: IncomingMessage, res: ServerResponse): void {
  let body = '';
  req.on('data', (chunk) => {
    body += chunk.toString();
  });
  req.on('end', () => {
    const params = new URLSearchParams(body);
    const codeVerifier = params.get('code_verifier');
    if (!codeVerifier) {
      res.statusCode = 400;
      res.setHeader('content-type', 'application/json');
      res.end(JSON.stringify({ error: 'invalid_request', error_description: 'missing code_verifier' }));
      return;
    }
    res.statusCode = 200;
    res.setHeader('content-type', 'application/json');
    res.end(
      JSON.stringify({
        access_token: 'stub-access-token',
        token_type: 'Bearer',
        expires_in: 3600,
      })
    );
  });
}

/**
 * /__control__/profile receives a JSON Profile from the test harness and
 * updates the in-memory activeProfile so the NEXT /userinfo call sees
 * the fresh fixture. The endpoint replies with the merged profile so
 * the caller can confirm the update.
 */
function handleSetProfile(req: IncomingMessage, res: ServerResponse): void {
  let body = '';
  req.on('data', (chunk) => {
    body += chunk.toString();
  });
  req.on('end', () => {
    try {
      const partial = body ? JSON.parse(body) : {};
      activeProfile = { ...activeProfile, ...partial };
      res.statusCode = 200;
      res.setHeader('content-type', 'application/json');
      res.end(JSON.stringify(activeProfile));
    } catch (err) {
      res.statusCode = 400;
      res.setHeader('content-type', 'application/json');
      res.end(JSON.stringify({ error: 'bad-json', message: String(err) }));
    }
  });
}

/**
 * /userinfo verifies the Bearer token then returns the active profile.
 * The Google userinfo endpoint emits the same sub/email/email_verified/name
 * shape we ship here.
 */
function handleUserInfo(req: IncomingMessage, res: ServerResponse): void {
  const auth = req.headers.authorization || '';
  if (auth !== 'Bearer stub-access-token') {
    res.statusCode = 401;
    res.setHeader('content-type', 'application/json');
    res.end(JSON.stringify({ error: 'invalid_token' }));
    return;
  }
  res.statusCode = 200;
  res.setHeader('content-type', 'application/json');
  res.end(
    JSON.stringify({
      sub: activeProfile.sub,
      email: activeProfile.email,
      email_verified: activeProfile.emailVerified,
      name: activeProfile.name,
    })
  );
}
