### Validate rendered workloads with Kyverno

Install the Kyverno CLI: https://kyverno.io/docs/kyverno-cli/

Install gomplate: https://docs.gomplate.ca/installing/ or run 
```bash
make install-tools
```

Then run the Kyverno checks against the rendered workloads:

```bash
make kyverno-workloads 
```

### CI / automation

The same render + validate steps run in the **YAML Policy Check** workflow "[yaml-policy-check.yml](../yaml-policy-check.yml)"  

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
