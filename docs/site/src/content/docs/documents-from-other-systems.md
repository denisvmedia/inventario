---
title: Documents from other systems
description: Send invoices and receipts from Paperless-ngx or another system to Inventario, where each one becomes a draft in the inbox.
---

Another system that keeps your documents, such as
[Paperless-ngx](https://docs.paperless-ngx.com/), can send each invoice or
receipt to Inventario. Every document it sends becomes a
[draft of a new item](../items/#drafts-of-a-new-item) in the group's
[Inbox](../inbox/), with the document attached and the details the other
system knows already filled in. You finish the draft like any other.

The other system calls one endpoint with a [personal API token](../api-tokens/).
Inventario never calls the other system back: it does not open the document's
address, and it does not need to reach the other system at all.

## What the draft holds

| Sent field | Becomes |
| --- | --- |
| `title` | The draft's name. |
| `source_url` | The first of the draft's **Links**. |
| `seller` | The draft's seller: set when the group already has a seller of that name, otherwise offered on the **Purchase** step of the draft, where you can create it. |
| `purchase_date` | The purchase date. |
| `amount`, `currency` | The original price and its currency. |
| `file` | The document itself, attached to the draft as a file of the group. |

A field that is not sent stays unknown on the draft. No AI scan runs on the
document: the draft holds only what the other system sent, and you add the
rest when you finish the item.

The draft's row in the inbox says where it came from, for example **From
Paperless-ngx**, and links to the document there when its address was sent.
The item you make from the draft keeps that link as its **Source** on the
**Details** tab.

## Set it up

1. In Inventario, create a token under **Preferences → Privacy & security →
   API tokens** (see [Settings & account](../settings-and-account/#api-tokens)).
   Give it **Read and write** access and limit it to the group the documents
   belong in. The account the token belongs to needs a role in that group that
   can add items, so not **Viewer**.
2. Find the group's slug: it is the part after `/g/` in the address of any
   page of the group, or the `slug` that `GET /api/v1/groups` returns for the
   token.
3. Point the other system at
   `https://inventario.example.com/api/v1/g/<group slug>/intake` with the
   header `Authorization: Bearer <token>`, and send the fields below.

## Paperless-ngx

In Paperless-ngx, create a workflow under **Workflows** with a trigger such as
**Document Added**, and add a **Webhook** action to it:

- **Webhook url**: `https://inventario.example.com/api/v1/g/<group slug>/intake`
- **Use parameters for webhook body**: on.
- **Send webhook payload as JSON**: off. The document can only be included in a
  form body.
- **Include document**: on.
- **Webhook headers**: `Authorization` with the value `Bearer inv_pat_…`, your
  token.
- **Webhook params**:

| Name | Value |
| --- | --- |
| `title` | `{{doc_title}}` |
| `document_id` | `{{doc_id}}` |
| `source_system` | `paperless-ngx` |
| `source_name` | `Paperless-ngx` |
| `source_url` | `{{doc_url}}` |
| `seller` | `{{correspondent}}` |
| `purchase_date` | `{{created}}` |

`{{doc_url}}` needs the `PAPERLESS_URL` setting of Paperless-ngx. Without it,
leave `source_url` out: the draft then has no link to the document.

When Inventario runs on your local network, Paperless-ngx must be allowed to
call it: `PAPERLESS_WEBHOOKS_ALLOW_INTERNAL_REQUESTS` has to stay `true`, its
default, and when `PAPERLESS_WEBHOOKS_ALLOWED_PORTS` is set it has to list
Inventario's port.

To send a price you keep in a monetary custom field, for example one named
**Price**, add `amount` with the value
`{{ custom_fields | get_cf_value("Price", "") }}`. Paperless-ngx writes such a
value with its currency in front, for example `EUR12.30`, which Inventario
reads as the amount and its currency.

Paperless-ngx waits five seconds for an answer and does not follow redirects,
so use the address Inventario answers on. It tries again up to three times when
the answer is an error status, a refusal included, but not when the request
timed out or could not connect: then it only logs the error. A large document
on a slow link can take longer than five seconds, so check the inbox when
Paperless-ngx logs a timeout, and send the document again if its draft is
missing. Sending a document again with the same fields makes no second
draft, see [Sending a document again](#sending-a-document-again).

## Fields

The endpoint takes the fields as a form (`application/x-www-form-urlencoded`),
as a multipart form (`multipart/form-data`) with the document in a part named
`file`, or as a flat JSON object without a document. A field sent twice is
refused, and so is an unknown field, so a typo shows up in the other system's
log instead of being dropped. In JSON a field is a string; `document_id`,
`source_system` and `source_instance` may also be whole numbers, which are
read as their digits.

| Field | Required | Format |
| --- | --- | --- |
| `title` | yes | Up to 255 characters. |
| `source_system` | yes | The system's id: letters, digits, dots, dashes and underscores, starting with a letter or a digit, up to 255 bytes, for example `paperless-ngx`. Letter case does not matter. |
| `document_id` | yes | The document's id in that system, up to 255 bytes. |
| `source_instance` | no | Tells two installations of one system apart, for example `home` and `office`, up to 255 bytes. Leave it out when you have one. |
| `source_name` | no | How the system is shown, for example `Paperless-ngx`, up to 100 characters. Without it the inbox shows `source_system`. |
| `source_url` | no | The document's address there: an absolute `http` or `https` address without a user name or password, up to 2048 bytes. It is stored and shown, never opened by Inventario. |
| `seller` | no | Up to 100 characters. |
| `purchase_date` | no | `YYYY-MM-DD`, or a date and time in RFC 3339 form, of which the date is kept. |
| `amount` | no | A number of zero or more with up to 13 digits before the decimal point and up to two after it, for example `129.90`, optionally after a currency code (`EUR129.90`). |
| `currency` | no | A three-letter code such as `EUR`. When `amount` starts with a code, the two must agree. |
| `file` | no | The document, in a multipart form only. Any file type the app accepts as an upload, up to the server's upload limit. |

Every field is text without NUL characters. A byte is one character for the
letters, digits and signs of plain ASCII; an accented letter takes two bytes
or more. The fields other than the file may be 64 KiB together. A curl call
that sends
a document looks like this:

```sh
curl -H "Authorization: Bearer $INVENTARIO_TOKEN" \
  -F title="Invoice 2024-117" -F source_system=paperless-ngx \
  -F document_id=117 -F source_url=https://paperless.example.com/documents/117/ \
  -F amount=EUR129.90 -F file=@invoice-117.pdf \
  https://inventario.example.com/api/v1/g/home-x7k2/intake
```

## Sending a document again

`source_system`, `source_instance` and `document_id` identify the document in
the group. Inventario remembers each document it received, so a document makes
one draft at most:

- The same document with the same fields and the same file again answers
  **200** with what became of it and makes nothing. A first delivery answers
  **201**.
- The same document with other fields or another file is refused with **409**
  `intake.document_changed`, and nothing changes. Edit the draft or the item
  in Inventario instead. This is what happens when a Paperless-ngx workflow
  with a **Document Updated** trigger sends a document after you edited it
  there.
- A delivery that failed half way, for example because the connection broke,
  is finished by sending the document again with the same fields and file,
  without a second draft or a second copy of the file. Other fields are
  refused then too: the first delivery stands for the values it sent.
- This holds after the draft was discarded, after the draft became an item,
  and after the item was deleted. To have a document make a new draft, send
  it with another `source_instance`.

The answer's `state` says what became of the document: `pending` while the
delivery has not made its draft yet, `draft` while its draft waits in the
inbox, `item` once the draft became an item, and `removed` when the draft was
discarded or the item deleted. `removed` is also the state of a document whose
batch was deleted from the inbox before its draft was made; sending it again
does not bring it back.

A **Full database** [backup](../backup-and-restore/) carries what Inventario
remembers about the documents it received, so a document sent again after a
restore makes no second draft either. See
[Documents from other systems](../backup-and-restore/#documents-from-other-systems)
there for what each restore strategy does with it.

## Errors

| Status | Code | Meaning |
| --- | --- | --- |
| 401 | `api_token.invalid` | The token is unknown, revoked or expired. |
| 403 | `intake.api_token_required` | The request came from a browser session. The endpoint takes an API token only. |
| 403 | `api_token.read_only` | The token may only read. |
| 403 | `api_token.group_not_allowed` | The token is limited to another group. |
| 403 | | The token's account has the **Viewer** role in the group. |
| 409 | `intake.document_changed` | The document was received before with other fields or another file. |
| 409 | `intake.document_conflict` | A batch in the inbox carries the document, but the group has no record of the delivery that made it. Delete that batch, then send the document again. |
| 413 | | The document or the fields are larger than allowed. |
| 415 | | The body is not a form, a multipart form or JSON. |
| 422 | | A field is missing, unknown, sent twice or malformed; the answer names it. |
| 423 | | A currency migration is running in the group. Send the document again later. |
| 429 | | Too many requests, or too many uploads at once. |
