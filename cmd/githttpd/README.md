# githttpd demo

This demo starts a minimal Git Smart HTTP server on `:8080`, serving bare repositories under `./.repositories`.

## Run

```bash
cd repocraft-server-go
go run ./cmd/githttpd
```

## Repository layout

Place bare repos under `./.repositories`, matching URL paths. Example for `http://localhost:8080/owner/repo.git`:

```
repocraft-server-go/
  .repositories/
    owner/
      repo/   # bare repo (run `git init --bare` inside this directory)
```

## Clone

```bash
git clone http://localhost:8080/owner/repo.git
```

## Push

From within a clone:

```bash
git push
```

Notes:
- Smart HTTP is stateless and uses `git-upload-pack` / `git-receive-pack` binaries on the host.
- No authentication is implemented in this demo.
