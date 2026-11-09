# BiletFlow frontend

The Next.js App Router frontend uses the OpenAPI contract in `../docs/openapi.yaml` as its API source of truth.

## Development

Start Prism from the frontend directory with `prism mock ../docs/openapi.yaml --host 0.0.0.0 --port 4010`. Then, from this directory, generate typed API definitions with `npm run generate:api` and start Next.js with `npm run dev`.

The browser calls same-origin `/api/*` routes. Next.js rewrites auth, users, and organizer-profile requests to `IDENTITY_API_URL` (default `http://localhost:8000`) and event requests to `EVENTS_API_URL` (default `http://localhost:8080`). To use Prism locally, set both server-side variables to `http://localhost:4010`. For deployed environments, set them to the corresponding backend URLs; do not use `NEXT_PUBLIC_*` API URLs.

The JWT is held in memory because the current API contract returns it in the login response and does not define an httpOnly-cookie session flow. A page reload clears the session by design. A 401 from an authenticated request clears the in-memory token so protected dashboard routes return to sign-in.

The event-list operation currently declares Bearer authentication even though the product brief calls it public. Explore displays only published, public events and offers sign-in rather than bypassing the documented security requirement. The edit request schema also omits `category` and does not accept `null` for clearing capacity or an end time; the edit form reflects those limits. Event edit, delete, and unpublish are wired to their documented routes, which the OpenAPI file currently marks as planned. Forgot-password and reset-password screens use the contract's request and completion endpoints.

`/dev/components` is available only outside production builds.
