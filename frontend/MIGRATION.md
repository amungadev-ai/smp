# Native migration

The original reference is preserved under static/reference/v016.html. Native components share its extracted CSS. This milestone implements the dashboard shell, navigation, project gallery, container-image simulation, release history, logs, rollback, domain inventory and association, server overview/activity and palette settings.

Remaining: build and browser validation; actual ZIP/folder inspection; Git; storage and filesystem; WordPress; mail and standalone webmail; other infrastructure pages; authentication and the Go API. No placeholder is a working server service.

## M02: Databases
Native database inventory, inline creation, credentials, user grants, privileges, allowed hosts, SQL/GZ sample export, import preview, maintenance, history, and optional project associations are implemented. All operations are simulated; live SQL and administration tools, actual backups/restore, real authentication and API integration remain pending. Unit tests run through `npm test` (Node.js 22+).
