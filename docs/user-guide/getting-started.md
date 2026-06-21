# Getting started with Inventario

Welcome! Inventario helps you keep track of everything you own — what it is,
where it lives, what it cost, and which warranties are still active. This guide
walks you through the things you'll do most often. You don't need to read it
top to bottom: jump to whatever you need.

The app also has a short product tour built in. If you ever want to see it
again, open the user menu and choose **Restart product tour**.

## Contents

- [Add your first item](#add-your-first-item)
- [Organise with locations, areas, and tags](#organise-with-locations-areas-and-tags)
- [Attach files (receipts, manuals, photos)](#attach-files-receipts-manuals-photos)
- [Scan an item with AI](#scan-an-item-with-ai)
- [Export and back up your data](#export-and-back-up-your-data)
- [Invite people to your group](#invite-people-to-your-group)

---

## Add your first item

An **item** is anything you want to keep track of — an appliance, a laptop, a
piece of furniture, a tool.

1. Click **Add item** (the button is on the Dashboard and on the All Items
   page).
2. The **Add item** dialog opens. You can start two ways:
   - **Fill with AI** — drop in a photo or a receipt/invoice PDF and let AI
     pre-fill the form for you (see [Scan an item with AI](#scan-an-item-with-ai)).
   - **Fill manually** — type the details yourself.
3. Work through the steps:
   - **Basics** — name, quantity, price, and other core details.
   - **Extras** — optional fields, including tags.
   - **Files** — optionally attach photos, receipts, or manuals (you can also
     add these later).
4. Click the create button to save. Your new item opens so you can review it.

Items don't have to be placed in a location to be saved. If you skip that, the
item appears with a small banner offering to **Place in location** — you can do
that whenever you like, or leave it unassigned.

If you're trying Inventario from the public landing page before creating an
account, you can draft your first item right there. After you sign up, the app
finishes adding it for you — your draft is never lost.

---

## Organise with locations, areas, and tags

Inventario gives you three ways to organise, and you can mix and match.

### Locations

A **location** is a physical place where you keep things — a house, a flat, a
garage, a storage unit, a vehicle.

1. Go to **Locations** in the sidebar.
2. Click **Add location**.
3. Give it a name. The address and description fields are optional — the
   address can be a street address, a room, or any free-text note.

### Areas

An **area** is a subdivision inside a location — Kitchen, Garage, Office, a
specific shelf. Areas are where items actually live.

1. Open a location, or use **Add area** from the Locations page.
2. Choose the parent location, pick an icon, and name the area (for example
   "Kitchen").
3. Items you assign to an area show up on that area's page, along with a few
   quick stats.

### Tags

**Tags** are flexible labels you can apply across items — think "fragile",
"electronics", or "to sell". They're independent of where an item is stored.

- Add tags to an item on the **Extras** step of the Add item dialog.
- Browse and manage them on the **Tags** page. Tags are split into two types:
  **Item tags** and **File tags** — use the switcher at the top to choose which
  set you're looking at.

A tag starts showing up in these lists once it's actually used somewhere, so a
brand-new account will have an empty Tags page until you apply your first tag.

---

## Attach files (receipts, manuals, photos)

You can attach files — photos, receipts, manuals, warranty certificates,
anything — to your items.

- **While adding an item:** use the **Files** step in the Add item dialog. Drop
  files onto the dropzone or click to browse.
- **From an existing item:** open the item, go to its **Files** tab, and upload
  there.
- **From the Files page:** the **Files** section in the sidebar lists every file
  in your group; you can switch between a grid and a list view.

Inventario sorts each file automatically based on its type — into **Photos**,
**Documents**, or **Other** — and you can change a file's category later from
its detail page. Files are always optional: if you skip them while creating an
item, you can add them at any time.

---

## Scan an item with AI

If AI vision is enabled on your server, you can let it read a photo or document
and pre-fill the item form for you.

1. In the **Add item** dialog, choose **Fill with AI**.
2. Drop in your files, or click to browse. Supported formats: **JPG, PNG, WEBP,
   HEIC/HEIF, or PDF** — up to **5 files** at a time. A clear photo of the item
   plus its label (or a receipt/invoice PDF) works best.
3. AI reads the files and shows you the details it extracted, with a confidence
   indicator for each.
4. On the **Review extracted details** step, untick anything that looks wrong,
   then choose **Use these values** to pre-fill the form.
5. Finish and save the item as usual. The files you scanned are attached to the
   item automatically — you can manage them on the Files step.

A few things to know:

- If AI finds more than one product in the files, it pre-fills the most
  prominent one and lets you add the others separately afterwards.
- If AI vision isn't enabled on your server, you'll see a message saying so —
  just use **Fill manually** to continue.
- Scanning is rate-limited to keep usage fair, so you may occasionally be asked
  to wait a moment before trying again.

---

## Export and back up your data

Inventario can export your data so you have your own copy, or so you can move it
to another instance. Find this under **Backup** in the sidebar.

### Create an export

1. Go to **Backup** and start a **New export**.
2. Choose what to include:
   - **Full database** — everything: locations, areas, items, files, and tags.
   - **Selected items** — pick specific locations, areas, or items.
3. Optionally add a description. If you leave it blank, Inventario names the
   export for you (for example, "Backup · Full database · 2026-05-13 10:42 UTC").
4. The export runs in the background. When it finishes, **Download** it from the
   list.

Backups download as `.inb` files — Inventario's signed backup format.

### Import or restore a backup

- **Import backup** lets you upload an existing `.inb` file. It's staged on the
  server first and only inspected when you start the restore.
- **Restore** brings the contents of an export into your current group. You pick
  a strategy and can run a **dry run** first to preview exactly what would
  change before anything is modified. You can also choose whether to restore the
  attached file data alongside the metadata.

Running a dry run before a real restore is a good habit — it shows you what
would happen without touching your live data.

---

## Invite people to your group

Inventario organises everything into **groups**. You can invite other people
into a group and give each of them a role.

1. Open **Members** in the sidebar.
2. Click **Invite**.
3. Enter the person's email address and choose a **role** (see below).
4. Send the invite. They'll get an invite link to join. If you'd rather, you can
   create a copy-paste invite link instead of sending an email.

Pending invites show a **Pending** badge; you can **Resend** or **Revoke** them
from the Members page.

### Roles

| Role | What they can do |
| --- | --- |
| **Owner** | Full access, including deleting the group and all its data. |
| **Administrator** | Manage members, group info, locations, and areas. |
| **User** | Add and remove items (but not locations or areas). |
| **Viewer** | View everything, but can't make any changes. |

---

## Need more help?

- **Keyboard shortcuts:** open **Settings → Help & support → Keyboard
  shortcuts**, or press `?` anywhere in the app.
- **Contact support / share feedback:** **Settings → Help & support → Contact
  support / share feedback** opens a form to ask a question, report a bug, or
  suggest a feature.
