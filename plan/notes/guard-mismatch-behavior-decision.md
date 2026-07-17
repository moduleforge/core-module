# Guard mismatch behavior decision

## Question

When the guard is configured and a request's resolved URL origin isn't in `allowedOrigins`, what should happen? Candidates:
- Throw (fail closed) — reject the request before it's made.
- Drop token, still request — silently omit the bearer token but still issue the request.
- Log/warn only — attach the token and issue the request regardless, just emit a console warning.

## Answer

Throw (fail closed) (Recommended)

Reject the request before it's made when the origin isn't allowed. Loudest, safest default for a security guard — makes misconfiguration/misuse visible immediately rather than silently degrading auth.
