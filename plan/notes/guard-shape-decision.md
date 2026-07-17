# Guard config shape decision

## Question

How should the origin guard be configured on `configureApiClient()`? Candidates:
- `allowedOrigins: string[]` allow-list — a pure validation guard that leaves `request(input, options)`'s existing calling convention untouched.
- `baseUrl: string` — implies also using it to resolve/construct request URLs, a materially larger change to `request()`'s calling convention.

## Answer

allowedOrigins list (Recommended)

A pure validation guard: `configureApiClient({ allowedOrigins: string[] })`. Checks the resolved request URL's origin against the list before attaching the token. Leaves `request(input, options)`'s existing calling convention untouched — lower risk, matches what the followup actually asked for.
