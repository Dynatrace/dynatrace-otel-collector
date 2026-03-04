# renderworkloads 

`renderworkloads` is an internal helper used by CI to **render the Kubernetes workload definitions in this repository**
into fully-materialized Kubernetes YAML.

The rendered output can then be checked with **Kyverno** to enforce a baseline container `securityContext`. This gives us a
regression test that the **components we include in the Dynatrace OTel Collector distribution** remain compatible with
these hardened settings in the deployment scenarios we test, and it provides a guardrail when adding new components.

## How to use

### 1) Render workloads (local)

Build and run the renderer to materialize all Kubernetes workloads used by the integration tests:

```bash
cd internal/renderworkloads
go build -o /tmp/rendercollectors .

OUT_BASE="/tmp/rendered-collectors-simple"

/tmp/rendercollectors \
  -repo-root "$(git rev-parse --show-toplevel)" \
  -integration-root internal/testbed/integration \
  -out-base "$OUT_BASE"
```

This produces:
- Rendered YAML workloads under `"$OUT_BASE"`
- A file list at `"$OUT_BASE/workloads.txt"` (one YAML path per line)

### 2) Validate rendered workloads with Kyverno (local)

Install the Kyverno CLI: https://kyverno.io/docs/kyverno-cli/

Then apply the repo’s policies to the rendered workloads:

```bash
OUT_BASE="/tmp/rendered-collectors-simple"

sed 's|^|-r |' "$OUT_BASE/workloads.txt" \
  | xargs -n 1000 kyverno apply .github/workflows/kyverno/policies/*.yaml
```

### CI / automation

The same render + validate steps run in the **YAML Policy Check** workflow [.github/workflows/yaml-policy-check.yml]( https://github.com/Dynatrace/dynatrace-otel-collector/blob/main/.github/workflows/yaml-policy-check.yml) 

## Kyverno policies

Policies live in: [`.github/workflows/kyverno/policies/`](https://github.com/Dynatrace/dynatrace-otel-collector/tree/main/.github/workflows/kyverno/policies)
The policy for the hardened Collector `securityContext`is [here](https://github.com/Dynatrace/dynatrace-otel-collector/blob/main/.github/workflows/kyverno/policies/collector-securitycontext.yaml)
It enforces the following container security settings:

- `securityContext.capabilities.drop: ["ALL"]`
- `securityContext.readOnlyRootFilesystem: true`
- `securityContext.allowPrivilegeEscalation: false`
- `securityContext.runAsNonRoot: true`
- `securityContext.runAsUser: 10001`
- `securityContext.runAsGroup: 10001`
- `securityContext.privileged: false`
- `securityContext.seccompProfile.type: RuntimeDefault`

These are widely recommended Kubernetes hardening defaults. For background, see:
- Kubernetes Security Context docs: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
- Kubernetes Pod Security Standards: https://kubernetes.io/docs/concepts/security/pod-security-standards/

## Notes / scope

- This is an **internal CI tool** (not part of the shipped Collector artifacts).
- The Kyverno validation applies to the **workloads/scenarios rendered and exercised by this repository’s CI**. It is
  intended as a compatibility/regression check and a guardrail for new additions — not a blanket guarantee that every
  possible configuration of every component will work under all hardened Kubernetes policies.
