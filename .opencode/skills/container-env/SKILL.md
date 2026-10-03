---
name: container-env
description: Use when setting up or starting work on a new or existing project in an opencode Docker container. Triggers on "new project", "existing project", "Docker", "Dockerfile", "docker-compose", "environment setup", or "how is this environment set up". The session already runs inside a pre-built opencode authoring container; this skill's job is to read the project code and derive the project's own dev/test container (Dockerfile + docker-compose.yml) from it. Do not audit or repair the authoring container.
---

# container-env

## Two containers, two roles

There are two distinct containers in this model. Never confuse them.

**Container A - the opencode authoring container (this session).**
The container this session is running in right now. It was built on the
host from the opencode base image and started by the user (e.g. via
`setup_opencode.ps1` and `docker run`). It is already running and already
sufficient for authoring code: writing, editing, committing. It is NOT
the subject of this skill. Do not inspect it for missing tools, do not
try to improve, repair, or rebuild it, and do not derive project
requirements from whatever is or is not installed inside it.

**Container B - the project's dev/test container.**
What this skill produces and maintains. Each project gets a `Dockerfile`
(build recipe) and a `docker-compose.yml` (run configuration). Container
B runs the project's code as the dev/test environment, isolated from
container A, with the source directory mounted and ports exposed so the
user can view the project at runtime. Its requirements come from the
project code, never from container A's toolchain.

## The job: read the code, derive container B

The core task is code analysis, not environment introspection.

1. Read the project code first: package manifests (package.json,
   requirements.txt/pyproject.toml, go.mod, Cargo.toml), language and
   toolchain, frameworks, entry points, exposed ports, test commands.
2. Derive what container B needs from those facts.
3. Write or update the project's `Dockerfile`: dependencies pinned and
   layered near the top of the file so build caching keeps rebuilds
   fast; app-code layers last.
4. Write or update the project's `docker-compose.yml`: run the built
   image as the dev/test environment, source mounted, ports exposed.

There is intentionally no separate runtime image. The project's run
target is the dev environment via compose. Add a standalone
runtime/packaged image only later, and only if the project is ever
shipped, deployed, or handed off.

Reference file layout (inside the project):

```
./Dockerfile          # project image: deps layered for cache-friendly rebuilds
./docker-compose.yml  # dev/test environment, source mounted, ports exposed
./(project code)      # written and edited in the opencode authoring container
```

## What NOT to do

- Do not audit container A. Its toolchain is irrelevant to container B's
  requirements.
- The `docker` CLI is generally NOT available inside container A. Image
  builds are triggered by the user on the host; the session writes the
  config files and the user runs the build.
- Do not install packages into container A as the authoritative change
  for a project dependency. Any such install is at most a temporary
  local probe and must be recorded back into the project's `Dockerfile`.

## New vs. existing projects

- **New project** (no container config yet): briefly discuss the
  project's purpose with the user, then draft the `Dockerfile` and
  `docker-compose.yml` following the model above. No scaffolding of
  build files before the purpose is agreed.
- **Existing project** (already has a Dockerfile/compose, e.g. a cloned
  repo): read and respect what is already there. Only fill gaps or fix
  what is broken; do not rewrite working config to match a preferred
  style.

## Dependency workflow: user-triggered, rebuild-only

Adding a package dependency over time is driven by the user, never
ambient:

1. The session reads the code and derives which packages it requires.
2. The session updates the project's `Dockerfile` (and
   `docker-compose.yml` if needed) to add the dependency as a new layer.
3. The **user** manually triggers the build/update
   (`docker compose build` / `docker compose up`) on the host. The
   session does not run it.

## Session start

When this skill activates, state in one line that the session runs in
the already-set-up opencode authoring container (A), then move directly
to analyzing the project code and container B's needs. No tool audits of
container A.

## Install notes

To install this as a live skill, place it at
`.opencode/skills/container-env/SKILL.md` (project scope) or
`~/.config/opencode/skills/container-env/SKILL.md` (global scope), then
quit and restart opencode for it to take effect.
