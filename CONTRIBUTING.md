# Contributing to baton-relay

`baton-relay` is the runnable demo for [baton](https://github.com/agarwalvivek29/baton). Fixes that make it easier to run (other platforms, kernels, clearer docs) are the most welcome contributions. SDK changes belong in the [baton repo](https://github.com/agarwalvivek29/baton/blob/main/CONTRIBUTING.md).

## Ground rules

- **Services stay tracing-free.** The services use plain `net/http` and baton. No OpenTelemetry SDK, no tracing code. That's the point of the demo.
- **Cite every eBPF agent setting.** Each key in `deploy/obi.yml` has a comment linking to the upstream docs or source at the pinned version. Don't add a key you can't cite.
- **Pin image tags.** No `latest`.
- **Measure, don't assume.** Numbers in `docs/goroutines.md` come from `scripts/count-traces.py` / `count-by-window.py` runs. If you change them, say how you measured and on which kernel.
- **Generic data only.** No real IDs, hostnames, or anything sensitive in the demo.

## Development

Clone both repos side by side (`go.mod` points at `../baton` until a tagged release):

```bash
git clone https://github.com/agarwalvivek29/baton
git clone https://github.com/agarwalvivek29/baton-relay
cd baton-relay
go vet ./... && go build ./services/...
shellcheck scripts/*.sh
cd deploy && docker compose up -d --build
```

## Pull requests

- One logical change per PR, with what, why and how it was tested.
- Commit subjects in the imperative mood, 72 characters or fewer.
- Security issues: follow [SECURITY.md](SECURITY.md), not public issues.

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). Contributions are licensed under [Apache-2.0](LICENSE).
