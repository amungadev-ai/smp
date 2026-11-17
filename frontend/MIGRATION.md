# Native migration

The original reference is preserved under static/reference/v016.html. Native components share its extracted CSS. This milestone implements the dashboard shell, navigation, project gallery, container-image simulation, release history, logs, rollback, domain inventory and association, server overview/activity and palette settings.

Remaining: build and browser validation; actual ZIP/folder inspection; Git; WordPress; mail and standalone webmail; other infrastructure pages; authentication and the Go API. No placeholder is a working server service.

## M02: Databases
Native database inventory, inline creation, credentials, user grants, privileges, allowed hosts, SQL/GZ sample export, import preview, maintenance, history, and optional project associations are implemented. All operations are simulated; live SQL and administration tools, actual backups/restore, real authentication and API integration remain pending. Unit tests run through `npm test` (Node.js 22+).

## M03: Storage and File Manager
Independent spaces with quota/privacy settings, shared folder tree and file operations, project Files tab, sample URLs, and a source picker that copies a selected ZIP or folder to a new project draft. Browser-only file content, no server filesystem access. Source copies remain unexecuted; ZIP unpacking/framework analysis and live storage API are pending. File limit: 1 MiB per upload/source, 3 MiB aggregate sample content, 500 entries per tree; actual browser quota may be lower. No overwrites. Existing container demo is retained.
