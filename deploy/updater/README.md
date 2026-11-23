# Host updater

`vibeapi-updater` runs on the Docker host. The web container never receives the
Docker socket. The updater accepts only release IDs and immutable digests from
the configured GitHub/Docker Hub release catalog, and replaces only one
allowlisted Compose service.

## Install

1. Create a locked `vibeapi-updater` system user, add it to the host Docker
   group, then build with
   `go build -o /usr/local/bin/vibeapi-updater ./cmd/vibeapi-updater`.
2. Copy `config.example.json` to `/etc/vibeapi-updater/config.json`, set absolute
   Compose paths under `/opt/vibeapi`, make those files (including `.env` and
   referenced env files) readable by `vibeapi-updater`, list any existing image repository under
   `bootstrap_repositories`, and generate two different random secrets of at
   least 32 characters. Bootstrap repositories are accepted only as the
   currently running/rollback source; selectable releases must still come from
   `image_repository` and its immutable catalog digest. If the service belongs
   to a Compose profile, list it under `compose_profiles`: Compose neither
   lists nor replaces a service whose profile is inactive.
3. Pass the same values to the web service as
   `SYSTEM_UPDATE_UPDATER_TOKEN` and `SYSTEM_UPDATE_READINESS_TOKEN`. Bind-mount
   `/run/vibeapi-updater` into the web container at the same path; do not mount
   `/var/run/docker.sock` into the web container. If the web container runs as
   a non-root user, add the host GID of `vibeapi-updater` with Compose
   `group_add` so it can open the `0660` Unix socket. On SELinux hosts, apply an
   appropriate bind-mount label instead of disabling SELinux.
4. Install `vibeapi-updater.service`, run `systemctl daemon-reload`, then enable
   and start it. The unit keeps `/run/vibeapi-updater` across updater restarts,
   because the web container would not see a recreated directory.

The example systemd sandbox hides home directories and assumes deployment
files live under `/opt/vibeapi`. If another directory is required, update both
`compose_files` and the unit's filesystem sandbox deliberately; do not grant
write access to the Compose directory.

If the Compose files or the env files they reference are readable only by root
(for example under `/root`), run the unit as root instead of the dedicated
user: remove `User`, `Group` and `SupplementaryGroups`, and set
`ProtectHome=read-only` so those files stay readable but not writable. Docker
group membership already gives root-equivalent control of the host, so this
does not widen what the updater can do through Docker.

`app_url` is restricted to a loopback host so the readiness secret cannot be
sent to a remote server. If the web service publishes no port, add a
loopback-only mapping such as `127.0.0.1:13000:3000` and point `app_url` at
it. `minimum_free_bytes` blocks an update when the Docker storage filesystem
falls below the configured reserve. It is a safety reserve, not an exact
estimate of the selected image's compressed or expanded size.

The updater writes `/var/lib/vibeapi-updater/managed-override.yaml` after a
successful switch or rollback. Include it as the final `-f` file in any later
manual `docker compose` command so the selected digest remains pinned, or set
`COMPOSE_FILE` in the project's `.env` to the Compose files followed by the
override so plain `docker compose` commands in the project directory include it.
Compose fails when a listed file is missing, so set `COMPOSE_FILE` only after
the override exists.

Before a rollback, back up the database. Replacing an image does not reverse
database migrations. Releases without a published image digest remain visible
in the panel but cannot be installed. Historical images that predate the
dedicated readiness endpoint are checked through the legacy `/api/status`
endpoint only when the dedicated endpoint returns HTTP 404.
