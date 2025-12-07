# Repocraft Server Go

The Go implementation of the Repocraft platform server.

## Demos

All demos serve bare repositories under `./.repositories` relative to the repo root.

- `cmd/gitsshd`: SSH-only Git server on `:2222`, git-upload-pack and git-receive-pack, authorized_keys auth.
- `cmd/githttpd`: Smart HTTP Git server on `:8080`, git-upload-pack and git-receive-pack, no auth.
