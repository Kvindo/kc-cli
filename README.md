# `kc` — Kvindo Cloud CLI

`kc` is the command-line client for [Kvindo Cloud](https://cloud.kvindo.com). It talks to the
same API as the web console, so anything you can see or change in the console you can also script
with `kc`. It is modelled on `kubectl`: you read resources with `kc get` and create/update them
declaratively with `kc apply -f <manifest>`.

> **How it works (good to know):** `kc` is a *thin client*. It sends your command to the server and
> prints the result — all logic runs server-side. This means the CLI never needs upgrading to gain
> new server features, and the output is identical to the API.

---

## 1. Install

### Option A — download a prebuilt binary

Grab the binary for your platform from the [latest release](https://github.com/Kvindo/kc-cli/releases/latest):

| Platform | Asset |
|---|---|
| Linux x86-64 | `kc-linux-amd64` |
| Linux ARM64 | `kc-linux-arm64` |
| macOS Intel | `kc-darwin-amd64` |
| macOS Apple Silicon | `kc-darwin-arm64` |
| Windows x86-64 | `kc-windows-amd64.exe` |
| Windows ARM64 | `kc-windows-arm64.exe` |

Rename it to `kc` (or `kc.exe`), make it executable, and put it on your `PATH`:

```sh
# Linux / macOS
chmod +x kc-linux-amd64
sudo mv kc-linux-amd64 /usr/local/bin/kc
kc version
```

### Option B — build from source

Requires Go 1.25+:

```sh
git clone https://github.com/Kvindo/kc-cli.git
cd kc-cli
make build            # produces ./kc
# or cross-compile every platform into dist/:
make build-all
```

## 2. Authenticate

`kc` authenticates with a **bearer token** tied to your user. Create one in the web console:

> **Console → IAM → Tokens → Create token** — copy the token value (shown once).

Then just run any command. If `kc` has no token yet it will prompt you (input is hidden) and save it:

```
$ kc get folders
Enter token for profile "default": ********
```

The token is stored in your profile (see [Configuration](#3-configuration)) and reused from then on.
Verify it any time:

```sh
kc login          # "Logged in. Token is valid."
kc logout         # clears the saved token (asks for confirmation)
```

Other ways to supply the token (no prompt):

```sh
kc config set-token <token>          # save it explicitly
export KC_TOKEN=<token>              # one-off / CI — bypasses the config file entirely
```

By default `kc` talks to `https://cloud-api.kvindo.com`. Override it per command with
`KC_API_URL=…`, or persist it with `kc config set-server <url>`.

---

## 3. Configuration

Config lives under `~/.kc/`:

```
~/.kc/config/default.yaml     # the "default" profile: { server, token }
~/.kc/config/active           # name of the currently-active profile
~/.kc/cache/                  # response cache (safe to delete)
```

A profile file looks like:

```yaml
server: https://cloud-api.kvindo.com
token: eyJhbGci...
```

### Profiles (contexts)

Use profiles to switch between accounts or environments — like `kubectl` contexts:

```sh
kc config view                       # show the active profile
kc config get-contexts               # list all profiles
kc config use-context staging        # switch active profile
kc config set-server <url>           # set server URL on the active profile
kc config set-token  <token>         # set token on the active profile
```

### Environment variables

| Variable | Effect |
|---|---|
| `KC_PROFILE` | Use this profile instead of the active one |
| `KC_API_URL` | Override the server URL |
| `KC_TOKEN` | Use this token inline (skips the config file — handy in CI) |

```sh
KC_PROFILE=staging kc get vm
KC_API_URL=https://cloud-api.kvindo.com KC_TOKEN=$TOKEN kc get folders
```

---

## 4. Key concepts

- **Folders are namespaces.** Every resource lives in a folder. Scope a command to one with
  `-n <folder>` (by name or ID), or list across all folders with `-A`.
- **Name *or* ID.** Wherever a command takes a resource, you can pass its **name** or its **ULID**.
  Names are not globally unique — if a name matches more than one resource, `kc` lists the matches
  and asks you to use the ID.
- **Declarative first.** You create and update resources by applying a **manifest** (`kc apply -f`).
  The manifest format is the kubectl-style envelope that `kc get -o yaml` prints, so the easiest way
  to learn any resource's fields is to look at an existing one with `-o yaml`.
- **References accept names.** Inside a manifest, fields that point at another resource (e.g.
  `folderId`, `vpcId`, `vpcSubnetId`, `routeTableId`, `securityGroupIds`, …) accept that resource's
  **name** — `kc` resolves it to the ID for you. (Raw `curl`/API calls require the ULID.)

---

## 5. Reading resources — `kc get`

```sh
kc get vm                      # list VMs in your default folder
kc get vm -n prod              # list VMs in folder "prod"
kc get vm -A                   # list VMs across ALL folders
kc get vm my-vm                # one resource by name
kc get vm 01k9...              # one resource by ULID
kc get vm,s3-bucket,folder     # several types at once
kc get all                     # every top-level resource type
kc get network-all             # every resource in the networking product
```

### Output formats (`-o`)

| Format | Description |
|---|---|
| `table` *(default)* | Human-readable columns |
| `wide` | Table with extra columns (IPs, flavor, provider, …) |
| `json` | kubectl-style envelope (`apiVersion`, `kind`, `metadata`, `spec`, `status`) |
| `yaml` | Same envelope as YAML — **this is the manifest format for `apply`** |

```sh
kc get vpc my-vpc -o yaml          # see a resource as a manifest
kc get vm -A -o wide               # extra columns
kc get s3-bucket -o json | jq .    # machine-readable
kc get vm -q                       # IDs only (scripting)
```

### Docker-style shortcuts

```sh
kc ps                  # = kc get vm
kc images              # = kc get image
kc image ls            # = kc get image
kc volume ls -n prod   # = kc get volume -n prod
kc container ls        # = kc get vm
```

---

## 6. Creating & updating — `kc apply -f`

You create or update resources by applying a **manifest** — one or more kubectl-style documents.
`kc apply` is **idempotent**: run it again and it updates the existing resource instead of making a
duplicate (matched by name within the folder).

```sh
kc apply -f my-resources.yaml      # apply a file
kc apply -f -                      # apply a manifest from stdin
cat my.yaml | kc apply -f -
kc apply -f one.yaml --wait        # block until reconciled (single-document manifests only)
```

> `apply` is the only declarative verb. `kc create` is reserved for a future imperative create and
> currently reports “not implemented yet” — use `kc apply -f` to create resources.

### Manifest format

A document is an envelope with `apiVersion`, `kind`, `metadata`, and (for most types) `spec`.
The simplest discovery trick: `kc get <type> <name> -o yaml` and edit what it prints.

```yaml
apiVersion: v1
kind: Folder
metadata:
  name: my-app
  folderId: 01k0parent...        # parent folder (name or ID)
spec: {}
---
apiVersion: v1
kind: Vpc
metadata:
  name: my-vpc
  folderId: my-app               # ← references the folder above BY NAME
spec:
  hostingProviderId: 01kk7at...  # from: kc get provider
---
apiVersion: v1
kind: VpcSubnet
metadata:
  name: my-subnet
  folderId: my-app
spec:
  vpcId: my-vpc                  # ← references the VPC above BY NAME
  ipv4Cidr: "10.10.0.0/24"
```

Apply it all at once:

```sh
kc apply -f stack.yaml
# Folder/my-app applied
# Vpc/my-vpc applied
# VpcSubnet/my-subnet applied
```

Notes:

- **Multiple documents** are separated by `---` and applied in order, so a later document can
  reference a resource an earlier one creates (by name, as above).
- **Idempotent.** If a document has no `metadata.id`, `kc` looks up an existing resource of that
  `kind` with the same name in the same folder: none → create, one → update, many → it asks you to
  add `metadata.id` to disambiguate.
- **Updating** = edit the YAML (or re-run `kc get … -o yaml > file`, change it) and `kc apply -f`
  again.
- **Read-only fields** like `status` are ignored on apply.

### Editing in place — `kc edit`

Opens the resource's manifest in your `$EDITOR`; on save it is applied. Works for any resource type:

```sh
kc edit folder my-app
kc edit vpc my-vpc
```

---

## 7. Deleting — `kc delete`

```sh
kc delete vpc my-vpc                 # by name
kc delete vpc 01k9...                # by ID
kc rm     vpc my-vpc                 # rm / remove are aliases for delete
kc delete vpc my-vpc --wait          # block until it is actually gone
kc delete -f stack.yaml              # delete everything described in a manifest
kc delete -f stack.yaml --wait       # (single-document manifests)
```

Delete dependents before their parents (e.g. a subnet before its VPC).

---

## 8. Command & flag reference

### Commands

| Command | Description |
|---|---|
| `get`, `ls`, `list` | List resources (all types, all output formats) |
| `api-resources` | List every resource type with its aliases and kind (like `kubectl api-resources`) |
| `apply` | Create/update from a manifest with `-f <file\|->` |
| `edit` | Edit a resource in `$EDITOR` |
| `delete`, `remove`, `rm` | Delete a resource (add `--wait`; or `-f <manifest>`) |
| `login` | Verify the token and confirm identity |
| `logout` | Clear the saved token |
| `version` | Print the server version |
| `config` | Manage profiles (`view`, `get-contexts`, `use-context`, `set-server`, `set-token`) |
| `help` | Show built-in help — also `kc` (no args), `kc ?`, `kc -h`, `kc --help` |

### Flags

| Flag | Short | Applies to | Description |
|---|---|---|---|
| `--folder <name\|id>` | `-n` | read verbs | Scope to a folder |
| `--all-folders` | `-A` | `get` | Show resources in every folder |
| `--output <fmt>` | `-o` | `get` | `table` (default), `wide`, `json`, `yaml` |
| `--quiet` | `-q` | `get` | Print IDs only |
| `--filename <file>` | `-f` | `apply`/`delete` | Manifest path (`-` = stdin) |
| `--wait` | `-w` | `delete`, single-doc `apply`/`delete` | Block until reconciled |

> For `apply` and `delete`, `-f`/`--filename` is the **manifest path** (`-` = stdin). To scope a
> `get` to a folder, use `-n`/`--folder`.

### Resource types (aliases)

```
Core (core-all): folder, provider, transaction
IaM (iam-all): user, token, policy
Compute (compute-all): vm, volume, volume-attachment, image, image-schedule,
                       sshkey, ssh-private-key, cert, on-off-schedule
Networking (network-all): vpc, subnet, sg, fip, route-table, route-table-route,
                          route-table-attachment, peering, peering-peer, peering-external-peer
LoadBalancer (lb-all): lb, lb-target-group, lb-static-target, lb-sd-target,
                       lb-http-listener, lb-https-listener, lb-tls-listener,
                       lb-tcp-listener, lb-udp-listener,
                       lb-http-listener-rule, lb-https-listener-rule, lb-tls-listener-rule,
                       lb-tcp-listener-rule, lb-udp-listener-rule
K8s (k8s-all): k8s, k8s-ng, k8s-user, k8s-user-role
S3 (s3-all): s3-bucket, s3-user, s3-user-policy
PostgreSQL (pg-all): pg, pg-ng, pg-paramset, pg-standalone
VPN (vpn-all): openvpn, openvpn-user, openvpn-user-settings
GitLab (gitlab-all): gitlab, gitlab-runner
AI (ai-all): ollama
Quota (quota-all): quota, quota-change-request
Billing (billing-all): billing-account
```

Besides the short aliases above, every type also accepts its API names, so the following all refer
to the same resource:

```
k8s-ng                 # short alias
k8sng                  # short alias, no separators
kubernetes-node-group  # kebab-case API path
kubernetesnodegroup    # lowercase API kind
KubernetesNodeGroup    # API kind (PascalCase)
```

Run `kc api-resources` to list every type with its aliases, kind, and product group.

Group shorthands: `kc get all` (every top-level type) and `kc get <product>-all`
(e.g. `kc get network-all`, `kc get k8s-all`).

Run `kc help` for the built-in summary.

---

## 9. Common workflows

**Snapshot a resource as a reusable manifest**

```sh
kc get vpc my-vpc -o yaml > my-vpc.yaml      # edit, then `kc apply -f my-vpc.yaml`
```

**Copy a stack between folders**

```sh
kc get vpc,subnet -n staging -o yaml > stack.yaml
# edit folderId values to point at "prod", then:
kc apply -f stack.yaml
```

**Find a hosting provider ID for a new VPC**

```sh
kc get provider
```

**Scripting (JSON + jq)**

```sh
kc get vm -A -o json | jq -r '.items[].metadata.name'
```

---

## 10. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| Prompted for a token every time | Token wasn't saved — run `kc config set-token <token>`, or check `~/.kc/config/`. |
| `authentication failed` | Token is invalid/expired — create a new one in the console (IAM → Tokens). |
| `multiple … resources named "x" — use ID` | The name is ambiguous; pass the ULID instead (`kc get <type>` to find it). |
| `… resources with that name — add metadata.id to disambiguate` (apply) | Same, in a manifest: set `metadata.id` on that document. |
| Wrong server / environment | Check `kc config view`; set with `kc config set-server` or `KC_API_URL`. |
| Stale output | Delete the cache: `rm -rf ~/.kc/cache`. |

---

## 11. Not yet available

These appear in `kc help` as *coming soon* and currently return “not implemented yet”. Until then,
use **manifests** (`kc apply -f`) for create/update:

- `kc describe` — detailed resource view
- Imperative `kc create <type> --flag …`, `kc update`/`patch`
- `kc start` / `kc stop` (VM power)
- Docker-style `run` / `stop`, and `image rm` / `volume rm`  (top-level `kc rm <type> <name>` does work — it's an alias for `delete`)

There is no `logs`, `exec`, or `watch` (`-w` streaming) in this version.
