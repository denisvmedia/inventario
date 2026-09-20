# Email deliverability runbook

Inventario sends four kinds of mail that a user is waiting for: address
verification, password reset, group invitations, and warranty or maintenance
reminders. The first three gate access to the account. A verification mail in
the spam folder is not a delivery problem the user reports — it is a user who
quietly gives up.

This runbook covers getting that mail into inboxes. It is about DNS and the
sending domain, not about Inventario's configuration; for the settings
themselves see [PRODUCTION.md](../PRODUCTION.md) §B5 (Kubernetes) or
[DOCKER.md](../DOCKER.md) (Compose).

## Before anything else: are you sending at all?

The default email provider is a **stub that silently drops every message**. It
exists so a development instance does not need an SMTP server, and it is the
single most common reason mail "does not arrive" on a fresh deployment.

```bash
# Kubernetes
helm get values <release> -n <ns> | grep -A3 '^email:'
# Compose
grep EMAIL_PROVIDER .env
```

`stub` means nothing is sent. Set a real provider before reading further.

Non-production deployments should use **Mailpit** instead: it accepts
everything, delivers nothing, and gives you a web inbox to read. The demo
compose stack wires it up already (`MAILPIT_UI_PORT`, default
<http://localhost:8025>). None of the DNS below applies to Mailpit.

## The four records

Three DNS records and one reverse lookup decide whether a receiving server
trusts your mail. Set them on the domain in `email.from` — not on the domain
Inventario is served from, if they differ.

### 1. SPF — which servers may send as your domain

One `TXT` record at the domain root. It lists the servers allowed to send, and
ends with a policy for everything else.

```dns
example.com.  IN  TXT  "v=spf1 include:spf.example-provider.com -all"
```

- `include:` comes from your provider's documentation. Use theirs verbatim; a
  hand-written `ip4:` list breaks the first time they add a sending IP.
- End with `-all` (hard fail), not `~all` (soft fail). Soft fail means "treat
  unauthorized mail as suspicious", which in practice means "deliver it to
  spam", which is the outcome you are trying to avoid for everyone else using
  your domain.
- **Exactly one** SPF record per domain. Two is a permanent error, not a
  merge — receivers reject both. If you already have one, add the `include:`
  to it rather than publishing a second.
- SPF has a hard limit of **10 DNS lookups**. Each `include:` costs at least
  one. Exceeding it is also a permanent error.

### 2. DKIM — a signature proving the message was not altered

Your provider generates a key pair, keeps the private half, and gives you a
public key to publish as a `TXT` record at a selector they choose:

```dns
<selector>._domainkey.example.com.  IN  TXT  "v=DKIM1; k=rsa; p=MIGfMA0GCS..."
```

There is nothing to configure in Inventario — the provider signs on the way
out. Publish the record, then confirm in the provider's dashboard that it sees
the key; most will not sign until they have verified it.

### 3. DMARC — what receivers should do when the first two fail

One `TXT` record at `_dmarc`, telling receivers your policy and where to send
reports.

```dns
_dmarc.example.com.  IN  TXT  "v=DMARC1; p=none; rua=mailto:dmarc@example.com; pct=100"
```

Start at `p=none`. It changes nothing about delivery and turns on the
aggregate reports, which tell you what is already being sent as your domain —
including things you forgot about. Read a week of them, confirm every
legitimate sender passes, then tighten:

```text
p=none  →  p=quarantine  →  p=reject
```

Moving to `p=reject` before your own mail passes SPF and DKIM will bounce it.
That is the mechanism working; it is still an outage you caused.

DMARC additionally requires **alignment**: the domain in the `From:` header
must match the domain that passed SPF or DKIM. This is why `email.from` matters
— see below.

### 4. Reverse DNS (PTR) — only if you run your own SMTP server

If you send through a provider, skip this; their IPs already have it. If you
run your own mail server, the sending IP needs a `PTR` record resolving to a
hostname that resolves back to the same IP. Many receivers reject mail from an
IP with no reverse lookup before looking at anything else, and it is set by
whoever owns the IP — your hosting provider — not in your own DNS zone.

Running your own outbound SMTP for transactional mail is a standing
commitment: IP reputation, blocklist monitoring, warm-up. For an application
that sends a handful of messages per user, a provider is the right answer.

## Make `email.from` align

```yaml
email:
  from: "Inventario <noreply@example.com>"   # example.com must be the domain you set the records on
  replyTo: "support@example.com"             # optional, but a real inbox is better than a black hole
```

Two rules:

- The `from` domain must be the domain carrying SPF/DKIM/DMARC. Sending as
  `@gmail.com` through your own provider fails DMARC alignment at every
  receiver, whatever your records say.
- The address must be one your provider has **verified**. Most refuse to send
  from an unverified sender, which surfaces in Inventario as a delivery error
  in the worker log rather than as a bounce.

`noreply@` is conventional for transactional mail, but set `replyTo` to
something a person reads. A user who replies to a verification mail asking why
it did not work deserves better than silence.

## Verify before you invite anyone

Do this once, on the real deployment, before the first external user:

1. **Send yourself one.** Register a test account with an address on a
   different provider than your own (a Gmail or Outlook address is ideal —
   they are the strictest and the most common).
2. **Read the headers.** In Gmail: ⋮ → *Show original*. You want three passes:

   ```text
   SPF:   PASS with IP 203.0.113.10
   DKIM:  PASS with domain example.com
   DMARC: PASS
   ```

   A `PASS` on SPF but `FAIL` on DMARC almost always means alignment: the
   envelope sender is your provider's domain and `From:` is yours, with no
   DKIM signature to bridge them. Publish the DKIM record.
3. **Score it.** Send one to a throwaway address from
   [mail-tester.com](https://www.mail-tester.com) and read the report. It
   checks the records above plus the things nobody thinks of — blocklist
   entries, missing `List-Unsubscribe`, an HTML-to-text ratio that reads as
   spam.
4. **Check the spam folder, not just the inbox.** "It arrived" and "it arrived
   where the user will look" are different results.

Repeat step 1 after any change to the sending domain, the provider, or
`email.from`.

## When mail still does not arrive

Work down this list; it is roughly ordered by how often each is the cause.

| Symptom | Where to look |
| --- | --- |
| Nothing is sent at all, no log line | Provider is still `stub` |
| Worker log shows an auth or sender error | Provider credentials, or an unverified `email.from` |
| Delivered to spam, SPF fails | SPF record missing, has two entries, or exceeds 10 lookups |
| Delivered to spam, SPF passes | No DKIM signature, or DMARC alignment fails |
| Some receivers only | Blocklist entry for the sending IP — check the mail-tester report |
| Link in the mail 404s or points at localhost | `app.publicUrl` is unset or wrong; it is not a deliverability problem |

The worker log is the first stop for anything that looks like a send failure:

```bash
kubectl -n <ns> logs deploy/<release>-inventario-worker-emails   # split mode
kubectl -n <ns> logs deploy/<release>-inventario                 # combined mode
docker compose logs inventario                                   # Compose
```

Leave `email.logUrls` off in production. It writes verification and reset links
to the log, which turns a log reader into an account takeover.
