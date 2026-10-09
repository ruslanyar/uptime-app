---
name: openapi-contract
description: Maintain this project's OpenAPI contract when creating, changing, or deleting API routes, or changing their request/response schemas, status codes, authentication, or CSRF requirements. Use also for swaggo annotations, Swagger generation and checks, and ReDoc HTML generation in uptime-app.
---

# OpenAPI Contract

Use this skill for API contract work in this repository. Keep handler annotations
and generated Swagger files in sync with the actual HTTP behavior.

## Sources and annotations

- Inspect route registration in `backend/internal/httpapi/`, the affected handler,
  its DTOs and middleware before updating annotations. Routes and code are the
  source of truth; generation alone does not prove that annotations match them.
- Keep global API metadata, `@BasePath` and `securityDefinitions` in
  `backend/cmd/api/main.go`. The contract uses Swagger / OpenAPI 2.0.
- Put swaggo annotations above every API handler. Follow nearby handlers for
  syntax: `@Summary`, `@Description`, `@Tags`, `@ID`, `@Accept`, `@Produce`,
  `@Param`, `@Security`, `@Success`, `@Failure`, `@Header` and `@Router` as
  applicable. Keep operation IDs unique and paths relative to `@BasePath`.
- Describe actual request DTOs, required fields, path/query/header parameters,
  content types, response schemas, response headers and reachable status codes.
  Use the shared `response.ErrorResponse` only for JSON error envelopes;
  file and plain-text responses need their own schemas. A `204` has no body.
- On route creation, add handler annotations; on changes, update all affected
  annotations and DTO schemas; on deletion, remove obsolete annotations and
  verify that the operation disappears from both generated contracts.
- Describe Bearer JWT through `BearerAuth` and `@Security BearerAuth` on protected
  operations. Describe cookie authentication in operation descriptions because
  OpenAPI 2.0 has no cookie security scheme. Preserve the precedence of a present
  `Authorization` header, including an invalid one, over the access-cookie.
- Document `X-CSRF-Protection: 1` as a required header on mutations and include
  middleware errors such as Origin/CSRF rejection. Check actual middleware and
  [authentication rules](../../../docs/rules/backend/authentication.md).
- For `/auth/profile`, keep the JSON body schema and alternative multipart schema
  in `@x-multipart-request`, with `@Accept json,mpfd`. Never combine `body` and
  `formData` parameters in one operation. Swagger UI does not interpret this
  extension automatically. Check limits against the handler and
  [profile rules](../../../docs/rules/backend/profile.md).

## Tools and generation

Paths in commands below are relative to `backend/` unless stated otherwise.
Scripts resolve the backend directory from their own location, so they can also
be invoked from another working directory using the correct script path.

Use `go tool swag`, pinned in `backend/go.mod`; do not install a separate `swag`
binary. Install the pinned Redocly CLI with `npm ci` from the repository root.
Its version is defined in the root `package.json` and lockfile.

After changing the HTTP contract, run:

```sh
go generate ./cmd/api
```

The directives invoke the skill's scripts to generate
`backend/api/openapi/swagger.json`, `swagger.yaml`, and `redoc.html`.
Include both Swagger files in the same change as the code and annotations.
Never edit generated contracts manually. The HTML build is ignored by Git;
do not commit it. Open it in a browser to view the API documentation;
ReDoc loads from a CDN and needs internet access.

To generate only Swagger, or only HTML from the existing Swagger:

```sh
sh ../.agents/skills/openapi-contract/scripts/generate-openapi.sh
sh ../.agents/skills/openapi-contract/scripts/generate-redoc.sh
```

`generate-openapi.sh` optionally accepts an output directory. Relative output
paths resolve from `backend/`; an absolute path is useful for temporary output.

## Validation

Before the Go checks, run:

```sh
sh ../.agents/skills/openapi-contract/scripts/check-openapi.sh
```

The check regenerates JSON/YAML in a temporary directory and fails if either
differs from the tracked contract. When it fails, update annotations as needed,
regenerate and review the contract diff against the affected routes, schemas,
status codes, authentication and CSRF behavior. Confirm deleted operations and
unused definitions disappear. Then run the backend checks required by
[backend/AGENTS.md](../../../backend/AGENTS.md) and
[backend/README.md](../../../backend/README.md#проверки).
