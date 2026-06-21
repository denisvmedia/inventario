---
title: Self-hosting
description: Run your own Inventario instance with Docker Compose, plus pointers to the operator runbooks.
---

Inventario is open source, so you can run it on your own machine or server and keep your inventory entirely under your control. This page gets you started and points you to the deeper operator guides.

:::note[Some technical comfort needed]
Self-hosting means running and maintaining the app yourself. You'll be comfortable with a terminal, Docker, and editing a config file. If that's not for you, an instance someone else runs works exactly the same once you're logged in — see [Getting started](../getting-started/).
:::

## Quick start with Docker Compose

The fastest way to try Inventario on your own machine is Docker Compose. You'll need [Docker](https://docs.docker.com/get-docker/) with Docker Compose, plus Git to clone the repository.

1. Clone the repository:

   ```bash
   git clone https://github.com/denisvmedia/inventario.git
   cd inventario
   ```

2. Start everything:

   ```bash
   docker-compose up -d
   ```

   The first start takes a few minutes while it builds the image, sets up PostgreSQL, runs migrations, and creates a default admin user.

3. Open `http://localhost:3333` and sign in with the default credentials (`admin@example.com` / `Admin123`).

The full walkthrough — configuration variables, where your data lives, upgrades, and troubleshooting — is in the repository's [QUICKSTART.md](https://github.com/denisvmedia/inventario/blob/master/QUICKSTART.md).

:::caution[Change the defaults before going live]
The out-of-the-box setup is for local trials only. Before exposing Inventario to a network, set your own `JWT_SECRET` and `FILE_SIGNING_KEY`, change the admin password, and put HTTPS in front of it. The quick start covers each step.
:::

## Going to production

Once you've outgrown the quick start, pick the runbook that matches how you want to run Inventario. These are the authoritative operator guides — this page only orients you toward them.

- **Docker Compose, in depth** — environment variables, data persistence, external PostgreSQL, cloud storage, and monitoring: [DOCKER.md](https://github.com/denisvmedia/inventario/blob/master/DOCKER.md).
- **Bare-metal / systemd** — running the binary directly with your own PostgreSQL, secrets, email, and Redis: [DEPLOYMENT.md](https://github.com/denisvmedia/inventario/blob/master/DEPLOYMENT.md).
- **Kubernetes / Helm release runbook** — cutting a release and deploying to a cluster, with upgrade and rollback steps: [PRODUCTION.md](https://github.com/denisvmedia/inventario/blob/master/PRODUCTION.md).

:::tip[Back up your data]
Whichever route you choose, set up regular backups of your database and uploaded files early. Inventario also has its own portable export format — see [Backup & restore](../backup-and-restore/) for the in-app side of this.
:::

## Where to next

With your instance running, the rest of this guide applies just the same as a hosted one:

- [Getting started](../getting-started/) — your first sign-in and a tour of the app
- [Items](../items/) — add and organize what you own
- [Settings & account](../settings-and-account/) — currency, profile, and preferences

Found a bug or have a question? Open an issue on the [GitHub repository](https://github.com/denisvmedia/inventario/issues).
