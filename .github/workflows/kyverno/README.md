### Validate rendered workloads with Kyverno (local)

Install the Kyverno CLI: https://kyverno.io/docs/kyverno-cli/

Then apply the repo’s policies to the rendered workloads. ( see [renderworkloads README](../../../internal/renderworkloads/README.md) for how to render the workloads in the first place)

```bash
OUT_BASE="/tmp/rendered-collectors-simple"

sed 's|^|-r |' "$OUT_BASE/workloads.txt" \
  | xargs -n 1000 kyverno apply .github/workflows/kyverno/policies/*.yaml
```

### CI / automation

The same render + validate steps run in the **YAML Policy Check** workflow [.github/workflows/yaml-policy-check.yml]( https://github.com/Dynatrace/dynatrace-otel-collector/blob/main/.github/workflows/yaml-policy-check.yml)

## Kyverno policies

Policies live in: [`.github/workflows/kyverno/policies/`](./policies)
The policy for the hardened Collector `securityContext`is [here](./policies/collector-securitycontext.yaml)
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
