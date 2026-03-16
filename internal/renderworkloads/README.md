
# renderworkloads

`renderworkloads` is an internal helper used by CI to **render the Kubernetes collector workload definitions in this repository**
into fully-materialized Kubernetes YAML.

The rendered output is then checked with **Kyverno** to enforce a baseline container `securityContext`. This provides:
- a regression test that the **components included in the Dynatrace OTel Collector distribution** remain compatible with hardened settings
- a guardrail when adding/changing components or manifests

## How to use (local)

```bash
make render-workloads 
```

This produces:
- Rendered workload YAMLs under `"$OUT_BASE"` (paths preserved relative to repo root)
- A file list at `"$OUT_BASE/workloads.txt"` (one rendered YAML path per line)

### Run Kyverno checks

This runs the Kyverno policies against the rendered workloads listed in `workloads.txt`.

```bash
OUT_BASE="/tmp/rendered-collectors-workloads"
make kyverno-workloads OUT_BASE="$OUT_BASE"
```

Expected output looks like:

```text
Applying 1 policy rule(s) to N resource(s)...
pass: N, fail: 0, warn: 0, error: 0, skip: 0
```

## Notes

- `kyverno-workloads` depends on `render-workloads` and will re-render before running Kyverno.
- If `workloads.txt` is empty, the Kyverno target will fail (to avoid silently doing nothing).
- You need `kyverno` available in your `PATH`.
