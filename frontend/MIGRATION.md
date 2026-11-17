# Native migration

The original reference is preserved under static/reference/v016.html. Native components share its extracted CSS. This milestone implements the dashboard shell, navigation, project gallery, container-image simulation, release history, logs, rollback, domain inventory and association, server overview/activity and palette settings.

Remaining: build and browser validation; Git; other infrastructure pages; authentication and the Go API. No placeholder is a working server service.

## M02: Databases
Native database inventory, inline creation, credentials, user grants, privileges, allowed hosts, SQL/GZ sample export, import preview, maintenance, history, and optional project associations are implemented. All operations are simulated; live SQL and administration tools, actual backups/restore, real authentication and API integration remain pending. Unit tests run through `npm test` (Node.js 22+).

## M03: Storage and File Manager
Independent spaces with quota/privacy settings, shared folder tree and file operations, project Files tab, sample URLs, and a source picker that copies a selected ZIP or folder to a new project draft. Browser-only file content, no server filesystem access. Source copies remain unexecuted; ZIP unpacking/framework analysis and live storage API are pending. File limit: 1 MiB per upload/source, 3 MiB aggregate sample content, 500 entries per tree; actual browser quota may be lower. No overwrites. Existing container demo is retained.

## M04: Application detection
Native source inspection for ZIPs and folders, bounded extraction with CRC/path checks, framework and nested root detection, package manager and command recommendations, optional runtime overrides, database hints and environment variable names. A detected or uncertain application can be saved as a draft; no build or application execution occurs. The input and expanded archive are each capped at 1 MiB for browser persistence, with 500 entries per tree and a 3 MiB aggregate file limit. ZIP64, encrypted, symlink, split and non-UTF-8 archives are rejected. Real Git and deployment engine remain pending.

## M05: Email and standalone webmail
Native Mailboxes, Forwarders and DNS & setup, mailbox details and password entry/generation, quotas, suspension, auto-replies, and client settings. Webmail opens a separate /webmail/[id] tab with no panel shell. Folder/search/reader/compose/draft/reply/forward/mark/star/move/restore/delete use mailbox-scoped demo state; browser storage events synchronize other tabs and mutations read the latest saved state. Simultaneous tab writes are not transactional. No SMTP/IMAP, inbound/outbound delivery, attachments, actual DNS checks, credentials, authorization or employee login are implemented. Mailbox switching is a demo-only feature; real deployment requires authenticated mailbox isolation.

## M06: Website hosting
The native Websites inventory shares project state. Guided fresh WordPress and migration setup creates a draft project, domain association, private MySQL database/user, and sample file workspace. Migration validates ZIP and SQL/SQL.GZ selection metadata only (or sample mode); no archive contents are read or restored. PHP/static uploads reuse source detection and File Manager; publishing readiness checks the entry point and records a preview without starting a server. WordPress install, backup import, migrations, PHP runtime, wp-admin login and actual publishing require backend work. Browser stores are separate; setup uses compensating rollback on ordinary write failures, not a transactional database.
