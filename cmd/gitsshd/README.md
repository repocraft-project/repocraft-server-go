# gitsshd demo

Launches an SSH-only Git server on `:2222`, generating a demo host key, creating empty `./ssh/authorized_keys` and `./.repositories`, then serving `git-upload-pack` / `git-receive-pack` only.

Run from repository root:

```bash
go run ./cmd/gitsshd
```

Add your public key to `./ssh/authorized_keys` and place bare repos under `./.repositories` (e.g., `./.repositories/owner/repo.git`), then clone:

```bash
git clone ssh://localhost:2222/owner/repo.git
```
