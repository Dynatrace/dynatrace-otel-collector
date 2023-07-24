# Applying a hotfix

This document describes the process to apply a hotfix to an upstream component.
For this document, we will use the `loggingexporter` as an example.

1. If it does not already exist, create the `hotfixes` directory.
   
```sh
mkdir -p hotfixes
```

2. Temporarily clone the upstream repo at the version required.

```sh
git clone -b v0.79.0 https://github.com/open-telemetry/opentelemetry-collector/
```

3. Copy the files from upstream into the `hotfixes` directory.

```sh
cp -r opentelemetry-collector/exporter/loggingexporter hotfixes/loggingexporter
```

4. Delete the temporary clone of the upstream repo.

```sh
rm -rf opentelemetry-collector
```

5. Add the local path as the `path` property on the module in the [`manifest.yaml`](../manifest.yaml).

```diff
diff --git a/manifest.yaml b/manifest.yaml
index 476710d..1dad064 100644
--- a/manifest.yaml
+++ b/manifest.yaml
@@ -14,6 +14,7 @@ receivers:
 
 exporters:
   - gomod: go.opentelemetry.io/collector/exporter/loggingexporter v0.79.0
+    path: ./hotfixes/loggingexporter
   - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.79.0
   - gomod: go.opentelemetry.io/collector/exporter/otlphttpexporter v0.79.0
```

From this point on you can apply any required changes to the upstream module and future builds will reflect the local version.
